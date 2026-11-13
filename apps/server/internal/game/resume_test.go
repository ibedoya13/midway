package game

import "testing"

// TestAddPlayerGivesAResumeToken checks that every player can resume.
func TestAddPlayerGivesAResumeToken(t *testing.T) {
	g := New(10, 10)

	p, ok := g.AddPlayer("p1", "Ana")
	if !ok {
		t.Fatal("the player must be added")
	}
	if p.ResumeToken == "" {
		t.Fatal("the player has no resume token")
	}
}

// TestResumeTokensAreUnique checks that two players never share a token.
func TestResumeTokensAreUnique(t *testing.T) {
	g := New(10, 10)

	a, _ := g.AddPlayer("p1", "Ana")
	b, _ := g.AddPlayer("p2", "Beto")

	if a.ResumeToken == b.ResumeToken {
		t.Fatal("two players share the same resume token")
	}
}

// TestResumePlayerReturnsTheAwayPlayer checks that a disconnected player can
// take its seat back.
func TestResumePlayerReturnsTheAwayPlayer(t *testing.T) {
	g := New(10, 10)
	p, _ := g.AddPlayer("p1", "Ana")

	g.SetDisconnected("p1", true)

	resumed, ok := g.ResumePlayer(p.ResumeToken)
	if !ok {
		t.Fatal("the player must be able to resume")
	}
	if resumed.ID != "p1" {
		t.Fatalf("resumed id = %q, want %q", resumed.ID, "p1")
	}
	if resumed.Disconnected {
		t.Fatal("the resumed player must not be marked as disconnected")
	}
}

// TestResumePlayerFailsForAConnectedPlayer checks that a token cannot be
// used to steal the seat of a player that is still connected.
func TestResumePlayerFailsForAConnectedPlayer(t *testing.T) {
	g := New(10, 10)
	p, _ := g.AddPlayer("p1", "Ana")

	if _, ok := g.ResumePlayer(p.ResumeToken); ok {
		t.Fatal("a connected player must not be resumable")
	}
}

// TestResumePlayerFailsWithAnUnknownToken checks that a made-up token is
// refused.
func TestResumePlayerFailsWithAnUnknownToken(t *testing.T) {
	g := New(10, 10)
	g.AddPlayer("p1", "Ana")
	g.SetDisconnected("p1", true)

	if _, ok := g.ResumePlayer("token-inventado"); ok {
		t.Fatal("an unknown token must be refused")
	}
	if _, ok := g.ResumePlayer(""); ok {
		t.Fatal("an empty token must be refused")
	}
}

// TestTheFleetSurvivesAResume checks that the player finds its fleet again.
func TestTheFleetSurvivesAResume(t *testing.T) {
	g := New(10, 10)
	p, _ := g.AddPlayer("p1", "Ana")
	g.PlaceShip("p1", Carrier, 0, 0, Horizontal)

	g.SetDisconnected("p1", true)
	resumed, ok := g.ResumePlayer(p.ResumeToken)
	if !ok {
		t.Fatal("the player must be able to resume")
	}

	snapshot := g.SnapshotFor(resumed.ID)
	if len(snapshot.Own.Ships) != 1 {
		t.Fatalf("the resumed player has %d ships, want 1", len(snapshot.Own.Ships))
	}
}

// TestTheResumeTokenIsNotBroadcast checks that the snapshot does not leak the
// token that admits a player back.
func TestTheResumeTokenIsNotBroadcast(t *testing.T) {
	g := New(10, 10)
	p, _ := g.AddPlayer("p1", "Ana")

	snapshot := g.SnapshotFor("p1")
	for _, player := range snapshot.Players {
		if player.ResumeToken != "" {
			t.Fatal("the snapshot must not carry the resume token")
		}
	}
	if snapshot.Players[0].ResumeToken == p.ResumeToken {
		t.Fatal("the snapshot must not carry the resume token")
	}
}
