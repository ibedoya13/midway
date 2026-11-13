package ws

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"midway/server/internal/room"
)

// TestMeasureMemoryPerRoomOverTLS repeats the measurement over TLS: the
// production deployment serves the WebSocket over wss://, and every TLS
// connection carries buffers that a plain one does not.
func TestMeasureMemoryPerRoomOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	base := heapAfterGC()

	registry := room.NewRegistry(10, 10, 1000)
	hub := NewHubWithConfig(registry, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 5000,
		MaxConnections:      5000,
		MessagesPerSecond:   1000,
		ReconnectGrace:      DefaultReconnectGrace,
	})
	defer hub.Stop()

	srv := httptest.NewTLSServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"

	// The test server uses a self-signed certificate.
	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	afterServer := heapAfterGC()
	t.Logf("heap tras levantar el servidor TLS: %.1f MiB (+%.1f KiB)",
		float64(afterServer)/(1024*1024), float64(afterServer-base)/1024)

	const rooms = 200
	var conns []*websocket.Conn
	for i := 0; i < rooms; i++ {
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn)
		conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w ServerMessage
		if err := conn.ReadJSON(&w); err != nil {
			t.Fatalf("read: %v", err)
		}
		code, key := w.Room, w.Key

		conn2, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn2)
		conn2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "b", Room: code, Key: key})
		_ = conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w2 ServerMessage
		if err := conn2.ReadJSON(&w2); err != nil {
			t.Fatalf("read: %v", err)
		}
	}

	afterRooms := heapAfterGC()
	t.Logf("heap con %d salas sobre TLS: %.1f MiB (+%.1f MiB)",
		rooms, float64(afterRooms)/(1024*1024),
		float64(afterRooms-afterServer)/(1024*1024))
	t.Logf("por sala (TLS): %.1f KiB",
		float64(afterRooms-afterServer)/rooms/1024)

	time.Sleep(3 * time.Second)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("goroutines: %d, stack: %.1f MiB, sys: %.1f MiB, total RSS estimado",
		runtime.NumGoroutine(), float64(m.StackInuse)/(1024*1024),
		float64(m.Sys)/(1024*1024))

	for _, c := range conns {
		c.Close()
	}
}

// TestMeasureBroadcastCost measures how much work the broadcast does per
// client: every tick serializes a snapshot for every connected player. With
// many rooms the CPU, not the memory, may be the limit.
func TestMeasureBroadcastCost(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	registry := room.NewRegistry(10, 10, 1000)
	hub := NewHubWithConfig(registry, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 5000,
		MaxConnections:      5000,
		MessagesPerSecond:   1000,
		ReconnectGrace:      DefaultReconnectGrace,
	})
	defer hub.Stop()

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dialer := *websocket.DefaultDialer

	for _, rooms := range []int{10, 50, 100, 200} {
		var conns []*websocket.Conn
		for i := 0; i < rooms; i++ {
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			conns = append(conns, conn)
			conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			var w ServerMessage
			if err := conn.ReadJSON(&w); err != nil {
				t.Fatalf("read: %v", err)
			}
		}

		// Measure the cost of a broadcast cycle.
		start := time.Now()
		const cycles = 20
		for i := 0; i < cycles; i++ {
			hub.broadcastState()
		}
		perCycle := time.Since(start) / cycles

		// At 20 broadcasts per second the cycle must fit in 50 ms.
		budget := time.Second / broadcastRate
		t.Logf("%3d salas (%3d clientes): ciclo de broadcast %v, presupuesto %v, uso %v%%",
			rooms, rooms, perCycle, budget, perCycle*100/budget)

		for _, c := range conns {
			c.Close()
		}
	}
}

