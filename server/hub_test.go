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

// TestSweepStopsTimersNoResurrection guards against a store-resurrection leak:
// Hub.sweep used to remove a room from the registry and delete it from the store
// without stopping its pending timers. A grace timer armed before the sweep (whose
// deadline lands after the sweep's collectible threshold, both keyed off graceTTL)
// would later fire graceExpired -> Leave -> persist, re-Saving a room the Hub no
// longer tracks — a permanent, unreachable store entry. sweep must close() the
// room (stopping timers, gating persist) before deleting it, so the timer firing
// after the sweep is a no-op.
func TestSweepStopsTimersNoResurrection(t *testing.T) {
	clock := newFakeClock(time.Unix(0, 0))
	h := NewHub(NewMemStore(), clock)
	code, hostTok, r := h.CreateRoom(cfg36(), "Host")

	pid, ok := r.playerFor(hostTok)
	if !ok {
		t.Fatal("host token must resolve to a player")
	}

	// Arm the grace timer 1 minute after room creation, so its deadline
	// (1min + graceTTL) lands strictly after the moment the room first becomes
	// collectible (emptyAt(=0) + graceTTL). This keeps the timer pending at sweep
	// time instead of firing as part of the same clock advance.
	clock.Advance(1 * time.Minute)
	r.onDisconnect(pid)

	// Reach exactly the collectible threshold (now - emptyAt == graceTTL) without
	// yet reaching the grace timer's later deadline.
	clock.Advance(graceTTL - time.Minute)

	h.sweep()

	if _, ok := h.Room(code); ok {
		t.Fatal("swept room must be gone from the registry")
	}

	// Fire the (should-be-stopped) grace timer. Before the fix this calls
	// graceExpired -> Leave -> persist, resurrecting the room in the store.
	clock.Advance(graceTTL)

	if _, ok, _ := h.store.Load(code); ok {
		t.Fatal("a timer firing after sweep must not resurrect the room in the store")
	}
}
