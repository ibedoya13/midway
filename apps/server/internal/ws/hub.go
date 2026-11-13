// Package ws implements the WebSocket transport for the Midway game server.
package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/game"
	"midway/server/internal/room"
)

const (
	// writeWait is the deadline for writing a single message.
	writeWait = 10 * time.Second
	// pongWait is how long the server waits for a ping before closing.
	pongWait = 60 * time.Second
	// pingPeriod is the interval between server pings.
	pingPeriod = (pongWait * 9) / 10
	// broadcastRate is how many state snapshots are sent per second.
	broadcastRate = 20
)

// The upgrader accepts the upgrade: the origin was already validated by
// ServeWS, which rejects the handshake with 403 before reaching this point.
// Keep this permissive on purpose so the check is not duplicated in two
// places.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// client is a single WebSocket connection.
type client struct {
	id   string
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
	hub  *Hub

	// ip is the remote address of the client, used for the connection limit.
	ip string
	// playerID is the identity of the player in the match. It survives a
	// reconnection, unlike the id of the connection.
	playerID string
	// resumeToken identifies the player across a reconnection.
	resumeToken string
	// bucket is the token bucket that limits the message rate.
	bucket *tokenBucket

	// roomMu guards the room field: it is written by the read goroutine and
	// read by the broadcast goroutine.
	roomMu sync.RWMutex
	// room is the room the client belongs to, nil until it joins one.
	room *room.Room
}

// currentGameID returns the identity of the player in the match, empty when
// the client is not in a room.
func (c *client) currentGameID() string {
	c.roomMu.RLock()
	defer c.roomMu.RUnlock()
	return c.playerID
}

// currentRoom returns the room of the client, or nil when it has not joined
// one yet.
func (c *client) currentRoom() *room.Room {
	c.roomMu.RLock()
	defer c.roomMu.RUnlock()
	return c.room
}

// setRoom assigns the room of the client.
func (c *client) setRoom(r *room.Room) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	c.room = r
}

// setPlayer assigns the identity of the player in the match.
func (c *client) setPlayer(playerID, resumeToken string) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	c.playerID = playerID
	c.resumeToken = resumeToken
}

// Hub owns the rooms and the connected clients.
type Hub struct {
	rooms   *room.Registry
	config  Config
	mu      sync.RWMutex
	clients map[*client]bool
	stop    chan struct{}

	// timersMu guards the pending removals.
	timersMu sync.Mutex
	// removals holds the pending removal of a player that went away.
	removals map[string]*time.Timer

	// connMu guards the connection counters.
	connMu sync.Mutex
	// connsPerIP counts the open sockets by remote address.
	connsPerIP map[string]int
	// totalConns is the number of open sockets.
	totalConns int
}

// NewHub creates a hub over a room registry with the default configuration
// and starts its broadcast loop.
func NewHub(rooms *room.Registry) *Hub {
	return NewHubWithConfig(rooms, ConfigFromEnv())
}

// NewHubWithConfig creates a hub with an explicit configuration and starts
// its broadcast loop.
func NewHubWithConfig(rooms *room.Registry, config Config) *Hub {
	h := &Hub{
		rooms:      rooms,
		config:     config,
		clients:    make(map[*client]bool),
		stop:       make(chan struct{}),
		connsPerIP: make(map[string]int),
		removals:   make(map[string]*time.Timer),
	}
	go h.Run()
	return h
}

// registerConnection counts a new socket and reports whether it is allowed.
func (h *Hub) registerConnection(ip string) bool {
	h.connMu.Lock()
	defer h.connMu.Unlock()

	if h.totalConns >= h.config.MaxConnections {
		return false
	}
	if h.connsPerIP[ip] >= h.config.MaxConnectionsPerIP {
		return false
	}
	h.connsPerIP[ip]++
	h.totalConns++
	return true
}

// releaseConnection gives back the slot of a closed socket.
func (h *Hub) releaseConnection(ip string) {
	h.connMu.Lock()
	defer h.connMu.Unlock()

	if h.connsPerIP[ip] > 0 {
		h.connsPerIP[ip]--
		if h.connsPerIP[ip] == 0 {
			delete(h.connsPerIP, ip)
		}
	}
	h.totalConns--
}

// Stop ends the broadcast loop.
func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
}

