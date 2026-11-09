package game

import (
	"sort"
	"testing"
)

// newMatch returns a game with two joined players in the setup phase.
func newMatch(t *testing.T) *Game {
	t.Helper()
	g := New(10, 10)
	if _, ok := g.AddPlayer("p1", "Ana"); !ok {
		t.Fatal("AddPlayer p1 failed")
	}
	if _, ok := g.AddPlayer("p2", "Beto"); !ok {
		t.Fatal("AddPlayer p2 failed")
	}
	return g
}

// readyFleet places the whole fleet for a player and marks it ready.
func readyFleet(t *testing.T, g *Game, id string) {
	t.Helper()
	placements := []struct {
		kind ShipKind
		x, y int
		o    Orientation
	}{
		{Carrier, 0, 0, Horizontal},
		{Battleship, 0, 2, Horizontal},
		{Cruiser, 0, 4, Horizontal},
		{Submarine, 0, 6, Horizontal},
		{Destroyer, 0, 8, Horizontal},
	}
	for _, p := range placements {
		if err := g.PlaceShip(id, p.kind, p.x, p.y, p.o); err != nil {
			t.Fatalf("PlaceShip %s: %v", p.kind, err)
		}
	}
	if err := g.Ready(id); err != nil {
		t.Fatalf("Ready: %v", err)
	}
}

func TestNewGameIsInSetupPhase(t *testing.T) {
	g := New(10, 10)

	if g.Phase() != PhaseSetup {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhaseSetup)
	}
	if len(g.Players()) != 0 {
		t.Fatalf("players = %d, want 0", len(g.Players()))
	}
}

func TestAddPlayerAssignsBoardAndFleet(t *testing.T) {
	g := New(10, 10)

	p, ok := g.AddPlayer("p1", "Ana")
	if !ok {
		t.Fatal("AddPlayer returned false")
	}
	if p.Name != "Ana" {
		t.Fatalf("name = %q, want Ana", p.Name)
	}
	if p.Board == nil {
		t.Fatal("player has no board")
	}
	if len(p.Fleet) != 5 {
		t.Fatalf("fleet = %d ships, want 5", len(p.Fleet))
	}
}

func TestAddPlayerRejectedWhenFull(t *testing.T) {
	g := New(10, 10)

	g.AddPlayer("p1", "A")
	g.AddPlayer("p2", "B")
	if _, ok := g.AddPlayer("p3", "C"); ok {
		t.Fatal("AddPlayer accepted a third player")
	}
}

func TestPlaceShipStoresItOnThePlayerBoard(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "A")
	g.AddPlayer("p2", "B")

	if err := g.PlaceShip("p1", Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	cells := g.Player("p1").Board.ShipCells(Cruiser)
	if len(cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(cells))
	}
}

func TestPlaceShipRejectsUnknownPlayer(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "A")

	if err := g.PlaceShip("ghost", Cruiser, 2, 3, Horizontal); err == nil {
		t.Fatal("PlaceShip accepted an unknown player")
	}
}

func TestReadyFailsWhenFleetIsIncomplete(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "A")
	g.AddPlayer("p2", "B")

	if err := g.PlaceShip("p1", Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}
	if err := g.Ready("p1"); err == nil {
		t.Fatal("Ready accepted an incomplete fleet")
	}
}

func TestGameStartsWhenBothPlayersAreReady(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "A")
	g.AddPlayer("p2", "B")

	readyFleet(t, g, "p1")
	if g.Phase() != PhaseSetup {
		t.Fatalf("phase = %q with one ready player, want %q", g.Phase(), PhaseSetup)
	}
	readyFleet(t, g, "p2")
	if g.Phase() != PhasePlaying {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhasePlaying)
	}
}

func TestFirstPlayerTakesTheFirstTurn(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	if g.CurrentTurn() != "p1" {
		t.Fatalf("current turn = %q, want p1", g.CurrentTurn())
	}
}

func TestShootOnOpponentBoard(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	result, err := g.Shoot("p1", 0, 0)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if result != Hit {
		t.Fatalf("result = %q, want %q", result, Hit)
	}
}

func TestShootOutOfTurnIsRejected(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	if _, err := g.Shoot("p2", 5, 5); err == nil {
		t.Fatal("Shoot accepted a player that is not on turn")
	}
}

func TestTurnPassesAfterShooting(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	if _, err := g.Shoot("p1", 0, 0); err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if g.CurrentTurn() != "p2" {
		t.Fatalf("current turn = %q, want p2", g.CurrentTurn())
	}
}

func TestShootOutOfBoundsIsRejected(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	if _, err := g.Shoot("p1", 10, 0); err == nil {
		t.Fatal("Shoot accepted a coordinate outside the board")
	}
}

func TestSunkShipIsReported(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	result, err := g.Shoot("p1", 0, 8)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if result != Hit {
		t.Fatalf("result = %q, want %q", result, Hit)
	}
	if _, err := g.Shoot("p2", 9, 9); err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	result, err = g.Shoot("p1", 1, 8)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if result != Sunk {
		t.Fatalf("result = %q, want %q", result, Sunk)
	}
}

