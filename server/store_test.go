package server

import (
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

func sampleSnapshot(code string) RoomSnapshot {
	cfg := game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
	st := game.NewSession(cfg, "host", "Host").Snapshot() // durable SessionState (Фаза A)
	return RoomSnapshot{
		Code:    code,
		Tokens:  map[Token]game.PlayerID{"tok-h": "host"},
		Session: st,
	}
}

func TestMemStoreRoundTrip(t *testing.T) {
	m := NewMemStore()
	snap := sampleSnapshot("ROOM01")
	if err := m.Save(snap); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := m.Load("ROOM01")
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if got.Code != "ROOM01" || got.Tokens["tok-h"] != "host" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestMemStoreSaveDeepCopiesTokens(t *testing.T) {
	m := NewMemStore()
	snap := sampleSnapshot("ROOM01")
	_ = m.Save(snap)
	// mutating the live snapshot's Tokens must not corrupt the store.
	snap.Tokens["tok-h"] = "hijacked"
	got, _, _ := m.Load("ROOM01")
	if got.Tokens["tok-h"] != "host" {
		t.Fatalf("stored token was aliased and mutated: %q", got.Tokens["tok-h"])
	}
}

func TestMemStoreLoadMiss(t *testing.T) {
	m := NewMemStore()
	if _, ok, err := m.Load("nope"); ok || err != nil {
		t.Fatalf("miss must be ok=false,nil; got ok=%v err=%v", ok, err)
	}
}

func TestMemStoreDeleteAndList(t *testing.T) {
	m := NewMemStore()
	_ = m.Save(sampleSnapshot("A"))
	_ = m.Save(sampleSnapshot("B"))
	if err := m.Delete("A"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := m.Load("A"); ok {
		t.Fatal("deleted room still present")
	}
	list, _ := m.List()
	if len(list) != 1 || list[0] != "B" {
		t.Fatalf("List after delete = %v, want [B]", list)
	}
}
