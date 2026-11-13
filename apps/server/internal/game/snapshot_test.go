package game

import (
	"encoding/json"
	"testing"
)

// TestSnapshotSerializesEmptyCollectionsAsArrays makes sure the JSON sent to
// the browser never contains null for the list fields. The frontend iterates
// those arrays directly, so a null breaks the canvas rendering.
// placeFullFleet places the five ships of a player.
func placeFullFleet(g *Game, id string) {
	placements := []struct {
		kind ShipKind
		x, y int
	}{
		{Carrier, 0, 0}, {Battleship, 0, 2}, {Cruiser, 0, 4},
		{Submarine, 0, 6}, {Destroyer, 0, 8},
	}
	for _, p := range placements {
		g.PlaceShip(id, p.kind, p.x, p.y, Horizontal)
	}
}

// TestSnapshotOfAnUnknownPlayerDoesNotRevealAnOpponent checks that a view
// built for an id that is not in the match does not fall back to a real
// player: that would leak the opponent board to an outsider.
func TestSnapshotOfAnUnknownPlayerDoesNotRevealAnOpponent(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")

	// Both players place a fleet and confirm, so the match starts.
	for _, id := range []string{"p1", "p2"} {
		placeFullFleet(g, id)
		g.Ready(id)
	}

	// Both players fire once, so each board holds the shots of the other.
	g.Shoot("p1", 3, 4)
	g.Shoot("p2", 5, 6)

	// The real opponent view carries the shots of the other player.
	real := g.SnapshotFor("p1")
	if len(real.Enemy.Shots) == 0 {
		t.Fatal("the opponent view must carry the shots of the other player")
	}
	if len(real.Own.Shots) == 0 {
		t.Fatal("the own view must carry the shots received")
	}

	// An unknown id must not see the shots of any real player.
	unknown := g.SnapshotFor("intruso")
	if len(unknown.Enemy.Shots) != 0 {
		t.Fatalf("an unknown id received %d shots of a real player", len(unknown.Enemy.Shots))
	}
	if len(unknown.Own.Shots) != 0 {
		t.Fatalf("an unknown id received %d own shots", len(unknown.Own.Shots))
	}
	if len(unknown.Players) != 0 {
		t.Fatalf("an unknown id received %d players", len(unknown.Players))
	}
	if len(unknown.ReadyPlayers) != 0 {
		t.Fatalf("an unknown id received %d ready players", len(unknown.ReadyPlayers))
	}
}

func TestSnapshotSerializesEmptyCollectionsAsArrays(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")

	snap := g.SnapshotFor("p1")
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, board := range []string{"own", "enemy"} {
		view, ok := decoded[board].(map[string]any)
		if !ok {
			t.Fatalf("%s board missing", board)
		}
		for _, field := range []string{"ships", "sunkShips", "shots"} {
			value, ok := view[field]
			if !ok {
				t.Fatalf("%s.%s missing", board, field)
			}
			if value == nil {
				t.Fatalf("%s.%s is null, want an array", board, field)
			}
			list, ok := value.([]any)
			if !ok {
				t.Fatalf("%s.%s is %T, want an array", board, field, value)
			}
			_ = list
		}
	}
}

// TestSnapshotOfUnknownPlayerSerializesArrays covers the case of a client
// that has not joined yet: it still receives snapshots, so the arrays must be
// arrays too.
func TestSnapshotOfUnknownPlayerSerializesArrays(t *testing.T) {
	g := New(10, 10)

	raw, err := json.Marshal(g.SnapshotFor("ghost"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(raw); contains(got, `"ships":null`) || contains(got, `"sunkShips":null`) || contains(got, `"shots":null`) {
		t.Fatalf("snapshot contains a null list: %s", got)
	}
}

// TestSnapshotWithShipsSerializesArrays checks the populated case.
func TestSnapshotWithShipsSerializesArrays(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.AddPlayer("p2", "Beto")
	g.PlaceShip("p1", Cruiser, 2, 3, Horizontal)

	raw, err := json.Marshal(g.SnapshotFor("p1"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if contains(string(raw), `null`) {
		t.Fatalf("snapshot contains a null value: %s", string(raw))
	}
}

// contains reports whether the haystack contains the needle.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

// indexOf returns the index of the needle in the haystack, or -1.
func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