// ServeWS upgrades an HTTP request to a WebSocket connection.
//
// The origin is checked before the upgrade, so a foreign website gets a 403
// instead of a socket it could abuse.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	if !h.config.AllowsOrigin(r.Header.Get("Origin")) {
		log.Printf("ws: origin rejected: %q", r.Header.Get("Origin"))
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	ip := clientIP(r)
	if !h.registerConnection(ip) {
		log.Printf("ws: connection limit reached for %s", ip)
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.releaseConnection(ip)
		log.Printf("ws: upgrade failed: %v", err)
		return
	}
	c := &client{
		ip:     ip,
		bucket: newTokenBucket(h.config.MessagesPerSecond),
		id:     newID(),
		conn:   conn,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
		hub:    h,
	}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go c.writePump()
	go c.readPump()
}

// Run broadcasts state snapshots to every client.
func (h *Hub) Run() {
	ticker := time.NewTicker(time.Second / broadcastRate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.broadcastState()
		case <-h.stop:
			return
		}
	}
}

// broadcastState sends the current snapshot of each room to the clients of
// that room. A client only ever sees the match it belongs to.
func (h *Hub) broadcastState() {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		r := c.currentRoom()
		if r == nil {
			// The client has not entered a room yet.
			continue
		}
		gameID := c.currentGameID()
		if gameID == "" {
			// The client is entering the room but has no identity yet.
			continue
		}
		snapshot := r.Game().SnapshotFor(gameID)
		payload, err := json.Marshal(ServerMessage{Type: TypeState, State: &snapshot})
		if err != nil {
			log.Printf("ws: marshal state: %v", err)
			continue
		}
		select {
		case c.send <- payload:
		default:
			// Slow client: drop it rather than blocking the broadcast.
			h.remove(c)
		}
	}
}

// scheduleRemoval releases the seat of a player that went away, unless it
// comes back before the grace period ends.
func (h *Hub) scheduleRemoval(r *room.Room, playerID string) {
	h.timersMu.Lock()
	defer h.timersMu.Unlock()

	if existing, ok := h.removals[playerID]; ok {
		existing.Stop()
	}
	h.removals[playerID] = time.AfterFunc(h.config.ReconnectGrace, func() {
		h.timersMu.Lock()
		delete(h.removals, playerID)
		h.timersMu.Unlock()

		g := r.Game()
		g.RemovePlayer(playerID)
		if len(g.Players()) == 0 {
			h.rooms.Delete(r.Code())
		}
	})
}

// cancelRemoval stops the pending removal of a player that came back.
func (h *Hub) cancelRemoval(playerID string) {
	h.timersMu.Lock()
	defer h.timersMu.Unlock()

	if timer, ok := h.removals[playerID]; ok {
		timer.Stop()
		delete(h.removals, playerID)
	}
}

// remove disconnects a client.
func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.done)
	}
	h.mu.Unlock()
	c.conn.Close()
}

// readPump reads client messages and applies them to the game.
func (c *client) readPump() {
	defer func() {
		c.hub.remove(c)
		// The seat is kept for a while: the client may come back.
		c.disconnectRoom()
		c.hub.releaseConnection(c.ip)
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var msg ClientMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		// A client that floods the server is disconnected: the instance is
		// shared by every player and cannot be exhausted.
		if !c.bucket.allow() {
			log.Printf("ws: message rate limit reached, closing the connection")
			return
		}
		c.handle(msg)
	}
}

// handle processes a single client message.
func (c *client) handle(msg ClientMessage) {
	switch msg.Type {
	case TypeCreate:
		c.createRoom(msg.Name)
	case TypeJoin:
		c.joinRoom(msg.Name, msg.Room, msg.Key, msg.ResumeToken)
	case TypeLeave:
		c.leaveRoom()
		c.sendJSON(ServerMessage{Type: TypeLeft})
	case TypePlace:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.PlaceShip(c.currentGameID(), game.ShipKind(msg.Kind), msg.X, msg.Y, game.Orientation(msg.Orientation)); err != nil {
			c.sendError(err.Error())
		}
	case TypeRemove:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.RemoveShip(c.currentGameID(), game.ShipKind(msg.Kind)); err != nil {
			c.sendError(err.Error())
		}
	case TypeReady:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.Ready(c.currentGameID()); err != nil {
			c.sendError(err.Error())
		}
	case TypeShoot:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if _, err := g.Shoot(c.currentGameID(), msg.X, msg.Y); err != nil {
			c.sendError(err.Error())
		}
	case TypeStart:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		g.Reset()
	case TypePing:
		c.sendJSON(ServerMessage{Type: TypePong})
	default:
		c.sendError("mensaje desconocido: " + msg.Type)
	}
}

