// Command midway runs the Midway game server: a two-player Battleship game.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"midway/server/internal/room"
	"midway/server/internal/ws"
)

func main() {
	port := envOrDefault("PORT", "8080")
	boardSize := envIntOrDefault("BOARD_SIZE", 10)

	config := ws.ConfigFromEnv()

	registry := room.NewRegistry(boardSize, boardSize)
	hub := ws.NewHubWithConfig(registry, config)
	defer hub.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("midway server listening on :%s (board %dx%d, up to %d rooms)",
		port, boardSize, boardSize, room.MaxRooms)
	log.Printf("allowed origins: %v (empty origin allowed: %t)",
		config.AllowedOrigins, config.AllowEmptyOrigin)
	log.Printf("limits: %d connections per ip, %d in total, %d messages per second",
		config.MaxConnectionsPerIP, config.MaxConnections, config.MessagesPerSecond)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// envOrDefault returns the environment variable value or a fallback.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envIntOrDefault returns the environment variable as an int or a fallback.
func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
