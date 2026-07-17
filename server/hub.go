package server

import (
	"sync"
	"time"

	"github.com/oustrix/shukh/game"
)

const (
	idleTTL       = 2 * time.Minute // Finished room lingers this long after going empty
	sweepInterval = 1 * time.Minute // background GC cadence
)

// Hub is the registry of rooms by code and their garbage collector (§3.1). Its only
// storage dependency is RoomStore; all timing goes through Clock.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
	store RoomStore
	clock Clock
}

// NewHub builds an empty hub over the given store and clock.
func NewHub(store RoomStore, clock Clock) *Hub {
	return &Hub{rooms: map[string]*Room{}, store: store, clock: clock}
}

// CreateRoom mints a collision-free code, creates the room (seating the host), and
// registers it. Returns the code, host token, and room.
func (h *Hub) CreateRoom(cfg game.Config, hostName string) (string, Token, *Room) {
	h.mu.Lock()
	defer h.mu.Unlock()
	code := h.freeCodeLocked()
	r, tok := NewRoom(code, cfg, hostName, h.store, h.clock)
	h.rooms[code] = r
	return code, tok, r
}

// freeCodeLocked generates codes until one is not already in the registry. Caller
// holds h.mu.
func (h *Hub) freeCodeLocked() string {
	for {
		c := newCode(cryptoBytes())
		if _, ok := h.rooms[c]; !ok {
			return c
		}
	}
}

// Room returns the room for a code, if present.
func (h *Hub) Room(code string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	return r, ok
}

// sweep removes collectible rooms (no live sockets past grace, or Finished past
// idle-TTL) from the registry and the store. Lock order: h.mu then each r.mu.
func (h *Hub) sweep() {
	now := h.clock.Now()
	h.mu.Lock()
	var dead []string
	for code, r := range h.rooms {
		if r.collectible(now) {
			dead = append(dead, code)
		}
	}
	for _, code := range dead {
		delete(h.rooms, code)
	}
	h.mu.Unlock()
	for _, code := range dead {
		_ = h.store.Delete(code)
	}
}

// StartSweeper schedules recurring GC via the clock. Real time in production; tests
// call sweep() directly with a fake clock instead.
func (h *Hub) StartSweeper() {
	h.clock.AfterFunc(sweepInterval, func() {
		h.sweep()
		h.StartSweeper()
	})
}
