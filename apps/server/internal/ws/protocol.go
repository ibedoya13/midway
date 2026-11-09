package ws

import "midway/server/internal/game"

// ClientMessage is a message sent by a client to the server.
type ClientMessage struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Kind        string `json:"kind,omitempty"`
	X           int    `json:"x,omitempty"`
	Y           int    `json:"y,omitempty"`
	Orientation string `json:"orientation,omitempty"`
}

// ServerMessage is a message sent by the server to a client.
type ServerMessage struct {
	Type     string         `json:"type"`
	PlayerID string         `json:"playerId,omitempty"`
	Room     string         `json:"room,omitempty"`
	Message  string         `json:"message,omitempty"`
	State    *game.Snapshot `json:"state,omitempty"`
}

// Message types exchanged over the wire.
const (
	// Client to server.
	TypeJoin   = "join"
	TypePlace  = "place"
	TypeRemove = "remove"
	TypeReady  = "ready"
	TypeShoot  = "shoot"
	TypeStart  = "start"
	TypePing   = "ping"

	// Server to client.
	TypeWelcome = "welcome"
	TypeState   = "state"
	TypeError   = "error"
	TypePong    = "pong"
)
