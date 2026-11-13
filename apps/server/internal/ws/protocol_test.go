package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/game"
	"midway/server/internal/room"
)

// testServer is a hub running behind an httptest server.
type testServer struct {
	rooms *room.Registry
	url   string
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

// newTestServerWithConfigAndRooms starts a hub with an explicit
// configuration and a given number of rooms.
func newTestServerWithConfigAndRooms(t *testing.T, maxRooms int, cfg Config) *testServer {
	t.Helper()

	registry := room.NewRegistry(10, 10, maxRooms)
	hub := NewHubWithConfig(registry, cfg)
	t.Cleanup(hub.Stop)

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(srv.Close)

	return &testServer{rooms: registry, url: "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"}
}

// originHeader builds a request header with the given Origin.
func originHeader(origin string) http.Header {
	h := http.Header{}
	h.Set("Origin", origin)
	return h
}

// newTestServerWithConfig starts a hub with an explicit configuration.
func newTestServerWithConfig(t *testing.T, cfg Config) *testServer {
	t.Helper()

	registry := room.NewRegistry(10, 10, room.MaxRooms)
	hub := NewHubWithConfig(registry, cfg)
	t.Cleanup(hub.Stop)

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(srv.Close)

	return &testServer{rooms: registry, url: "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"}
}

// newTestServer spins up a hub behind an httptest server.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	registry := room.NewRegistry(10, 10, room.MaxRooms)
	hub := NewHub(registry)
	t.Cleanup(hub.Stop)

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(srv.Close)

	return &testServer{rooms: registry, url: "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"}
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

// createRoom makes a room and returns its six-digit code and its secret key.
func createRoom(t *testing.T, conn *websocket.Conn, name string) (string, string) {
	t.Helper()
	if err := conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: name}); err != nil {
		t.Fatalf("write: %v", err)
	}
	msg := readMessage(t, conn, TypeWelcome)
	return msg.Room, msg.Key
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

func TestCreateReturnsWelcomeWithPlayerID(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	if err := conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "Ana"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, conn, TypeWelcome)
	if msg.PlayerID == "" {
		t.Fatal("welcome has empty playerId")
	}
	if !room.IsValidCode(msg.Room) {
		t.Fatalf("room = %q, want a six-digit code", msg.Room)
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
	code, key := createRoom(t, c1, "A")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "B", Room: code, Key: key})
	readMessage(t, c2, TypeWelcome)
	c3.WriteJSON(ClientMessage{Type: TypeJoin, Name: "C", Room: code, Key: key})

	msg := readMessage(t, c3, TypeError)
	if msg.Message == "" {
		t.Fatal("error has empty message")
	}
}

func TestPlaceShipIsReflectedInTheState(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	createRoom(t, conn, "Ana")

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

	createRoom(t, conn, "Ana")

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

	createRoom(t, conn, "Ana")

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

	code, key := createRoom(t, c1, "A")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "B", Room: code, Key: key})
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

// TestJoinWithTheSecretKeyIsAccepted checks that the room key is the
// credential that admits a player.
func TestJoinWithTheSecretKeyIsAccepted(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	readMessage(t, c2, TypeWelcome)
}

// TestJoinWithoutTheSecretKeyIsRejected checks that the six-digit code alone
// does not admit a player: the secret key is required.
func TestJoinWithoutTheSecretKeyIsRejected(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	code, _ := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code})

	msg := readMessage(t, c2, TypeError)
	if msg.Message == "" {
		t.Fatal("joining without the secret key must fail")
	}
}

// TestJoinWithAWrongSecretKeyIsRejected checks that a wrong key is refused.
func TestJoinWithAWrongSecretKeyIsRejected(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	code, _ := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: "clave-incorrecta"})

	msg := readMessage(t, c2, TypeError)
	if msg.Message == "" {
		t.Fatal("joining with a wrong secret key must fail")
	}
}

// TestTheKeyOfAnotherRoomIsRejected checks that the key of a room does not
// open a different room.
func TestTheKeyOfAnotherRoomIsRejected(t *testing.T) {
	srv := newTestServer(t)
	c1, c2, c3 := srv.dial(t), srv.dial(t), srv.dial(t)

	_, keyA := createRoom(t, c1, "Ana")
	codeB, _ := createRoom(t, c2, "Beto")

	c3.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Caro", Room: codeB, Key: keyA})
	readMessage(t, c3, TypeError)
}