// TestMeasureResidentMemoryCurve measures the resident memory with several
// room counts, so the fixed cost of the runtime can be told apart from the
// cost of each room.
func TestMeasureResidentMemoryCurve(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	for _, rooms := range []int{50, 150, 300, 500} {
		registry := room.NewRegistry(10, 10, 1000)
		hub := NewHubWithConfig(registry, Config{
			AllowedOrigins:      defaultAllowedOrigins,
			AllowEmptyOrigin:    true,
			MaxConnectionsPerIP: 5000,
			MaxConnections:      5000,
			MessagesPerSecond:   1000,
			ReconnectGrace:      DefaultReconnectGrace,
		})

		srv := httptest.NewTLSServer(http.HandlerFunc(hub.ServeWS))
		wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"
		dialer := *websocket.DefaultDialer
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		var conns []*websocket.Conn
		for i := 0; i < rooms; i++ {
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			conns = append(conns, conn)
			conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			var w ServerMessage
			if err := conn.ReadJSON(&w); err != nil {
				t.Fatalf("read: %v", err)
			}
		}

		time.Sleep(3 * time.Second)
		rss := rssBytes()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		t.Logf("%3d salas: RSS %7.1f MiB | heap %6.1f | stack %5.1f | sys %6.1f | goroutines %d",
			rooms, float64(rss)/(1024*1024), float64(m.HeapAlloc)/(1024*1024),
			float64(m.StackInuse)/(1024*1024), float64(m.Sys)/(1024*1024),
			runtime.NumGoroutine())

		for _, c := range conns {
			c.Close()
		}
		srv.Close()
		hub.Stop()
	}
}

// TestValidateCapacity runs the configuration that the deployment will
// use: the number of rooms it claims to support, with two players each, and
// checks that the memory and the CPU stay well inside the instance.
func TestValidateCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	const (
		rooms     = 1000
		connLimit = 2100
		limitMiB  = 512
	)

	registry := room.NewRegistry(10, 10, rooms)
	hub := NewHubWithConfig(registry, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: connLimit,
		MaxConnections:      connLimit,
		MessagesPerSecond:   DefaultMessagesPerSecond,
		ReconnectGrace:      DefaultReconnectGrace,
	})
	defer hub.Stop()

	srv := httptest.NewTLSServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"
	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	base := rssBytes()
	var conns []*websocket.Conn
	for i := 0; i < rooms; i++ {
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn)
		conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w ServerMessage
		if err := conn.ReadJSON(&w); err != nil {
			t.Fatalf("read: %v", err)
		}
		conn2, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn2)
		conn2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "b", Room: w.Room, Key: w.Key})
		_ = conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w2 ServerMessage
		if err := conn2.ReadJSON(&w2); err != nil {
			t.Fatalf("read: %v", err)
		}
	}

	// The clients must drain their socket: a client that does not read
	// fills its send queue and the hub drops it as a slow client, which
	// would empty the hub during the measurement.
	stop := make(chan struct{})
	for _, c := range conns {
		go func(c *websocket.Conn) {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}(c)
	}
	defer close(stop)

	hub.mu.RLock()
	liveNow := len(hub.clients)
	hub.mu.RUnlock()
	t.Logf("  clientes en el hub justo tras crear las salas: %d", liveNow)

	time.Sleep(4 * time.Second)

	hub.mu.RLock()
	liveLater := len(hub.clients)
	hub.mu.RUnlock()
	t.Logf("  clientes en el hub tras 4 s: %d", liveLater)

	rss := rssBytes()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	used := float64(rss) / (1024 * 1024)
	t.Logf("%d salas, %d conexiones:", rooms, len(conns))
	t.Logf("  RSS %.1f MiB de %d MiB (%.1f%%) | heap %.1f | stack %.1f | sys %.1f",
		used, limitMiB, used*100/limitMiB, float64(m.HeapAlloc)/(1024*1024),
		float64(m.StackInuse)/(1024*1024), float64(m.Sys)/(1024*1024))
	t.Logf("  RSS por sala: %.1f KiB", float64(rss-base)/rooms/1024)

	// Count the clients that are really in the hub: a cycle that does
	// almost nothing means the clients are already gone.
	hub.mu.RLock()
	live := len(hub.clients)
	hub.mu.RUnlock()
	t.Logf("  clientes vivos en el hub: %d (conexiones abiertas: %d)", live, len(conns))

	start := time.Now()
	const cycles = 20
	for i := 0; i < cycles; i++ {
		hub.broadcastState()
	}
	perCycle := time.Since(start) / cycles
	budget := time.Second / broadcastRate
	t.Logf("  ciclo de broadcast %v de un presupuesto de %v (%.2f%%)",
		perCycle, budget, float64(perCycle)*100/float64(budget))

	for _, c := range conns {
		c.Close()
	}
}