// createRoom makes a new room and joins it as its first player.
func (c *client) createRoom(name string) {
	if c.currentRoom() != nil {
		c.sendError("ya estás en una sala")
		return
	}
	created, err := c.hub.rooms.Create()
	if err != nil {
		c.sendError(err.Error())
		return
	}
	player, ok := created.Game().AddPlayer(c.id, name)
	if !ok {
		c.hub.rooms.Delete(created.Code())
		c.sendError("no se pudo entrar en la sala")
		return
	}
	// The identity is set before the room: otherwise there is a window in
	// which the client has a room but no identity, and the broadcast would
	// send it an empty snapshot.
	c.setPlayer(player.ID, player.ResumeToken)
	c.setRoom(created)
	// The creator receives the secret key: it is what it has to share with
	// the opponent to let the opponent in.
	c.sendJSON(ServerMessage{
		Type:        TypeWelcome,
		PlayerID:    player.ID,
		Room:        created.Code(),
		Key:         created.Key(),
		ResumeToken: player.ResumeToken,
	})
}

// joinRoom enters an existing room.
//
// The client sends the six-digit code of the room and one of the two
// credentials:
//
//   - the resume token, when it comes back after a disconnection: it takes
//     its seat again, with the fleet and the place in the turn order;
//   - the secret key of the room, when it enters for the first time.
//
// The code alone admits nobody: it is short enough to be guessed.
func (c *client) joinRoom(name, code, key, resumeToken string) {
	if c.currentRoom() != nil {
		c.sendError("ya estás en una sala")
		return
	}
	code = strings.TrimSpace(code)
	if !room.IsValidCode(code) {
		c.sendError("el código de sala debe tener seis dígitos")
		return
	}
	found, ok := c.hub.rooms.Get(code)
	if !ok {
		c.sendError("no existe la sala " + code)
		return
	}

	// A player that comes back after a disconnection takes its seat again.
	if player, ok := found.Game().ResumePlayer(strings.TrimSpace(resumeToken)); ok {
		c.setPlayer(player.ID, player.ResumeToken)
		c.setRoom(found)
		c.hub.cancelRemoval(player.ID)
		c.sendJSON(ServerMessage{
			Type:        TypeWelcome,
			PlayerID:    player.ID,
			Room:        found.Code(),
			ResumeToken: player.ResumeToken,
		})
		return
	}

	// The secret key is the credential of a first entry.
	if !found.MatchesKey(key) {
		c.sendError("la clave de la sala no es correcta")
		return
	}
	// A match that already started cannot be joined: it would break the turn
	// order and the boards of the players who are already playing.
	if found.Game().Phase() == game.PhasePlaying {
		c.sendError("la partida de esa sala ya empezó")
		return
	}

	player, ok := found.Game().AddPlayer(c.id, name)
	if !ok {
		c.sendError("la sala está llena")
		return
	}
	c.setPlayer(player.ID, player.ResumeToken)
	c.setRoom(found)
	c.sendJSON(ServerMessage{
		Type:        TypeWelcome,
		PlayerID:    player.ID,
		Room:        found.Code(),
		ResumeToken: player.ResumeToken,
	})
}

// leaveRoom removes the client from its room. The room is released when it is
// left empty, and a match in progress is abandoned.
//
// This is the deliberate exit: the player gives up its seat.
func (c *client) leaveRoom() {
	r := c.currentRoom()
	if r == nil {
		return
	}
	c.setRoom(nil)

	g := r.Game()
	wasPlaying := g.Phase() == game.PhasePlaying
	g.RemovePlayer(c.currentGameID())
	if wasPlaying {
		// The match is abandoned: the remaining player starts over.
		g.Reset()
	}
	if len(g.Players()) == 0 {
		c.hub.rooms.Delete(r.Code())
	}
}

// disconnectRoom marks the player as away without giving up its seat: the
// client may come back and resume the match. The seat is released after the
// grace period if it does not.
func (c *client) disconnectRoom() {
	r := c.currentRoom()
	if r == nil {
		return
	}
	playerID := c.currentGameID()
	if playerID == "" {
		return
	}
	r.Game().SetDisconnected(playerID, true)
	c.hub.scheduleRemoval(r, playerID)
}

// requireGame returns the match of the client's room.
func (c *client) requireGame() (*game.Game, error) {
	r := c.currentRoom()
	if r == nil {
		return nil, fmt.Errorf("debes entrar en una sala antes de jugar")
	}
	return r.Game(), nil
}

// sendJSON marshals and queues a message for the client.
func (c *client) sendJSON(msg ServerMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal: %v", err)
		return
	}
	select {
	case c.send <- payload:
	default:
	}
}

// sendError queues an error message for the client.
func (c *client) sendError(text string) {
	c.sendJSON(ServerMessage{Type: TypeError, Message: text})
}

// writePump writes queued messages and keeps the connection alive.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case payload := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-c.done:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
