package ws

import "midway/server/internal/game"

// ClientMessage is a message sent by a client to the server.
type ClientMessage struct {
	Type string `json:"type"`
	// Name is the display name of the player.
	Name string `json:"name,omitempty"`
	// Room is the six-digit code of the room to join.
	Room string `json:"room,omitempty"`
	// Key is the secret key of the room. It is the credential that admits a
	// player: the code alone is not enough.
	Key string `json:"key,omitempty"`
	// ResumeToken identifies a player that comes back after a disconnection.
	ResumeToken string `json:"resumeToken,omitempty"`
	// Kind is the ship type of a place or remove message.
	Kind string `json:"kind,omitempty"`
	// X and Y are the coordinates of a cell.
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
	// Orientation is the direction of a ship.
	Orientation string `json:"orientation,omitempty"`
}

// ServerMessage is a message sent by the server to a client.
type ServerMessage struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId,omitempty"`
	Room     string `json:"room,omitempty"`
	// Key is the secret key of the room. It is only sent to the player that
	// created the room, which has to share it with the opponent.
	Key string `json:"key,omitempty"`
	// ResumeToken identifies the player across a reconnection.
	ResumeToken string         `json:"resumeToken,omitempty"`
	Message     string         `json:"message,omitempty"`
	State       *game.Snapshot `json:"state,omitempty"`
}

// Message types exchanged over the wire.
const (
	// Client to server.
	TypeCreate = "create"
	TypeJoin   = "join"
	TypeLeave  = "leave"
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
	TypeLeft    = "left"
)