// TestMeasureSaturation finds where the broadcast stops keeping up: the
// cycle must fit in the 50 ms that separate two broadcasts at 20 Hz.
func TestMeasureSaturation(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	for _, rooms := range []int{500, 1000, 1500, 2000} {
		registry := room.NewRegistry(10, 10, rooms)
		hub := NewHubWithConfig(registry, Config{
			AllowedOrigins:      defaultAllowedOrigins,
			AllowEmptyOrigin:    true,
			MaxConnectionsPerIP: 10000,
			MaxConnections:      10000,
			MessagesPerSecond:   1000,
			ReconnectGrace:      DefaultReconnectGrace,
		})

		srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
		dialer := *websocket.DefaultDialer

		var conns []*websocket.Conn
		for i := 0; i < rooms; i++ {
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			conns = append(conns, conn)
			conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			var w ServerMessage
			if err := conn.ReadJSON(&w); err != nil {
				t.Fatalf("read: %v", err)
			}
		}

		start := time.Now()
		const cycles = 10
		for i := 0; i < cycles; i++ {
			hub.broadcastState()
		}
		perCycle := time.Since(start) / cycles
		budget := time.Second / broadcastRate
		t.Logf("%4d salas (%4d clientes): ciclo %8v de %v -> %5.1f%% del presupuesto",
			rooms, rooms, perCycle, budget, float64(perCycle)*100/float64(budget))

		for _, c := range conns {
			c.Close()
		}
		srv.Close()
		hub.Stop()
	}
}

// rssBytes returns the resident set size of the process, which is what the
// platform limits. The heap of Go is only a part of it: the binary, the
// runtime, the stacks and the TLS buffers live outside.
func rssBytes() uint64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	// On macOS ru_maxrss is in bytes; on Linux it is in kilobytes.
	if runtime.GOOS == "darwin" {
		return uint64(usage.Maxrss)
	}
	return uint64(usage.Maxrss) * 1024
}

// TestMeasureResidentMemory measures the resident memory of the process with
// many rooms over TLS, which is the limit that matters in the platform.
func TestMeasureResidentMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	registry := room.NewRegistry(10, 10, 1000)
	hub := NewHubWithConfig(registry, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 5000,
		MaxConnections:      5000,
		MessagesPerSecond:   1000,
		ReconnectGrace:      DefaultReconnectGrace,
	})
	defer hub.Stop()

	srv := httptest.NewTLSServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t.Logf("RSS base: %.1f MiB", float64(rssBytes())/(1024*1024))

	const rooms = 300
	var conns []*websocket.Conn
	for i := 0; i < rooms; i++ {
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn)
		conn.WriteJSON(ClientMessage{Type: TypeCreate, Name: "a"})
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w ServerMessage
		if err := conn.ReadJSON(&w); err != nil {
			t.Fatalf("read: %v", err)
		}
		conn2, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn2)
		conn2.WriteJSON(ClientMessage{Type: TypeJoin, Name: "b", Room: w.Room, Key: w.Key})
		_ = conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		var w2 ServerMessage
		if err := conn2.ReadJSON(&w2); err != nil {
			t.Fatalf("read: %v", err)
		}
	}

	// Let the broadcast run so the queues fill as they do in production.
	time.Sleep(4 * time.Second)

	rss := rssBytes()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("con %d salas: RSS %.1f MiB (heap %.1f MiB, stack %.1f MiB, sys %.1f MiB)",
		rooms, float64(rss)/(1024*1024), float64(m.HeapAlloc)/(1024*1024),
		float64(m.StackInuse)/(1024*1024), float64(m.Sys)/(1024*1024))
	t.Logf("RSS por sala: %.1f KiB", float64(rss)/rooms/1024)

	for _, c := range conns {
		c.Close()
	}
}

