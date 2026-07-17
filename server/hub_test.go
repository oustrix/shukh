package server

import (
	"testing"
	"time"
)

func TestCreateRoomRetrievableUniqueCode(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	code, tok, r := h.CreateRoom(cfg36(), "Host")
	if code == "" || len(code) != 6 {
		t.Fatalf("bad code %q", code)
	}
	got, ok := h.Room(code)
	if !ok || got != r {
		t.Fatal("created room must be retrievable by code")
	}
	if _, ok := r.playerFor(tok); !ok {
		t.Fatal("host token must belong to the room")
	}
	code2, _, _ := h.CreateRoom(cfg36(), "Host2")
	if code2 == code {
		t.Fatal("codes must be unique across rooms")
	}
}

func TestSweepRemovesEmptyExpiredKeepsLive(t *testing.T) {
	clock := newFakeClock(time.Unix(0, 0))
	h := NewHub(NewMemStore(), clock)
	liveCode, _, live := h.CreateRoom(cfg36(), "Live")
	deadCode, _, _ := h.CreateRoom(cfg36(), "Dead")

	// keep the live room busy with a socket
	live.mu.Lock()
	live.noteConnOpened()
	live.mu.Unlock()

	clock.Advance(graceTTL + time.Minute)
	h.sweep()

	if _, ok := h.Room(liveCode); !ok {
		t.Fatal("a room with a live socket must survive sweep")
	}
	if _, ok := h.Room(deadCode); ok {
		t.Fatal("an empty room past grace must be swept")
	}
	if _, ok, _ := h.store.Load(deadCode); ok {
		t.Fatal("sweep must also delete the room from the store")
	}
}