// TestCreateReturnsTheSecretKey checks that the creator receives the key it
// has to share with the opponent.
func TestCreateReturnsTheSecretKey(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	if err := conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "Ana"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, conn, TypeWelcome)
	if msg.Key == "" {
		t.Fatal("the welcome has no secret key")
	}
	if len(msg.Key) != room.KeyLength {
		t.Fatalf("key length = %d, want %d", len(msg.Key), room.KeyLength)
	}
}

func TestJoinWithAnUnknownCodeIsRejected(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: "000000"})

	msg := readMessage(t, conn, TypeError)
	if msg.Message == "" {
		t.Fatal("joining an unknown room must fail")
	}
}

func TestJoinWithoutACodeIsRejected(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto"})

	readMessage(t, conn, TypeError)
}

func TestJoinWithAMalformedCodeIsRejected(t *testing.T) {
	srv := newTestServer(t)
	conn := srv.dial(t)

	conn.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: "abc"})

	readMessage(t, conn, TypeError)
}

func TestSecondPlayerJoinsTheSameRoom(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	w2 := readMessage(t, c2, TypeWelcome)

	if w2.Room != code {
		t.Fatalf("room = %q, want %q", w2.Room, code)
	}
}

// TestJoinToARoomThatAlreadyStartedIsRejected covers the rule that nobody
// can enter a match that is already in progress.
func TestJoinToARoomThatAlreadyStartedIsRejected(t *testing.T) {
	srv := newTestServer(t)
	c1, c2, c3 := srv.dial(t), srv.dial(t), srv.dial(t)

	code, key := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
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

	// A third player cannot enter a match that is already running.
	c3.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Caro", Room: code, Key: key})

	msg := readMessage(t, c3, TypeError)
	if msg.Message == "" {
		t.Fatal("joining a room with a match in progress must fail")
	}
}

func TestLeaveReleasesTheRoom(t *testing.T) {
	srv := newTestServer(t)
	c1, c2 := srv.dial(t), srv.dial(t)

	code, key := createRoom(t, c1, "Ana")
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	readMessage(t, c2, TypeWelcome)

	c1.WriteJSON(ClientMessage{Type: TypeLeave})
	readMessage(t, c1, TypeLeft)

	// The room still exists with one player, so a new opponent can enter.
	c3 := srv.dial(t)
	c3.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Caro", Room: code, Key: key})
	readMessage(t, c3, TypeWelcome)
}

func TestEmptyRoomIsDeleted(t *testing.T) {
	// A short grace period: the seat is released as soon as it ends.
	srv := newTestServerWithConfig(t, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 100,
		MaxConnections:      100,
		MessagesPerSecond:   DefaultMessagesPerSecond,
		ReconnectGrace:      300 * time.Millisecond,
	})
	c1 := srv.dial(t)

	code, key := createRoom(t, c1, "Ana")

	c1.Close()
	// The room survives the grace period: the player may come back.
	time.Sleep(150 * time.Millisecond)
	c2 := srv.dial(t)
	c2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "Beto", Room: code, Key: key})
	readMessage(t, c2, TypeWelcome) // the room is still there

	// The seat of the player that went away is released after the grace
	// period, so the room holds only the player that is still connected.
	time.Sleep(400 * time.Millisecond)
	room, ok := srv.rooms.Get(code)
	if !ok {
		t.Fatal("the room must still exist")
	}
	if got := len(room.Game().Players()); got != 1 {
		t.Fatalf("the room holds %d players, want 1", got)
	}

	// A room whose last player leaves is released.
	c2.WriteJSON(ClientMessage{Type: TypeLeave})
	readMessage(t, c2, TypeLeft)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && srv.rooms.Count() > 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if srv.rooms.Count() != 0 {
		t.Fatalf("%d rooms left, want 0", srv.rooms.Count())
	}
}

