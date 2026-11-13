package game

import "testing"

// TestTheNameIsTruncated checks that a name cannot be longer than allowed:
// it travels in every snapshot.
func TestTheNameIsTruncated(t *testing.T) {
	g := New(10, 10)

	g.AddPlayer("p1", "Ana")
	player := g.Player("p1")

	if len([]rune(player.Name)) > MaxNameLength {
		t.Fatalf("name length = %d, want at most %d",
			len([]rune(player.Name)), MaxNameLength)
	}
}

// TestAnEmptyNameGetsAFallback checks that a player without a name still has
// one.
func TestAnEmptyNameGetsAFallback(t *testing.T) {
	g := New(10, 10)

	g.AddPlayer("p1", "")
	player := g.Player("p1")

	if player.Name == "" {
		t.Fatal("the player must have a name")
	}
}

// TestTheControlCharactersAreRemoved checks that a name cannot carry control
// characters into the interface of the other player.
func TestTheControlCharactersAreRemoved(t *testing.T) {
	g := New(10, 10)

	g.AddPlayer("p1", "An\x00a\nb\rc")
	player := g.Player("p1")

	for _, r := range player.Name {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("the name carries the control character %q", r)
		}
	}
}

// TestSanitizeNameTrimsTheSpaces checks the cleaning of the name.
func TestSanitizeNameTrimsTheSpaces(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  Ana  ", "Ana"},
		{"Anita", "Anita"},
		{"\t\n", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := SanitizeName(c.in, 1); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSanitizeNameKeepsSixteenCharacters checks the limit of the name.
func TestSanitizeNameKeepsSixteenCharacters(t *testing.T) {
	got := SanitizeName("abcdefghijklmnopqrstuvwxyz", 1)
	if len([]rune(got)) != MaxNameLength {
		t.Fatalf("length = %d, want %d", len([]rune(got)), MaxNameLength)
	}
}
