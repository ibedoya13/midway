package ws

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAPlayerThatComesBackKeepsItsSeat covers the reconnection: the client
// that lost the connection takes its seat again, with the same identity and
// the same fleet.
func TestAPlayerThatComesBackKeepsItsSeat(t *testing.T) {
	srv := newTestServer(t)
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	w2 := readMessage(t, second, TypeWelcome)

	// The second player places a ship and confirms.
	second.WriteJSON(ClientMessage{
		Type: TypePlace, Kind: "cruiser", X: 2, Y: 3, Orientation: "horizontal",
	})
	second.WriteJSON(ClientMessage{Type: TypeReady})

	// The connection is cut.
	second.Close()
	time.Sleep(200 * time.Millisecond)

	// The player comes back with its resume token.
	back := srv.dial(t)
	back.WriteJSON(ClientMessage{
		Type: TypeJoin, Name: "Beto", Room: code, ResumeToken: w2.ResumeToken,
	})
	w := readMessage(t, back, TypeWelcome)

	if w.PlayerID != w2.PlayerID {
		t.Fatalf("playerId = %q, want %q", w.PlayerID, w2.PlayerID)
	}
	if w.ResumeToken != w2.ResumeToken {
		t.Fatal("the resume token must not change")
	}

	// The fleet is still there.
	snap := readState(t, back)
	if len(snap.Own.Ships) != 1 {
		t.Fatalf("the player has %d ships, want 1", len(snap.Own.Ships))
	}
}

// TestAResumeTokenDoesNotAdmitAStranger checks that the resume token only
// works for the player that owns it.
func TestAResumeTokenDoesNotAdmitAStranger(t *testing.T) {
	srv := newTestServer(t)
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	w2 := readMessage(t, second, TypeWelcome)

	second.Close()
	time.Sleep(200 * time.Millisecond)

	// A third client with a made-up token is refused.
	stranger := srv.dial(t)
	stranger.WriteJSON(ClientMessage{
		Type: TypeJoin, Name: "Intruso", Room: code, ResumeToken: "token-inventado",
	})
	readMessage(t, stranger, TypeError)

	// The real player still comes back.
	back := srv.dial(t)
	back.WriteJSON(ClientMessage{
		Type: TypeJoin, Name: "Beto", Room: code, ResumeToken: w2.ResumeToken,
	})
	readMessage(t, back, TypeWelcome)
}

// TestAResumeTokenOfAConnectedPlayerIsRefused checks that the token of a
// player that is still connected cannot be used to take its seat.
func TestAResumeTokenOfAConnectedPlayerIsRefused(t *testing.T) {
	srv := newTestServer(t)
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	w2 := readMessage(t, second, TypeWelcome)

	// The player is still connected: its token must not admit anybody.
	thief := srv.dial(t)
	thief.WriteJSON(ClientMessage{
		Type: TypeJoin, Name: "Ladrón", Room: code, ResumeToken: w2.ResumeToken,
	})
	readMessage(t, thief, TypeError)
}

// TestTheOpponentSeesThePlayerAsAway checks that the other player is told
// that its opponent went away.
func TestTheOpponentSeesThePlayerAsAway(t *testing.T) {
	srv := newTestServer(t)
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	readMessage(t, second, TypeWelcome)

	second.Close()

	// The first player sees the opponent as disconnected.
	away := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !away {
		snap := readState(t, first)
		for _, player := range snap.Players {
			if player.Name == "Beto" && player.Disconnected {
				away = true
			}
		}
	}
	if !away {
		t.Fatal("the opponent was never marked as disconnected")
	}
}

// TestTheSeatIsReleasedAfterTheGracePeriod checks that a player that does
// not come back frees its seat.
func TestTheSeatIsReleasedAfterTheGracePeriod(t *testing.T) {
	srv := newTestServerWithConfig(t, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 100,
		MaxConnections:      100,
		MessagesPerSecond:   DefaultMessagesPerSecond,
		ReconnectGrace:      300 * time.Millisecond,
	})
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	readMessage(t, second, TypeWelcome)

	second.Close()

	// After the grace period the seat is free again.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		room, ok := srv.rooms.Get(code)
		if ok && len(room.Game().Players()) == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	room, ok := srv.rooms.Get(code)
	if !ok {
		t.Fatal("the room must still exist")
	}
	if got := len(room.Game().Players()); got != 1 {
		t.Fatalf("the room holds %d players, want 1", got)
	}
}

// TestTheResumeTokenIsNotInTheState checks that the token never travels in
// the state broadcast.
func TestTheResumeTokenIsNotInTheState(t *testing.T) {
	srv := newTestServer(t)
	first, second := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, first, "Ana")
	second.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	w2 := readMessage(t, second, TypeWelcome)

	snap := readState(t, first)
	for _, player := range snap.Players {
		if player.ResumeToken != "" {
			t.Fatal("the state must not carry the resume token")
		}
	}
	if w2.ResumeToken == "" {
		t.Fatal("the player must receive a resume token")
	}
}

var _ = websocket.ErrCloseSent
