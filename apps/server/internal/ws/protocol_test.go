package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/game"
)

// testServer is a hub running behind an httptest server.
type testServer struct {
	game *game.Game
	url  string
}

// dial opens a new client connection to the server.
func (s *testServer) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(s.url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// newTestServer spins up a hub behind an httptest server.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	g := game.New(10, 10)
	hub := NewHub(g)
	t.Cleanup(hub.Stop)

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(srv.Close)

	return &testServer{game: g, url: "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"}
}

// readMessage reads the next JSON message of the expected type.
func readMessage(t *testing.T, conn *websocket.Conn, wantType string) ServerMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var msg ServerMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.Type == wantType {
			return msg
		}
	}
}

// readState reads the next state snapshot.
func readState(t *testing.T, conn *websocket.Conn) game.Snapshot {
	t.Helper()
	msg := readMessage(t, conn, TypeState)
	if msg.State == nil {
		t.Fatal("state message has no state payload")
	}
	return *msg.State
}

// placeFleet places the whole fleet for a client and confirms it.
func placeFleet(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	placements := []struct {
		kind game.ShipKind
		x, y int
	}{
		{game.Carrier, 0, 0},
		{game.Battleship, 0, 2},
		{game.Cruiser, 0, 4},
		{game.Submarine, 0, 6},
		{game.Destroyer, 0, 8},
	}
	for _, p := range placements {
		if err := conn.WriteJSON(ClientMessage{
			Type: TypePlace, Kind: string(p.kind), X: p.x, Y: p.y, Orientation: string(game.Horizontal),
		}); err != nil {
			t.Fatalf("write place: %v", err)
		}
	}
	if err := conn.WriteJSON(ClientMessage{Type: TypeReady}); err != nil {
		t.Fatalf("write ready: %v", err)
	}
}

func TestJoinReturnsWelcomeWithPlayerID(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	if err := conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Ana"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, conn, TypeWelcome)
	if msg.PlayerID == "" {
		t.Fatal("welcome has empty playerId")
	}
	if msg.Room == "" {
		t.Fatal("welcome has empty room")
	}
}

func TestPingReturnsPong(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	if err := conn.WriteJSON(ClientMessage{Type: TypePing}); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, conn, TypePong)
	if msg.Type != TypePong {
		t.Fatalf("type = %q, want pong", msg.Type)
	}
}

func TestUnknownMessageTypeReturnsError(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	if err := conn.WriteJSON(ClientMessage{Type: "nonsense"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, conn, TypeError)
	if msg.Message == "" {
		t.Fatal("error has empty message")
	}
}

func TestThirdPlayerIsRejected(t *testing.T) {
	srv := newTestServer(t)

	c1, c2, c3 := srv.dial(t), srv.dial(t), srv.dial(t)
	c1.WriteJSON(ClientMessage{Type: TypeJoin, Name: "A"})
	readMessage(t, c1, TypeWelcome)
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "B"})
	readMessage(t, c2, TypeWelcome)
	c3.WriteJSON(ClientMessage{Type: TypeJoin, Name: "C"})

	msg := readMessage(t, c3, TypeError)
	if msg.Message == "" {
		t.Fatal("error has empty message")
	}
}

func TestPlaceShipIsReflectedInTheState(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Ana"})
	readMessage(t, conn, TypeWelcome)

	conn.WriteJSON(ClientMessage{
		Type: TypePlace, Kind: string(game.Cruiser), X: 2, Y: 3, Orientation: string(game.Horizontal),
	})

	snap := readState(t, conn)
	if len(snap.Own.Ships) != 1 {
		t.Fatalf("own ships = %d, want 1", len(snap.Own.Ships))
	}
	if snap.Own.Ships[0].Kind != game.Cruiser {
		t.Fatalf("ship kind = %q, want %q", snap.Own.Ships[0].Kind, game.Cruiser)
	}
}

func TestInvalidPlacementReturnsError(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Ana"})
	readMessage(t, conn, TypeWelcome)

	// A carrier of 5 does not fit starting at column 7.
	conn.WriteJSON(ClientMessage{
		Type: TypePlace, Kind: string(game.Carrier), X: 7, Y: 0, Orientation: string(game.Horizontal),
	})

	msg := readMessage(t, conn, TypeError)
	if msg.Message == "" {
		t.Fatal("error has empty message")
	}
}

func TestStateMessageIsBroadcastToJoinedPlayers(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Ana"})
	readMessage(t, conn, TypeWelcome)

	snap := readState(t, conn)
	if len(snap.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(snap.Players))
	}
	if snap.Phase != game.PhaseSetup {
		t.Fatalf("phase = %q, want %q", snap.Phase, game.PhaseSetup)
	}
}

func TestStartReturnsTheMatchToSetup(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	c1.WriteJSON(ClientMessage{Type: TypeJoin, Name: "A"})
	readMessage(t, c1, TypeWelcome)
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "B"})
	readMessage(t, c2, TypeWelcome)

	placeFleet(t, c1)
	placeFleet(t, c2)

	deadline := time.Now().Add(3 * time.Second)
	started := false
	for time.Now().Before(deadline) && !started {
		snap := readState(t, c1)
		started = snap.Phase == game.PhasePlaying
	}
	if !started {
		t.Fatal("match never started")
	}

	c1.WriteJSON(ClientMessage{Type: TypeStart})

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := readState(t, c1)
		if snap.Phase == game.PhaseSetup {
			return
		}
	}
	t.Fatal("match never returned to setup")
}

func TestDecodeClientMessage(t *testing.T) {
	var msg ClientMessage
	if err := json.Unmarshal([]byte(`{"type":"shoot","x":3,"y":4}`), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != TypeShoot || msg.X != 3 || msg.Y != 4 {
		t.Fatalf("msg = %+v", msg)
	}
}

func TestDecodePlaceMessage(t *testing.T) {
	var msg ClientMessage
	raw := `{"type":"place","kind":"cruiser","x":1,"y":2,"orientation":"vertical"}`
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != TypePlace || msg.Kind != "cruiser" || msg.X != 1 || msg.Y != 2 || msg.Orientation != "vertical" {
		t.Fatalf("msg = %+v", msg)
	}
}