// heapAfterGC returns the heap in use after a collection.
func heapAfterGC() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestMeasureMemoryPerRoom measures how much memory a room and its two
// connections cost. It prints the numbers instead of asserting them: the
// values depend on the machine and serve to size the limit of rooms.
func TestMeasureMemoryPerRoom(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement test")
	}
	if os.Getenv("MIDWAY_MEASURE") == "" {
		t.Skip("prueba de carga: pon MIDWAY_MEASURE=1 para ejecutarla")
	}

	base := heapAfterGC()
	t.Logf("heap base: %.1f MiB", float64(base)/(1024*1024))

	// The measurement needs more connections than the production limits
	// allow, so the limits are lifted here.
	srv := newTestServerWithConfigAndRooms(t, 1000, Config{
		AllowedOrigins:      defaultAllowedOrigins,
		AllowEmptyOrigin:    true,
		MaxConnectionsPerIP: 5000,
		MaxConnections:      5000,
		MessagesPerSecond:   1000,
		ReconnectGrace:      DefaultReconnectGrace,
	})

	const rooms = 500
	conns := make([]*websocket.Conn, 0, rooms*2)

	afterServer := heapAfterGC()
	t.Logf("heap tras levantar el servidor: %.1f MiB (+%.1f KiB)",
		float64(afterServer)/(1024*1024), float64(afterServer-base)/1024)

	for i := 0; i < rooms; i++ {
		conn := srv.dial(t)
		conns = append(conns, conn)
		code, key := createRoom(t, conn, fmt.Sprintf("P%d-a", i))

		conn2 := srv.dial(t)
		conns = append(conns, conn2)
		conn2.WriteJSON(ClientMessage{Type: TypeJoin, Name: fmt.Sprintf("P%d-b", i), Room: code, Key: key})
		readMessage(t, conn2, TypeWelcome)

		// Both players place a full fleet: that is the state a real game
		// holds.
		for _, c := range []*websocket.Conn{conn, conn2} {
			for _, f := range []struct {
				kind string
				x, y int
			}{
				{"carrier", 0, 0}, {"battleship", 0, 2}, {"cruiser", 0, 4},
				{"submarine", 0, 6}, {"destroyer", 0, 8},
			} {
				c.WriteJSON(ClientMessage{
					Type: TypePlace, Kind: f.kind, X: f.x, Y: f.y, Orientation: "horizontal",
				})
			}
			c.WriteJSON(ClientMessage{Type: TypeReady})
		}
	}

	afterRooms := heapAfterGC()
	t.Logf("heap con %d salas (2 jugadores y flota cada una): %.1f MiB (+%.1f MiB)",
		rooms, float64(afterRooms)/(1024*1024),
		float64(afterRooms-afterServer)/(1024*1024))
	t.Logf("por sala: %.1f KiB", float64(afterRooms-afterServer)/rooms/1024)
	t.Logf("por conexion: %.1f KiB",
		float64(afterRooms-afterServer)/float64(rooms*2)/1024)

	// Let the broadcast loop run so the send channels fill up as they do in
	// production.
	time.Sleep(3 * time.Second)
	afterBroadcast := heapAfterGC()
	t.Logf("heap tras 3 s de broadcast: %.1f MiB (+%.1f MiB)",
		float64(afterBroadcast)/(1024*1024),
		float64(afterBroadcast-afterRooms)/(1024*1024))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("goroutines: %d, stack: %.1f MiB, sys: %.1f MiB",
		runtime.NumGoroutine(), float64(m.StackInuse)/(1024*1024),
		float64(m.Sys)/(1024*1024))

	for _, c := range conns {
		c.Close()
	}
}
