// Package game implements the authoritative game simulation for Midway, a
// two-player Battleship game.
//
// Each player owns a hidden 10x10 board and a fleet of five ships. During the
// setup phase players place their fleet secretly; once both are ready the
// players alternate turns announcing coordinates, answering "water", "hit" or
// "sunk", until one fleet is completely destroyed.
package game

import (
	"fmt"
	"sync"
)

// ShipKind identifies a ship of the fleet.
type ShipKind string

// The fleet described in the rules.
const (
	Carrier    ShipKind = "carrier"
	Battleship ShipKind = "battleship"
	Cruiser    ShipKind = "cruiser"
	Submarine  ShipKind = "submarine"
	Destroyer  ShipKind = "destroyer"
)

// Orientation is the direction a ship occupies on the board.
type Orientation string

// Valid orientations.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

// Result is the outcome of a shot.
type Result string

// Shot outcomes.
const (
	Water Result = "water"
	Hit   Result = "hit"
	Sunk  Result = "sunk"
)

// Phase is the lifecycle stage of a match.
type Phase string

// Match phases.
const (
	PhaseSetup    Phase = "setup"
	PhasePlaying  Phase = "playing"
	PhaseGameOver Phase = "gameover"
)

// Point is a cell of the board. X is the column (0 = A), Y is the row (0 = 1).
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ShipSpec describes a ship type of the fleet.
type ShipSpec struct {
	Kind ShipKind `json:"kind"`
	Name string   `json:"name"`
	Size int      `json:"size"`
}

// DefaultFleet returns the fleet described in the rules.
func DefaultFleet() []ShipSpec {
	return []ShipSpec{
		{Kind: Carrier, Name: "Portaaviones", Size: 5},
		{Kind: Battleship, Name: "Acorazado", Size: 4},
		{Kind: Cruiser, Name: "Crucero", Size: 3},
		{Kind: Submarine, Name: "Submarino", Size: 3},
		{Kind: Destroyer, Name: "Destructor", Size: 2},
	}
}

// Board is a player's grid with its fleet and the shots it received.
type Board struct {
	width, height int

	// ships holds the placed ships indexed by kind.
	ships map[ShipKind]*Ship
	// shots maps a cell to the result of the shot fired there.
	shots map[Point]Result
	// hits counts the damaged cells per ship.
	hits map[ShipKind]int
}

// Ship is a placed ship.
type Ship struct {
	Kind        ShipKind    `json:"kind"`
	Name        string      `json:"name"`
	Size        int         `json:"size"`
	X           int         `json:"x"`
	Y           int         `json:"y"`
	Orientation Orientation `json:"orientation"`
}

// NewBoard creates an empty board.
func NewBoard(width, height int) *Board {
	return &Board{
		width:  width,
		height: height,
		ships:  make(map[ShipKind]*Ship),
		shots:  make(map[Point]Result),
		hits:   make(map[ShipKind]int),
	}
}

// PlaceShip puts a ship on the board. It fails when the ship leaves the board
// or overlaps another ship.
func (b *Board) PlaceShip(kind ShipKind, x, y int, o Orientation) error {
	spec, ok := specFor(kind)
	if !ok {
		return fmt.Errorf("barco desconocido: %s", kind)
	}
	if _, exists := b.ships[kind]; exists {
		return fmt.Errorf("el %s ya está colocado", spec.Name)
	}
	if o != Horizontal && o != Vertical {
		return fmt.Errorf("orientación inválida: %s", o)
	}

	ship := &Ship{Kind: kind, Name: spec.Name, Size: spec.Size, X: x, Y: y, Orientation: o}
	for _, cell := range shipCells(ship, b.width, b.height) {
		if !b.inBounds(cell.X, cell.Y) {
			return fmt.Errorf("el %s no cabe en el tablero", spec.Name)
		}
		for _, other := range b.ships {
			for _, occupied := range shipCells(other, b.width, b.height) {
				if occupied == cell {
					return fmt.Errorf("el %s se superpone con otro barco", spec.Name)
				}
			}
		}
	}

	b.ships[kind] = ship
	return nil
}

// RemoveShip takes a ship off the board.
func (b *Board) RemoveShip(kind ShipKind) error {
	if _, ok := b.ships[kind]; !ok {
		return fmt.Errorf("el %s no está colocado", kind)
	}
	delete(b.ships, kind)
	delete(b.hits, kind)
	return nil
}

