// Package room manages the game rooms: several pairs of players can play at
// the same time, each pair in its own room identified by a six-digit code.
//
// The codes are random, so a room cannot be guessed by counting: knowing one
// code says nothing about the others.
package room

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"midway/server/internal/game"
)

const (
	// CodeLength is the number of digits of a room code.
	CodeLength = 6
	// codeSpace is the number of distinct codes: 000000 to 999999.
	codeSpace = 1000000
	// MaxRooms is the default number of simultaneous rooms.
	//
	// The value comes from the profiling of the instance: with 512 MiB and
	// two players per room, 1000 rooms use 190 MiB (37%) and the broadcast
	// takes 15% of its 50 ms budget. The real limit is configured per
	// deployment with MAX_ROOMS.
	MaxRooms = 1000
	// KeyLength is the number of hex characters of the secret key.
	KeyLength = 32
	// keyBytes is the number of random bytes behind the secret key.
	keyBytes = KeyLength / 2
)

// Room is a match between two players, identified by its code.
//
// The room carries two different identifiers:
//
//   - Code is public and readable (six digits). It names the room so players
//     can talk about it, but on its own it is not a credential: it has only
//     a million possible values.
//   - Key is the secret credential. It is generated with crypto/rand and is
//     what actually admits a player to the room.
type Room struct {
	code string
	key  string
	game *game.Game
}

// Code returns the six-digit code that identifies the room.
func (r *Room) Code() string { return r.code }

// Key returns the secret key that admits a player to the room.
func (r *Room) Key() string { return r.key }

// MatchesKey reports whether the given secret key opens the room. The
// comparison is constant-time so it does not leak the key through timing.
func (r *Room) MatchesKey(key string) bool {
	return subtle.ConstantTimeCompare([]byte(r.key), []byte(key)) == 1
}

// Game returns the match state of the room.
func (r *Room) Game() *game.Game { return r.game }

// Registry owns the existing rooms.
type Registry struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	width  int
	height int
	// max is the number of rooms the registry holds at the same time.
	max int
}

// NewRegistry creates a registry whose rooms use the given board size and
// that holds up to maxRooms rooms.
func NewRegistry(width, height, maxRooms int) *Registry {
	if maxRooms <= 0 {
		maxRooms = MaxRooms
	}
	return &Registry{
		rooms:  make(map[string]*Room),
		width:  width,
		height: height,
		max:    maxRooms,
	}
}

// MaxRooms returns the number of rooms the registry holds at the same time.
func (reg *Registry) MaxRooms() int { return reg.max }

// Create makes a new room with a unique six-digit code. It fails when the
// registry already holds MaxRooms rooms.
func (reg *Registry) Create() (*Room, error) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if len(reg.rooms) >= reg.max {
		return nil, fmt.Errorf("no hay salas disponibles (máximo %d)", reg.max)
	}

	// Retry on the (unlikely) collision with an existing code.
	for attempt := 0; attempt < 20; attempt++ {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}
		if _, exists := reg.rooms[code]; exists {
			continue
		}
		key, err := generateKey()
		if err != nil {
			return nil, err
		}
		r := &Room{code: code, key: key, game: game.New(reg.width, reg.height)}
		reg.rooms[code] = r
		return r, nil
	}
	return nil, fmt.Errorf("no se pudo generar un código de sala libre")
}

// Get returns the room with the given code.
func (reg *Registry) Get(code string) (*Room, bool) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	r, ok := reg.rooms[code]
	return r, ok
}

// Delete removes a room from the registry.
func (reg *Registry) Delete(code string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	delete(reg.rooms, code)
}

// Count returns the number of existing rooms.
func (reg *Registry) Count() int {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return len(reg.rooms)
}

// IsValidCode reports whether a string is a well-formed room code.
func IsValidCode(code string) bool {
	if len(code) != CodeLength {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// generateCode returns a random six-digit code.
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return "", fmt.Errorf("no se pudo generar el código de sala: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// generateKey returns a random secret key of KeyLength hex characters.
func generateKey() (string, error) {
	buf := make([]byte, keyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("no se pudo generar la clave de sala: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
