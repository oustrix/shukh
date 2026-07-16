package server

import (
	"maps"
	"sync"

	"github.com/oustrix/shukh/game"
)

// RoomSnapshot is a room's durable state — plain, serializable data (§4). The
// ephemeral machinery (sockets, subscription channels, timers) is rebuilt on
// reconnect/restart and is not part of the snapshot.
type RoomSnapshot struct {
	Code    string
	Tokens  map[Token]game.PlayerID
	Session game.SessionState // {Config, Host, Stage, Order, Names, Game engine.State}
}

// RoomStore is the storage seam (L2-5): the Hub depends only on this. MemStore is
// the MVP default; a RedisStore / SQLStore would implement the same interface with
// nothing else changed.
type RoomStore interface {
	Save(RoomSnapshot) error
	Load(code string) (RoomSnapshot, bool, error)
	Delete(code string) error
	List() ([]string, error)
}

// MemStore is an in-memory RoomStore under a mutex.
type MemStore struct {
	mu    sync.Mutex
	rooms map[string]RoomSnapshot
}

var _ RoomStore = (*MemStore)(nil)

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{rooms: map[string]RoomSnapshot{}} }

// Save write-throughs a deep copy: Tokens, and Session.Order/Names/Game are all
// cloned so later mutation of the live snapshot (or of a value returned by Load)
// cannot alias the store's storage.
func (m *MemStore) Save(snap RoomSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rooms[snap.Code] = copySnapshot(snap)
	return nil
}

func (m *MemStore) Load(code string) (RoomSnapshot, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap, ok := m.rooms[code]
	if !ok {
		return RoomSnapshot{}, false, nil
	}
	return copySnapshot(snap), true, nil
}

func (m *MemStore) Delete(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, code)
	return nil
}

func (m *MemStore) List() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.rooms))
	for c := range m.rooms {
		out = append(out, c)
	}
	return out, nil
}

func copySnapshot(snap RoomSnapshot) RoomSnapshot {
	cp := snap
	cp.Tokens = make(map[Token]game.PlayerID, len(snap.Tokens))
	for k, v := range snap.Tokens {
		cp.Tokens[k] = v
	}
	cp.Session.Order = append([]game.PlayerID(nil), snap.Session.Order...)
	cp.Session.Names = maps.Clone(snap.Session.Names)
	cp.Session.Game = snap.Session.Game.Clone()
	return cp
}
