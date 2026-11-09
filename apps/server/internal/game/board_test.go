package game

import (
	"testing"
)

// defaultBoard returns a 10x10 board, the size used by the rules.
func defaultBoard() *Board {
	return NewBoard(10, 10)
}

func TestFleetMatchesTheRules(t *testing.T) {
	fleet := DefaultFleet()
	want := map[ShipKind]int{
		Carrier:    5,
		Battleship: 4,
		Cruiser:    3,
		Submarine:  3,
		Destroyer:  2,
	}
	if len(fleet) != len(want) {
		t.Fatalf("fleet size = %d, want %d", len(fleet), len(want))
	}
	total := 0
	for _, ship := range fleet {
		size, ok := want[ship.Kind]
		if !ok {
			t.Fatalf("unexpected ship %q", ship.Kind)
		}
		if ship.Size != size {
			t.Fatalf("ship %q size = %d, want %d", ship.Kind, ship.Size, size)
		}
		total += ship.Size
	}
	if total != 17 {
		t.Fatalf("fleet cells = %d, want 17", total)
	}
}

func TestPlaceShipHorizontal(t *testing.T) {
	b := defaultBoard()

	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	cells := b.ShipCells(Cruiser)
	if len(cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(cells))
	}
	want := []Point{{X: 2, Y: 3}, {X: 3, Y: 3}, {X: 4, Y: 3}}
	for i, cell := range cells {
		if cell != want[i] {
			t.Fatalf("cell %d = %+v, want %+v", i, cell, want[i])
		}
	}
}

func TestPlaceShipVertical(t *testing.T) {
	b := defaultBoard()

	if err := b.PlaceShip(Cruiser, 2, 3, Vertical); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	cells := b.ShipCells(Cruiser)
	want := []Point{{X: 2, Y: 3}, {X: 2, Y: 4}, {X: 2, Y: 5}}
	for i, cell := range cells {
		if cell != want[i] {
			t.Fatalf("cell %d = %+v, want %+v", i, cell, want[i])
		}
	}
}

func TestPlaceShipOutOfBounds(t *testing.T) {
	b := defaultBoard()

	// A carrier of 5 starting at column 7 does not fit horizontally.
	if err := b.PlaceShip(Carrier, 7, 0, Horizontal); err == nil {
		t.Fatal("PlaceShip accepted a ship that leaves the board")
	}
	if err := b.PlaceShip(Cruiser, 0, 9, Vertical); err == nil {
		t.Fatal("PlaceShip accepted a ship that leaves the board")
	}
}

func TestPlaceShipOverlap(t *testing.T) {
	b := defaultBoard()

	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}
	if err := b.PlaceShip(Submarine, 4, 3, Horizontal); err == nil {
		t.Fatal("PlaceShip accepted an overlapping ship")
	}
	if err := b.PlaceShip(Submarine, 3, 3, Vertical); err == nil {
		t.Fatal("PlaceShip accepted an overlapping ship")
	}
}

func TestPlaceShipTwiceIsRejected(t *testing.T) {
	b := defaultBoard()

	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}
	if err := b.PlaceShip(Cruiser, 5, 5, Horizontal); err == nil {
		t.Fatal("PlaceShip accepted the same ship twice")
	}
}

func TestRemoveShipFreesItsCells(t *testing.T) {
	b := defaultBoard()

	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}
	if err := b.RemoveShip(Cruiser); err != nil {
		t.Fatalf("RemoveShip: %v", err)
	}
	if err := b.PlaceShip(Submarine, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip after removal: %v", err)
	}
}

func TestRemoveShipThatIsNotPlaced(t *testing.T) {
	b := defaultBoard()

	if err := b.RemoveShip(Cruiser); err == nil {
		t.Fatal("RemoveShip accepted a ship that was never placed")
	}
}

func TestShootOnEmptyCellIsWater(t *testing.T) {
	b := defaultBoard()

	result, err := b.Shoot(5, 5)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if result != Water {
		t.Fatalf("result = %q, want %q", result, Water)
	}
}

func TestShootOnShipIsHit(t *testing.T) {
	b := defaultBoard()
	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	result, err := b.Shoot(3, 3)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if result != Hit {
		t.Fatalf("result = %q, want %q", result, Hit)
	}
}

func TestShootLastCellOfShipIsSunk(t *testing.T) {
	b := defaultBoard()
	if err := b.PlaceShip(Cruiser, 0, 0, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	first, _ := b.Shoot(0, 0)
	second, _ := b.Shoot(1, 0)
	if first != Hit || second != Hit {
		t.Fatalf("results = %q/%q, want hit/hit", first, second)
	}
	third, err := b.Shoot(2, 0)
	if err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if third != Sunk {
		t.Fatalf("result = %q, want %q", third, Sunk)
	}
}

func TestShootSameCellTwiceIsRejected(t *testing.T) {
	b := defaultBoard()

	if _, err := b.Shoot(5, 5); err != nil {
		t.Fatalf("Shoot: %v", err)
	}
	if _, err := b.Shoot(5, 5); err == nil {
		t.Fatal("Shoot accepted the same cell twice")
	}
}

func TestShootOutOfBounds(t *testing.T) {
	b := defaultBoard()

	if _, err := b.Shoot(-1, 0); err == nil {
		t.Fatal("Shoot accepted a negative coordinate")
	}
	if _, err := b.Shoot(0, 10); err == nil {
		t.Fatal("Shoot accepted a row beyond the board")
	}
}

func TestAllSunkOnlyWhenEveryShipIsDestroyed(t *testing.T) {
	b := defaultBoard()
	if err := b.PlaceShip(Destroyer, 0, 0, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	b.Shoot(0, 0)
	if b.AllSunk() {
		t.Fatal("AllSunk is true with a partially destroyed ship")
	}
	b.Shoot(1, 0)
	if !b.AllSunk() {
		t.Fatal("AllSunk is false with every ship destroyed")
	}
}

func TestAllSunkIsFalseWithShipsStillAlive(t *testing.T) {
	b := defaultBoard()
	if err := b.PlaceShip(Destroyer, 0, 0, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}
	if err := b.PlaceShip(Cruiser, 5, 5, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	b.Shoot(0, 0)
	b.Shoot(1, 0)
	if b.AllSunk() {
		t.Fatal("AllSunk is true while the cruiser is alive")
	}
}

func TestShipAtFindsTheOwningShip(t *testing.T) {
	b := defaultBoard()
	if err := b.PlaceShip(Cruiser, 2, 3, Horizontal); err != nil {
		t.Fatalf("PlaceShip: %v", err)
	}

	kind, ok := b.ShipAt(3, 3)
	if !ok {
		t.Fatal("ShipAt found no ship on an occupied cell")
	}
	if kind != Cruiser {
		t.Fatalf("kind = %q, want %q", kind, Cruiser)
	}
	if _, ok := b.ShipAt(9, 9); ok {
		t.Fatal("ShipAt found a ship on an empty cell")
	}
}

func TestPlacedShipsListsEveryShip(t *testing.T) {
	b := defaultBoard()
	b.PlaceShip(Cruiser, 2, 3, Horizontal)
	b.PlaceShip(Destroyer, 7, 7, Vertical)

	ships := b.PlacedShips()
	if len(ships) != 2 {
		t.Fatalf("ships = %d, want 2", len(ships))
	}
}