// Shoot fires at a cell and returns the outcome: water, hit or sunk.
func (b *Board) Shoot(x, y int) (Result, error) {
	point := Point{X: x, Y: y}
	if !b.inBounds(x, y) {
		return "", fmt.Errorf("coordenada fuera del tablero")
	}
	if _, already := b.shots[point]; already {
		return "", fmt.Errorf("ya disparaste a esa casilla")
	}

	kind, found := b.ShipAt(x, y)
	if !found {
		b.shots[point] = Water
		return Water, nil
	}

	b.shots[point] = Hit
	b.hits[kind]++
	if b.hits[kind] >= b.shipSize(kind) {
		b.shots[point] = Sunk
		return Sunk, nil
	}
	return Hit, nil
}

// ShipAt returns the ship occupying a cell, if any.
func (b *Board) ShipAt(x, y int) (ShipKind, bool) {
	for kind, ship := range b.ships {
		for _, cell := range shipCells(ship, b.width, b.height) {
			if cell.X == x && cell.Y == y {
				return kind, true
			}
		}
	}
	return "", false
}

// ShipCells returns the cells occupied by a ship.
func (b *Board) ShipCells(kind ShipKind) []Point {
	ship, ok := b.ships[kind]
	if !ok {
		return nil
	}
	return shipCells(ship, b.width, b.height)
}

// PlacedShips returns the ships currently on the board.
func (b *Board) PlacedShips() []Ship {
	out := make([]Ship, 0, len(b.ships))
	for _, kind := range fleetOrder {
		if ship, ok := b.ships[kind]; ok {
			out = append(out, *ship)
		}
	}
	return out
}

// SunkShips returns the ships that have been completely destroyed.
func (b *Board) SunkShips() []Ship {
	out := make([]Ship, 0)
	for _, ship := range b.PlacedShips() {
		if b.hits[ship.Kind] >= ship.Size {
			out = append(out, ship)
		}
	}
	return out
}

// AllSunk reports whether every ship on the board has been destroyed.
func (b *Board) AllSunk() bool {
	if len(b.ships) == 0 {
		return false
	}
	for kind, size := range b.hits {
		if size < b.shipSize(kind) {
			return false
		}
	}
	for kind := range b.ships {
		if b.hits[kind] < b.shipSize(kind) {
			return false
		}
	}
	return true
}

// Shots returns the shots fired at this board.
func (b *Board) Shots() map[Point]Result {
	out := make(map[Point]Result, len(b.shots))
	for point, result := range b.shots {
		out[point] = result
	}
	return out
}

// HitAt reports the shot result on a cell, if any.
func (b *Board) HitAt(x, y int) (Result, bool) {
	result, ok := b.shots[Point{X: x, Y: y}]
	return result, ok
}

// inBounds reports whether a cell is inside the board.
func (b *Board) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < b.width && y < b.height
}

// shipSize returns the size of a placed ship.
func (b *Board) shipSize(kind ShipKind) int {
	if ship, ok := b.ships[kind]; ok {
		return ship.Size
	}
	if spec, ok := specFor(kind); ok {
		return spec.Size
	}
	return 0
}

// fleetOrder is the canonical order of the fleet.
var fleetOrder = []ShipKind{Carrier, Battleship, Cruiser, Submarine, Destroyer}

// specFor returns the fleet spec of a ship kind.
func specFor(kind ShipKind) (ShipSpec, bool) {
	for _, spec := range DefaultFleet() {
		if spec.Kind == kind {
			return spec, true
		}
	}
	return ShipSpec{}, false
}

// shipCells returns the cells a ship occupies, clipping to the board.
func shipCells(ship *Ship, width, height int) []Point {
	cells := make([]Point, 0, ship.Size)
	for i := 0; i < ship.Size; i++ {
		x, y := ship.X, ship.Y
		if ship.Orientation == Horizontal {
			x += i
		} else {
			y += i
		}
		if x < 0 || y < 0 || x >= width || y >= height {
			// Keep the cell so callers can detect the overflow.
			cells = append(cells, Point{X: x, Y: y})
			continue
		}
		cells = append(cells, Point{X: x, Y: y})
	}
	return cells
}

// BoardSize returns the dimensions of a board.
func (b *Board) BoardSize() (int, int) {
	return b.width, b.height
}

// mu guards concurrent access to a board.
var _ = sync.RWMutex{}
