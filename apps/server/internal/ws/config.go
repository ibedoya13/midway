package ws

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the tunable settings of the WebSocket server.
//
// Everything is read from the environment so that a deployment can be tuned
// without a rebuild. The defaults are the ones that keep local development
// working.
type Config struct {
	// AllowedOrigins is the list of origins allowed to open a socket.
	AllowedOrigins []string
	// AllowEmptyOrigin accepts clients that send no Origin header: probes,
	// health checks and other non-browser callers.
	AllowEmptyOrigin bool
	// MaxConnectionsPerIP limits how many sockets a single address opens.
	MaxConnectionsPerIP int
	// MaxConnections limits how many sockets are open at the same time.
	MaxConnections int
	// MessagesPerSecond is the rate limit of messages of a single socket.
	MessagesPerSecond int
	// ReconnectGrace is how long the seat of a disconnected player is kept.
	ReconnectGrace time.Duration
}

// Default values, tuned so that local development is not disturbed.
const (
	// DefaultMaxConnectionsPerIP allows several players on the same
	// network: one address is a whole household or office behind the same
	// router, and each of them may open a socket.
	DefaultMaxConnectionsPerIP = 20
	// DefaultMaxConnections caps the total number of open sockets.
	DefaultMaxConnections = 100
	// DefaultMessagesPerSecond allows a burst of gameplay messages.
	DefaultMessagesPerSecond = 20
	// DefaultReconnectGrace is how long a player that loses the connection
	// keeps its seat. Cloud Run closes the WebSocket after 60 minutes, so
	// the grace period has to be long enough for the client to come back.
	DefaultReconnectGrace = 5 * time.Minute
)

// defaultAllowedOrigins are the origins allowed when the environment does
// not say otherwise: the deployed frontend and the local development server.
var defaultAllowedOrigins = []string{
	"https://midway.amarrete.com",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// ConfigFromEnv builds the configuration from the environment.
//
// ALLOWED_ORIGINS      comma-separated list of allowed origins.
// ALLOW_EMPTY_ORIGIN   whether clients without an Origin may connect.
// MAX_CONNECTIONS_PER_IP  sockets allowed per remote address.
// MAX_CONNECTIONS      total number of sockets allowed.
// MESSAGES_PER_SECOND  messages allowed per second and socket.
//
// A value that cannot be parsed falls back to the default, so a broken
// environment never disables a limit.
func ConfigFromEnv() Config {
	return Config{
		AllowedOrigins:      allowedOriginsFromEnv(),
		AllowEmptyOrigin:    boolFromEnv("ALLOW_EMPTY_ORIGIN", true),
		MaxConnectionsPerIP: intFromEnv("MAX_CONNECTIONS_PER_IP", DefaultMaxConnectionsPerIP),
		MaxConnections:      intFromEnv("MAX_CONNECTIONS", DefaultMaxConnections),
		MessagesPerSecond:   intFromEnv("MESSAGES_PER_SECOND", DefaultMessagesPerSecond),
		ReconnectGrace:      durationFromEnv("RECONNECT_GRACE_SECONDS", DefaultReconnectGrace),
	}
}

// allowedOriginsFromEnv reads the allowlist, falling back to the defaults.
//
// The list is separated by commas or semicolons. The semicolon exists
// because the gcloud CLI treats a comma as the separator of its own
// key=value dict arguments, so a comma-separated value cannot be passed
// through it.
func allowedOriginsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if raw == "" {
		return append([]string(nil), defaultAllowedOrigins...)
	}
	var origins []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return append([]string(nil), defaultAllowedOrigins...)
	}
	return origins
}

// AllowsOrigin reports whether the given Origin may open a socket.
//
// An empty Origin comes from a client that is not a browser. The rule is
// explicit so it never matches the allowlist by accident.
//
// Besides the literal list, the origins of the private network are allowed:
// the game is played between devices of the same local network, whose IP
// changes with the router.
func (c Config) AllowsOrigin(origin string) bool {
	if origin == "" {
		return c.AllowEmptyOrigin
	}
	for _, allowed := range c.AllowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return isPrivateNetworkOrigin(origin)
}

// isPrivateNetworkOrigin reports whether the origin points to a private
// network address (192.168.x.x, 10.x.x.x or 172.16-31.x.x).
func isPrivateNetworkOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsPrivate() {
		return false
	}
	// Only the local development server ports are accepted.
	switch parsed.Port() {
	case "5173", "4173":
		return true
	}
	return false
}

// durationFromEnv reads a duration in seconds from the environment.
func durationFromEnv(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

// boolFromEnv reads a boolean from the environment.
func boolFromEnv(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

// intFromEnv reads a positive integer from the environment.
func intFromEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
