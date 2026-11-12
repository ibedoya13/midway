package game

import (
	"encoding/json"
	"testing"
)

// TestSnapshotSerializesEmptyCollectionsAsArrays makes sure the JSON sent to
// the browser never contains null for the list fields. The frontend iterates
// those arrays directly, so a null breaks the canvas rendering.
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
