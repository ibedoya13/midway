package ws

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/game"
)

// TestTwoMatchesRunInParallel drives two independent matches at the same
// time: each pair places its fleet, plays its own turns and never observes
// the other match.
func TestTwoMatchesRunInParallel(t *testing.T) {
	srv := newTestServer(t)

	a1, a2 := srv.dial(t), srv.dial(t)
	b1, b2 := srv.dial(t), srv.dial(t)

	roomA, keyA := createRoom(t, a1, "Ana")
	roomB, keyB := createRoom(t, b1, "Beto")

	a2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Caro", Room: roomA, Key: keyA})
	readMessage(t, a2, TypeWelcome)
	b2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Dani", Room: roomB, Key: keyB})
	readMessage(t, b2, TypeWelcome)

	// Both pairs place their fleet and confirm.
	fleet := []struct {
		kind game.ShipKind
		x, y int
	}{
		{game.Carrier, 0, 0}, {game.Battleship, 0, 2}, {game.Cruiser, 0, 4},
		{game.Submarine, 0, 6}, {game.Destroyer, 0, 8},
	}
	for _, conn := range []*websocket.Conn{a1, a2, b1, b2} {
		for _, f := range fleet {
			conn.WriteJSON(ClientMessage{
				Type: TypePlace, Kind: string(f.kind), X: f.x, Y: f.y,
				Orientation: string(game.Horizontal),
			})
		}
		conn.WriteJSON(ClientMessage{Type: TypeReady})
	}

	// Both matches must be running.
	for _, conn := range []*websocket.Conn{a1, b1} {
		deadline := time.Now().Add(5 * time.Second)
		playing := false
		for time.Now().Before(deadline) && !playing {
			snap := readState(t, conn)
			playing = snap.Phase == game.PhasePlaying
		}
		if !playing {
			t.Fatal("the match never started")
		}
	}

	// A shot in match A is invisible in match B.
	a1.WriteJSON(ClientMessage{Type: TypeShoot, X: 0, Y: 0})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := readState(t, b1)
		if len(snap.Enemy.Shots) != 0 {
			t.Fatalf("match B observed %d shots from match A", len(snap.Enemy.Shots))
		}
		if len(snap.Players) != 2 {
			t.Fatalf("match B has %d players, want 2", len(snap.Players))
		}
	}
}