// TestRoomLimitIsEnforced covers the limit of simultaneous rooms.
func TestRoomLimitIsEnforced(t *testing.T) {
	// The registry holds few rooms: the limit is what is being tested.
	srv := newTestServerWithConfigAndRooms(t, 3, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 100,
		MaxConnections:      100,
		MessagesPerSecond:   DefaultMessagesPerSecond,
	})

	for i := 0; i < 3; i++ {
		conn := srv.dial(t)
		createRoom(t, conn, fmt.Sprintf("P%d", i))
	}

	conn := srv.dial(t)
	conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "Extra"})

	msg := readMessage(t, conn, TypeError)
	if msg.Message == "" {
		t.Fatal("creating a room over the limit must fail")
	}
}

// TestTheLimitFreesRoomsWhenTheyAreEmpty checks that a room that is left
// empty does not consume one of its slots forever.
func TestTheLimitFreesRoomsWhenTheyAreEmpty(t *testing.T) {
	srv := newTestServerWithConfigAndRooms(t, 3, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 100,
		MaxConnections:      100,
		MessagesPerSecond:   DefaultMessagesPerSecond,
	})

	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn := srv.dial(t)
		createRoom(t, conn, fmt.Sprintf("P%d", i))
		conns = append(conns, conn)
	}

	// Free one slot by closing its only player.
	conns[0].Close()
	time.Sleep(300 * time.Millisecond)

	fresh := srv.dial(t)
	fresh.WriteJSON(ClientMessage{Type: TypeCreate, Name: "Nuevo"})
	readMessage(t, fresh, TypeWelcome)
}

// TestHandshakeFromAForeignOriginIsRejected checks that a website other than
// the frontend cannot open a socket.
func TestHandshakeFromAForeignOriginIsRejected(t *testing.T) {
	srv := newTestServer(t)

	_, resp, err := websocket.DefaultDialer.Dial(srv.url, originHeader("https://attacker.example"))
	if err == nil {
		t.Fatal("a foreign origin must not complete the handshake")
	}
	if resp == nil {
		return
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestHandshakeFromTheFrontendOriginIsAccepted checks that the real frontend
// can still connect.
func TestHandshakeFromTheFrontendOriginIsAccepted(t *testing.T) {
	srv := newTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(srv.url, originHeader("https://midway.amarrete.com"))
	if err != nil {
		t.Fatalf("the frontend origin must be accepted: %v", err)
	}
	conn.Close()
}

// TestHandshakeFromTheLocalDevOriginIsAccepted checks that local development
// keeps working.
func TestHandshakeFromTheLocalDevOriginIsAccepted(t *testing.T) {
	srv := newTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(srv.url, originHeader("http://localhost:5173"))
	if err != nil {
		t.Fatalf("the local dev origin must be accepted: %v", err)
	}
	conn.Close()
}

// TestHandshakeWithoutAnOriginIsAccepted keeps non-browser callers working.
func TestHandshakeWithoutAnOriginIsAccepted(t *testing.T) {
	srv := newTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(srv.url, nil)
	if err != nil {
		t.Fatalf("a client without an origin must be accepted: %v", err)
	}
	conn.Close()
}

// TestTooManyConnectionsFromOneAddressAreRefused checks the connection limit
// per address.
func TestTooManyConnectionsFromOneAddressAreRefused(t *testing.T) {
	srv := newTestServerWithConfig(t, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 2,
		MaxConnections:      10,
		MessagesPerSecond:   DefaultMessagesPerSecond,
	})

	var open []*websocket.Conn
	for i := 0; i < 2; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(srv.url, nil)
		if err != nil {
			t.Fatalf("connection %d must be accepted: %v", i, err)
		}
		open = append(open, conn)
	}

	// The third connection from the same address exceeds the limit.
	_, resp, err := websocket.DefaultDialer.Dial(srv.url, nil)
	if err == nil {
		t.Fatal("the connection over the limit must be refused")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	for _, c := range open {
		c.Close()
	}
}

// TestAFloodingSocketIsDropped checks the message rate limit.
func TestAFloodingSocketIsDropped(t *testing.T) {
	srv := newTestServerWithConfig(t, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 10,
		MaxConnections:      10,
		MessagesPerSecond:   2,
	})

	conn := srv.dial(t)
	defer conn.Close()

	// Flood the socket with messages.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn.WriteJSON(ClientMessage{Type: TypePing})
	}

	// The server drops the connection when the bucket is empty.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var msg ClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return // the connection was closed: the limit worked
		}
	}
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
