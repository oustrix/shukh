package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func cfg36() Config {
	return Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
}

func TestLobbyJoin(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if s.Stage() != Lobby {
		t.Fatalf("new session must be in Lobby, got %v", s.Stage())
	}
	if _, ok := s.seatOf("h"); !ok {
		t.Fatal("host must occupy a seat")
	}
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatalf("join rejected: %v", err)
	}
	if err := s.Join("p2", "Bob again"); err != ErrDuplicate {
		t.Fatalf("duplicate join: want ErrDuplicate, got %v", err)
	}
}

func TestLobbyFull(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	// host + 7 more = 8 (D-3 max)
	for i := 0; i < 7; i++ {
		if err := s.Join(PlayerID(rune('a'+i)), "x"); err != nil {
			t.Fatalf("join %d rejected: %v", i, err)
		}
	}
	if err := s.Join("overflow", "x"); err != ErrFull {
		t.Fatalf("9th join: want ErrFull, got %v", err)
	}
}

func TestLeaveInLobby(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	s.Leave("p2")
	if _, ok := s.seatOf("p2"); ok {
		t.Fatal("left player must lose its seat in Lobby")
	}
}

func TestStartRequiresHostAndTwoPlayers(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Start("h", 1); err != ErrTooFewPlayers {
		t.Fatalf("solo start: want ErrTooFewPlayers, got %v", err)
	}
	_ = s.Join("p2", "Bob")
	if err := s.Start("p2", 1); err != ErrNotHost {
		t.Fatalf("non-host start: want ErrNotHost, got %v", err)
	}
	if err := s.Start("h", 42); err != nil {
		t.Fatalf("host start rejected: %v", err)
	}
	if s.Stage() != Playing {
		t.Fatalf("after Start stage must be Playing, got %v", s.Stage())
	}
	if err := s.Start("h", 42); err != ErrNotLobby {
		t.Fatalf("double start: want ErrNotLobby, got %v", err)
	}
}

func TestLeaveMigratesHost(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.Join("p3", "Cara"); err != nil {
		t.Fatal(err)
	}
	s.Leave("h") // the host leaves the lobby

	// The host role migrates to the new order[0] (p2); p3 is still a non-host.
	if err := s.Start("p3", 1); err != ErrNotHost {
		t.Fatalf("non-host p3 start: want ErrNotHost, got %v", err)
	}
	if err := s.Start("p2", 42); err != nil {
		t.Fatalf("migrated host p2 must be able to Start, got %v", err)
	}
	if s.Stage() != Playing {
		t.Fatalf("after Start stage must be Playing, got %v", s.Stage())
	}
}

func TestLeaveHostEmptiesRoom(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	s.Leave("h") // sole player leaves → host migration is a no-op
	if len(s.order) != 0 {
		t.Fatalf("room must be empty after the sole player leaves, got order %v", s.order)
	}
}

func TestSetConfigHostLobbyOnly(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	c52 := Config{Rules: engine.RuleSet{DeckSize: engine.Deck52}, Mode: engine.Guard}
	if err := s.SetConfig("p2", c52); err != ErrNotHost {
		t.Fatalf("non-host SetConfig: want ErrNotHost, got %v", err)
	}
	if err := s.SetConfig("h", c52); err != nil {
		t.Fatalf("host SetConfig rejected: %v", err)
	}
	_ = s.Join("p2", "Bob")
	_ = s.Start("h", 7)
	if err := s.SetConfig("h", cfg36()); err != ErrNotLobby {
		t.Fatalf("SetConfig after start: want ErrNotLobby, got %v", err)
	}
}
