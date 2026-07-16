// Package game is Layer 1 (D-6): transport-agnostic orchestration of a single live
// «Шух» game over the pure engine. A Session holds authoritative state, maps
// players to seats, runs the Lobby→Playing→Finished lifecycle, accepts actions, and
// projects a per-seat view + event stream. It knows nothing about sockets, room
// codes, or reconnect tokens — those live in Layer 2.
package game

import (
	"errors"
	"sync"

	"github.com/oustrix/shukh/engine"
)

// PlayerID is a stable, opaque identity handed in by Layer 2. Session only stores
// and compares it; it never interprets it.
type PlayerID string

// Lifecycle is the coarse stage of a session.
type Lifecycle int

const (
	Lobby Lifecycle = iota
	Playing
	Finished
)

// Config is the per-match setup chosen in the lobby (D-8/D-10): deck rules and
// enforcement mode. The player roster is assembled by Session from join order.
type Config struct {
	Rules engine.RuleSet
	Mode  engine.EnforcementMode
}

// Lobby / lifecycle errors.
var (
	ErrNotLobby      = errors.New("game: action allowed only in the lobby")
	ErrNotHost       = errors.New("game: only the host may do this")
	ErrFull          = errors.New("game: table is full (max 8, D-3)")
	ErrDuplicate     = errors.New("game: player already joined")
	ErrUnknownPlayer = errors.New("game: unknown player")
	ErrNotPlaying    = errors.New("game: game is not in progress")
	ErrNotYours      = errors.New("game: action does not belong to this player")
	ErrTooFewPlayers = errors.New("game: need at least 2 players to start (D-3)")
)

const maxPlayers = 8

// Session is the synchronous, mutex-guarded orchestrator of one game (S-3).
type Session struct {
	mu    sync.Mutex
	cfg   Config
	host  PlayerID
	stage Lifecycle

	order []PlayerID          // join order → seat index (R-2.13 clockwise seating)
	names map[PlayerID]string // display names

	state engine.State // authoritative; valid once stage >= Playing

	subs map[PlayerID]*subscriber // populated in Task 9
}

// NewSession creates a lobby seated by the host (seat 0, the eventual shuffler R-4.7).
func NewSession(cfg Config, host PlayerID, hostName string) *Session {
	return &Session{
		cfg:   cfg,
		host:  host,
		stage: Lobby,
		order: []PlayerID{host},
		names: map[PlayerID]string{host: hostName},
		subs:  map[PlayerID]*subscriber{},
	}
}

// Stage returns the current lifecycle stage.
func (s *Session) Stage() Lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stage
}

// Join seats a new player at the end of the clockwise order. Lobby only.
func (s *Session) Join(id PlayerID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if _, ok := s.names[id]; ok {
		return ErrDuplicate
	}
	if len(s.order) >= maxPlayers {
		return ErrFull
	}
	s.order = append(s.order, id)
	s.names[id] = name
	return nil
}

// Leave removes a player from the lobby (mid-game leave is a Layer-2 disconnect
// concern, out of scope here). No-op if the game has started or the player is absent.
func (s *Session) Leave(id PlayerID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return
	}
	if _, ok := s.names[id]; !ok {
		return
	}
	delete(s.names, id)
	for i, p := range s.order {
		if p == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// seatOf maps a player to its seat index (its position in join order). The caller
// must hold s.mu, except tests that never race.
func (s *Session) seatOf(id PlayerID) (engine.SeatID, bool) {
	for i, p := range s.order {
		if p == id {
			return engine.SeatID(i), true
		}
	}
	return 0, false
}

// temporary stub — replaced in Task 9 (subscribe.go)
type subscriber struct{}
