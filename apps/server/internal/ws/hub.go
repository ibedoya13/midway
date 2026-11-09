// Package ws implements the WebSocket transport for the Midway game server.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/game"
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
	id     string
	conn   *websocket.Conn
	send   chan []byte
	done   chan struct{}
	hub    *Hub
	joined bool
}

// Hub owns the match and the connected clients.
type Hub struct {
	game    *game.Game
	room    string
	mu      sync.RWMutex
	clients map[*client]bool
	stop    chan struct{}
}

// NewHub creates a hub around a game instance and starts its broadcast loop.
func NewHub(g *game.Game) *Hub {
	h := &Hub{
		game:    g,
		room:    "midway-1",
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

// broadcastState sends the current snapshot to every client. Each client
// receives its own view: the own board shows the ships, the enemy board
// hides them.
func (h *Hub) broadcastState() {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		snapshot := h.game.SnapshotFor(c.id)
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
		if c.joined {
			c.hub.game.RemovePlayer(c.id)
		}
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
	case TypeJoin:
		if c.joined {
			c.sendError("ya estás en una partida")
			return
		}
		if _, ok := c.hub.game.AddPlayer(c.id, msg.Name); !ok {
			c.sendError("la sala está llena")
			return
		}
		c.joined = true
		c.sendJSON(ServerMessage{Type: TypeWelcome, PlayerID: c.id, Room: c.hub.room})
	case TypePlace:
		if !c.joined {
			c.sendError("debes unirte antes de jugar")
			return
		}
		if err := c.hub.game.PlaceShip(c.id, game.ShipKind(msg.Kind), msg.X, msg.Y, game.Orientation(msg.Orientation)); err != nil {
			c.sendError(err.Error())
		}
	case TypeRemove:
		if !c.joined {
			c.sendError("debes unirte antes de jugar")
			return
		}
		if err := c.hub.game.RemoveShip(c.id, game.ShipKind(msg.Kind)); err != nil {
			c.sendError(err.Error())
		}
	case TypeReady:
		if !c.joined {
			c.sendError("debes unirte antes de jugar")
			return
		}
		if err := c.hub.game.Ready(c.id); err != nil {
			c.sendError(err.Error())
		}
	case TypeShoot:
		if !c.joined {
			c.sendError("debes unirte antes de jugar")
			return
		}
		if _, err := c.hub.game.Shoot(c.id, msg.X, msg.Y); err != nil {
			c.sendError(err.Error())
		}
	case TypeStart:
		if !c.joined {
			c.sendError("debes unirte antes de jugar")
			return
		}
		c.hub.game.Reset()
	case TypePing:
		c.sendJSON(ServerMessage{Type: TypePong})
	default:
		c.sendError("mensaje desconocido: " + msg.Type)
	}
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
