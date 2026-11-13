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

	// roomMu guards the room field: it is written by the read goroutine and
	// read by the broadcast goroutine.
	roomMu sync.RWMutex
	// room is the room the client belongs to, nil until it joins one.
	room *room.Room
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

// Hub owns the rooms and the connected clients.
type Hub struct {
	rooms   *room.Registry
	mu      sync.RWMutex
	clients map[*client]bool
	stop    chan struct{}
}

// NewHub creates a hub over a room registry and starts its broadcast loop.
func NewHub(rooms *room.Registry) *Hub {
	h := &Hub{
		rooms:   rooms,
		clients: make(map[*client]bool),
		stop:    make(chan struct{}),
	}
	go h.Run()
	return h
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
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade failed: %v", err)
		return
	}
	c := &client{
		id:   newID(),
		conn: conn,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
		hub:  h,
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
		snapshot := r.Game().SnapshotFor(c.id)
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
		c.leaveRoom()
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
		c.handle(msg)
	}
}

// handle processes a single client message.
func (c *client) handle(msg ClientMessage) {
	switch msg.Type {
	case TypeCreate:
		c.createRoom(msg.Name)
	case TypeJoin:
		c.joinRoom(msg.Name, msg.Room, msg.Key)
	case TypeLeave:
		c.leaveRoom()
		c.sendJSON(ServerMessage{Type: TypeLeft})
	case TypePlace:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.PlaceShip(c.id, game.ShipKind(msg.Kind), msg.X, msg.Y, game.Orientation(msg.Orientation)); err != nil {
			c.sendError(err.Error())
		}
	case TypeRemove:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.RemoveShip(c.id, game.ShipKind(msg.Kind)); err != nil {
			c.sendError(err.Error())
		}
	case TypeReady:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if err := g.Ready(c.id); err != nil {
			c.sendError(err.Error())
		}
	case TypeShoot:
		g, err := c.requireGame()
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if _, err := g.Shoot(c.id, msg.X, msg.Y); err != nil {
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
	if _, ok := created.Game().AddPlayer(c.id, name); !ok {
		c.hub.rooms.Delete(created.Code())
		c.sendError("no se pudo entrar en la sala")
		return
	}
	c.setRoom(created)
	// The creator receives the secret key: it is what it has to share with
	// the opponent to let the opponent in.
	c.sendJSON(ServerMessage{
		Type: TypeWelcome, PlayerID: c.id, Room: created.Code(), Key: created.Key(),
	})
}

// joinRoom enters an existing room identified by its six-digit code and its
// secret key.
func (c *client) joinRoom(name, code, key string) {
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
	// The secret key is the credential: the code alone does not admit a
	// player, so a guessed code is not enough to sit in a match.
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
	if _, ok := found.Game().AddPlayer(c.id, name); !ok {
		c.sendError("la sala está llena")
		return
	}
	c.setRoom(found)
	c.sendJSON(ServerMessage{Type: TypeWelcome, PlayerID: c.id, Room: found.Code()})
}

// leaveRoom removes the client from its room. The room is released when it is
// left empty, and a match in progress is abandoned.
func (c *client) leaveRoom() {
	r := c.currentRoom()
	if r == nil {
		return
	}
	c.setRoom(nil)

	g := r.Game()
	wasPlaying := g.Phase() == game.PhasePlaying
	g.RemovePlayer(c.id)
	if wasPlaying {
		// The match is abandoned: the remaining player starts over.
		g.Reset()
	}
	if len(g.Players()) == 0 {
		c.hub.rooms.Delete(r.Code())
	}
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
