package game

import "testing"

func TestRemoveShipUnconfirmsTheFleet(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")

	readyFleet(t, g, "p1")

	// Removing a ship must cancel the confirmation, otherwise the player
	// could not place it again: PlaceShip rejects changes on a ready fleet.
	if err := g.RemoveShip("p1", Cruiser); err != nil {
		t.Fatalf("RemoveShip: %v", err)
	}
	if g.Player("p1").Ready {
		t.Fatal("player is still ready after removing a ship")
	}

	if err := g.PlaceShip("p1", Cruiser, 0, 4, Horizontal); err != nil {
		t.Fatalf("PlaceShip after removing: %v", err)
	}
}

func TestPlaceShipIsRejectedAfterReady(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")

	readyFleet(t, g, "p1")

	if err := g.PlaceShip("p1", Carrier, 5, 5, Horizontal); err == nil {
		t.Fatal("PlaceShip accepted a ship on a confirmed fleet")
	}
}

func TestRemoveShipIsRejectedOnceTheMatchStarted(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")

	readyFleet(t, g, "p1")
	readyFleet(t, g, "p2")

	if g.Phase() != PhasePlaying {
		t.Fatalf("phase = %q, want %q", g.Phase(), PhasePlaying)
	}
	if err := g.RemoveShip("p1", Cruiser); err == nil {
		t.Fatal("RemoveShip accepted a ship once the match started")
	}
}
