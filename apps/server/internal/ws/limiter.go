package ws

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// tokenBucket limits how many messages a socket may send per second.
//
// The bucket refills continuously. When it is empty the socket is dropped:
// a client that floods the server gets disconnected instead of consuming the
// instance that serves the game.
type tokenBucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	rate     float64 // tokens per second
	last     time.Time
	now      func() time.Time
}

// newTokenBucket creates a bucket that allows rate messages per second with
// a burst equal to the rate.
func newTokenBucket(rate int) *tokenBucket {
	if rate <= 0 {
		rate = DefaultMessagesPerSecond
	}
	return &tokenBucket{
		capacity: float64(rate),
		tokens:   float64(rate),
		rate:     float64(rate),
		last:     time.Now(),
		now:      time.Now,
	}
}

// allow reports whether a message may be processed, consuming a token.
func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	// Refill the bucket.
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// clientIP returns the address of the client, preferring the header set by
// the proxy in front of the server.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// The header holds a list: the first entry is the original client.
		for i := 0; i < len(forwarded); i++ {
			if forwarded[i] == ',' {
				return trimSpace(forwarded[:i])
			}
		}
		return trimSpace(forwarded)
	}
	if real := r.Header.Get("X-Real-Ip"); real != "" {
		return trimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// trimSpace removes the spaces around a value.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
