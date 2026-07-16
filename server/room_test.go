package server

import (
	"testing"
	"time"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// cfg36 is the shared 36-card Middle config for server tests.
func cfg36() game.Config {
	return game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
}

func newTestRoom(t *testing.T) (*Room, *MemStore, *fakeClock) {
	t.Helper()
	store := NewMemStore()
	clock := newFakeClock(time.Unix(0, 0))
	r, tok := NewRoom("ROOM01", cfg36(), "Host", store, clock)
	if _, ok := r.playerFor(tok); !ok {
		t.Fatal("host token must map to a player")
	}
	return r, store, clock
}

func TestNewRoomSeatsHost(t *testing.T) {
	store := NewMemStore()
	r, tok := NewRoom("ROOM01", cfg36(), "Host", store, newFakeClock(time.Unix(0, 0)))
	pid, ok := r.playerFor(tok)
	if !ok {
		t.Fatal("host token not registered")
	}
	if st := r.session.Snapshot(); st.Host != pid {
		t.Fatalf("host token maps to %q, but session host is %q", pid, st.Host)
	}
}

func TestRoomJoinAddsSeatAndToken(t *testing.T) {
	r, _, _ := newTestRoom(t)
	tok2, err := r.Join("Bob")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	pid2, ok := r.playerFor(tok2)
	if !ok {
		t.Fatal("joined token not registered")
	}
	st := r.session.Snapshot()
	if len(st.Order) != 2 || st.Order[1] != pid2 {
		t.Fatalf("join did not seat Bob at index 1: order=%v", st.Order)
	}
	if r.seatOf(pid2) != 1 {
		t.Fatalf("seatOf(Bob) = %d, want 1", r.seatOf(pid2))
	}
}

func TestRoomPersistWritesLoadableSnapshot(t *testing.T) {
	_, store, _ := newTestRoom(t)
	snap, ok, err := store.Load("ROOM01")
	if err != nil || !ok {
		t.Fatalf("Load after NewRoom: ok=%v err=%v", ok, err)
	}
	if snap.Code != "ROOM01" || len(snap.Tokens) != 1 {
		t.Fatalf("persisted snapshot wrong: %+v", snap)
	}
	if snap.Session.Host == "" {
		t.Fatal("persisted session must carry a host")
	}
}
