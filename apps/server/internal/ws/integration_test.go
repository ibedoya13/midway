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
	"midway/server/internal/room"
)

// TestTwoPlayersPlayAFullBattleshipMatch drives a complete match between two
// real WebSocket clients: both join, both place their fleet secretly, the
// match starts, they alternate shooting, and a sunk ship becomes visible.
//
// The test is agnostic about which connection joined first: the turn order is
// decided by the server, so the roles are read from the snapshots.
func TestTwoPlayersPlayAFullBattleshipMatch(t *testing.T) {
	registry := room.NewRegistry(10, 10, room.MaxRooms)
	hub := NewHub(registry)
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dial := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}

	first, second := dial(), dial()
	defer first.Close()
	defer second.Close()

	send := func(conn *websocket.Conn, msg ClientMessage) {
		if err := conn.WriteJSON(msg); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	send(first, ClientMessage{Type: TypeCreate, Name: "Ana"})
	w1 := readMessage(t, first, TypeWelcome)
	roomCode := w1.Room
	send(second, ClientMessage{Type: TypeJoin, Name: "Beto", Room: roomCode, Key: w1.Key})
	w2 := readMessage(t, second, TypeWelcome)
	firstID := w1.PlayerID
	secondID := w2.PlayerID

	// Both players place the same fleet layout in secret.
	fleet := []struct {
		kind game.ShipKind
		x, y int
	}{
		{game.Carrier, 0, 0},
		{game.Battleship, 0, 2},
		{game.Cruiser, 0, 4},
		{game.Submarine, 0, 6},
		{game.Destroyer, 0, 8},
	}
	for _, conn := range []*websocket.Conn{first, second} {
		for _, f := range fleet {
			send(conn, ClientMessage{
				Type: TypePlace, Kind: string(f.kind), X: f.x, Y: f.y,
				Orientation: string(game.Horizontal),
			})
		}
		send(conn, ClientMessage{Type: TypeReady})
	}

	// Wait until the match is playing.
	playing := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !playing {
		snap := readState(t, first)
		playing = snap.Phase == game.PhasePlaying
	}
	if !playing {
		t.Fatal("match never reached the playing phase")
	}

	// The server decides who shoots first.
	starter := readState(t, first).CurrentTurn
	if starter != firstID && starter != secondID {
		t.Fatalf("current turn = %q, want one of the players", starter)
	}
	attacker, defender := first, second
	attackerID, defenderID := firstID, secondID
	if starter == secondID {
		attacker, defender = second, first
		attackerID, defenderID = secondID, firstID
	}

	// The attacker hits the carrier of the defender.
	waitForTurn(t, attacker, attackerID)
	send(attacker, ClientMessage{Type: TypeShoot, X: 0, Y: 0})
	var snap game.Snapshot
	shotSeen := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !shotSeen {
		snap = readState(t, attacker)
		shotSeen = len(snap.Enemy.Shots) == 1
	}
	if !shotSeen {
		t.Fatal("the shot never showed up on the enemy board")
	}
	if snap.Enemy.Shots[0].Result != game.Hit {
		t.Fatalf("shot result = %q, want %q", snap.Enemy.Shots[0].Result, game.Hit)
	}
	if snap.CurrentTurn == starter {
		t.Fatalf("current turn = %q, want the opponent after shooting", snap.CurrentTurn)
	}

	// The attacker must not see the enemy fleet before it is sunk.
	if len(snap.Enemy.Ships) != 0 {
		t.Fatalf("enemy ships = %d, want 0 (hidden)", len(snap.Enemy.Ships))
	}

	// The defender answers at a free cell of the attacker's board.
	waitForTurn(t, defender, defenderID)
	send(defender, ClientMessage{Type: TypeShoot, X: 9, Y: 9})
	readState(t, defender)

	// Sink the destroyer: two hits on a two-cell ship.
	waitForTurn(t, attacker, attackerID)
	send(attacker, ClientMessage{Type: TypeShoot, X: 0, Y: 8})
	readState(t, attacker)
	waitForTurn(t, defender, defenderID)
	send(defender, ClientMessage{Type: TypeShoot, X: 9, Y: 8})
	readState(t, defender)
	waitForTurn(t, attacker, attackerID)
	send(attacker, ClientMessage{Type: TypeShoot, X: 1, Y: 8})

	// The sunk ship is now visible to the attacker.
	sunk := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !sunk {
		snap = readState(t, attacker)
		sunk = len(snap.Enemy.SunkShips) == 1
	}
	if !sunk {
		t.Fatal("sunk ship never showed up on the enemy board")
	}
}

// waitForTurn waits until the match is waiting for the given player.
//
// The two players talk over different connections, so a shot sent right after
// the opponent's shot can arrive first and be refused. The test has to wait
// for the turn instead of assuming the order.
func waitForTurn(t *testing.T, conn *websocket.Conn, playerID string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := readState(t, conn)
		if snap.CurrentTurn == playerID {
			return
		}
	}
	t.Fatalf("the turn never came to %q", playerID)
}

// TestSnapshotMatchesProtocolShape ensures the state payload uses the field
// names the frontend expects.
func TestSnapshotMatchesProtocolShape(t *testing.T) {
	g := game.New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")

	payload, err := json.Marshal(ServerMessage{Type: TypeState, State: ptr(g.SnapshotFor("p1"))})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["type"] != TypeState {
		t.Fatalf("type = %v, want %q", decoded["type"], TypeState)
	}
	state, ok := decoded["state"].(map[string]any)
	if !ok {
		t.Fatal("state field missing")
	}
	for _, field := range []string{
		"phase", "width", "height", "players", "own", "enemy",
		"currentTurn", "winner", "yourFleet", "enemyFleet", "readyPlayers",
	} {
		if _, ok := state[field]; !ok {
			t.Fatalf("state field %q missing", field)
		}
	}
}

// ptr returns a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}
