package game

import (
	"testing"
)

func TestSnapshotLobbyHasRosterNoView(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	up, err := s.SnapshotFor("h")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if up.Stage != Lobby || up.View != nil {
		t.Fatalf("lobby snapshot must have nil View, got stage=%v view=%v", up.Stage, up.View)
	}
	if len(up.Roster) != 2 || up.Roster[0].Name != "Host" || up.Roster[1].Name != "Bob" {
		t.Fatalf("roster wrong: %+v", up.Roster)
	}
}

func TestSnapshotPlayingHidesOpponents(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	_ = s.Start("h", 42)
	up, err := s.SnapshotFor("h")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if up.View == nil {
		t.Fatal("playing snapshot must carry a View")
	}
	if up.View.You != 0 {
		t.Fatalf("host is seat 0, got You=%d", up.View.You)
	}
	// D-9: opponents are counts only — there is no card field on OpponentView.
	if len(up.View.Opponents) != 1 {
		t.Fatalf("want 1 opponent, got %d", len(up.View.Opponents))
	}
	if up.View.Opponents[0].HandCount == 0 {
		t.Fatal("opponent hand count should be public and non-zero after dealing")
	}
}

func TestSnapshotUnknownPlayer(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if _, err := s.SnapshotFor("ghost"); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}
