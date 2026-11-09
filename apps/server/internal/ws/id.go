package ws

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a short random identifier for a connection.
func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "anon"
	}
	return hex.EncodeToString(buf)
}
