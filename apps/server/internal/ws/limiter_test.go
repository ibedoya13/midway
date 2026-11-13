package ws

import (
	"net/http"
	"testing"
	"time"
)

// newRequest builds a request with the given remote address and headers.
func newRequest(remoteAddr string, headers map[string]string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	r.RemoteAddr = remoteAddr
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	return r
}

// TestTokenBucketAllowsTheBurst checks that the bucket allows a burst equal
// to its capacity.
func TestTokenBucketAllowsTheBurst(t *testing.T) {
	bucket := newTokenBucket(3)

	for i := 0; i < 3; i++ {
		if !bucket.allow() {
			t.Fatalf("message %d must be allowed by the burst", i+1)
		}
	}
	if bucket.allow() {
		t.Fatal("the fourth message must be refused")
	}
}

// TestTokenBucketRefillsOverTime checks that the bucket recovers.
func TestTokenBucketRefillsOverTime(t *testing.T) {
	bucket := newTokenBucket(2)
	base := time.Now()
	bucket.now = func() time.Time { return base }
	bucket.last = base

	for i := 0; i < 2; i++ {
		bucket.allow()
	}
	if bucket.allow() {
		t.Fatal("the bucket must be empty")
	}

	// A second later there is room for two more messages.
	base = base.Add(time.Second)
	if !bucket.allow() {
		t.Fatal("the bucket must have refilled")
	}
	if !bucket.allow() {
		t.Fatal("the bucket must have refilled")
	}
	if bucket.allow() {
		t.Fatal("the bucket must be empty again")
	}
}

// TestTokenBucketNeverExceedsItsCapacity checks that a long pause does not
// build an unbounded burst.
func TestTokenBucketNeverExceedsItsCapacity(t *testing.T) {
	bucket := newTokenBucket(2)
	base := time.Now()
	bucket.now = func() time.Time { return base }
	bucket.last = base

	base = base.Add(time.Hour)

	allowed := 0
	for i := 0; i < 10; i++ {
		if !bucket.allow() {
			break
		}
		allowed++
	}
	if allowed != 2 {
		t.Fatalf("allowed %d messages, want 2", allowed)
	}
}

// TestClientIPPrefersTheForwardedHeader checks that the address of the real
// client is taken from the proxy header.
func TestClientIPPrefersTheForwardedHeader(t *testing.T) {
	r := newRequest("203.0.113.7:5000", map[string]string{
		"X-Forwarded-For": "198.51.100.9, 10.0.0.1",
	})

	if got := clientIP(r); got != "198.51.100.9" {
		t.Fatalf("clientIP = %q, want %q", got, "198.51.100.9")
	}
}

// TestClientIPFallsBackToTheRemoteAddress checks the path without a proxy.
func TestClientIPFallsBackToTheRemoteAddress(t *testing.T) {
	r := newRequest("192.168.0.5:4000", nil)

	if got := clientIP(r); got != "192.168.0.5" {
		t.Fatalf("clientIP = %q, want %q", got, "192.168.0.5")
	}
}