func TestPlayerWinsWhenEnemyFleetIsDestroyed(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	// Sink every cell of p2's fleet while alternating turns.
	for i, c := range p2FleetCells() {
		turn := g.CurrentTurn()
		if turn != "p1" {
			answer := p2AnswerCell(i)
			if _, err := g.Shoot("p2", answer[0], answer[1]); err != nil {
				t.Fatalf("Shoot p2: %v", err)
			}
			turn = g.CurrentTurn()
		}
		if _, err := g.Shoot(turn, c[0], c[1]); err != nil {
			t.Fatalf("Shoot %s: %v", turn, err)
		}
	}

	if g.Phase() != PhaseGameOver {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhaseGameOver)
	}
	if g.Winner() != "p1" {
		t.Fatalf("winner = %q, want p1", g.Winner())
	}
}

func TestShootingAfterGameOverIsRejected(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	for i, c := range p2FleetCells() {
		turn := g.CurrentTurn()
		if turn != "p1" {
			answer := p2AnswerCell(i)
			g.Shoot("p2", answer[0], answer[1])
			turn = g.CurrentTurn()
		}
		g.Shoot(turn, c[0], c[1])
	}

	if _, err := g.Shoot(g.CurrentTurn(), 5, 5); err == nil {
		t.Fatal("Shoot accepted a shot after the game ended")
	}
}

func TestSnapshotHidesEnemyShips(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	snap := g.SnapshotFor("p1")
	if len(snap.Enemy.Ships) != 0 {
		t.Fatalf("enemy ships = %d, want 0 (hidden)", len(snap.Enemy.Ships))
	}
	if len(snap.Own.Ships) != 5 {
		t.Fatalf("own ships = %d, want 5", len(snap.Own.Ships))
	}
}

func TestSnapshotShowsEnemySunkShips(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	g.Shoot("p1", 0, 8)
	g.Shoot("p2", 9, 9)
	g.Shoot("p1", 1, 8)

	snap := g.SnapshotFor("p1")
	if len(snap.Enemy.SunkShips) != 1 {
		t.Fatalf("enemy sunk ships = %d, want 1", len(snap.Enemy.SunkShips))
	}
}

func TestSnapshotListsShots(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	g.Shoot("p1", 3, 3)

	snap := g.SnapshotFor("p1")
	if len(snap.Enemy.Shots) != 1 {
		t.Fatalf("enemy shots = %d, want 1", len(snap.Enemy.Shots))
	}
	if len(snap.Own.Shots) != 0 {
		t.Fatalf("own shots = %d, want 0", len(snap.Own.Shots))
	}
}

func TestResetReturnsToSetupPhase(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")
	g.Shoot("p1", 0, 0)

	g.Reset()

	if g.Phase() != PhaseSetup {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhaseSetup)
	}
	if g.CurrentTurn() != "" {
		t.Fatalf("current turn = %q, want empty", g.CurrentTurn())
	}
	if len(g.Player("p1").Board.PlacedShips()) != 0 {
		t.Fatal("reset kept the ships on the board")
	}
}

func TestRemovePlayerReturnsToSetupPhase(t *testing.T) {
	g := newMatch(t)
	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	g.RemovePlayer("p2")

	if g.Phase() != PhaseSetup {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhaseSetup)
	}
	if len(g.Players()) != 1 {
		t.Fatalf("players = %d, want 1", len(g.Players()))
	}
}

func TestCoordinatesParseColumnLetters(t *testing.T) {
	x, y, ok := ParseCoordinate("A1")
	if !ok {
		t.Fatal("ParseCoordinate rejected A1")
	}
	if x != 0 || y != 0 {
		t.Fatalf("A1 = (%d,%d), want (0,0)", x, y)
	}

	x, y, ok = ParseCoordinate("J10")
	if !ok {
		t.Fatal("ParseCoordinate rejected J10")
	}
	if x != 9 || y != 9 {
		t.Fatalf("J10 = (%d,%d), want (9,9)", x, y)
	}
}

func TestCoordinatesRejectInvalidInput(t *testing.T) {
	for _, input := range []string{"", "K1", "A0", "A11", "1A", "AA", "A1B"} {
		if _, _, ok := ParseCoordinate(input); ok {
			t.Fatalf("ParseCoordinate accepted %q", input)
		}
	}
}

// p2FleetCells returns every cell of the fleet used by readyFleet.
func p2FleetCells() [][2]int {
	cells := [][2]int{}
	for row, size := range map[int]int{0: 5, 2: 4, 4: 3, 6: 3, 8: 2} {
		for i := 0; i < size; i++ {
			cells = append(cells, [2]int{i, row})
		}
	}
	// Keep a deterministic order.
	sort.Slice(cells, func(i, j int) bool {
		if cells[i][1] != cells[j][1] {
			return cells[i][1] < cells[j][1]
		}
		return cells[i][0] < cells[j][0]
	})
	return cells
}

// p2AnswerCell returns a free cell of p1's board for the opponent's answer.
// It walks the last columns so the answers never repeat.
func p2AnswerCell(index int) [2]int {
	return [2]int{9 - index/10, index % 10}
}
