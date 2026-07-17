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
	"github.com/oustrix/shukh/shuffle"
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

	subs map[PlayerID]chan Update // push channels, one per active subscriber
}

// NewSession creates a lobby seated by the host (seat 0, the eventual shuffler R-4.7).
func NewSession(cfg Config, host PlayerID, hostName string) *Session {
	return &Session{
		cfg:   cfg,
		host:  host,
		stage: Lobby,
		order: []PlayerID{host},
		names: map[PlayerID]string{host: hostName},
		subs:  map[PlayerID]chan Update{},
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
// If the leaving player is the host and at least one player remains, the host role
// migrates to the new order[0] (L2-3); if the room becomes empty the host is left
// dangling and Layer 2 GCs the room — nothing to migrate to.
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
	if id == s.host && len(s.order) > 0 {
		s.host = s.order[0] // migrate the host role to the next seat (L2-3)
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

// SeatOf reports the seat index for id and whether id is seated. Thread-safe.
func (s *Session) SeatOf(id PlayerID) (engine.SeatID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seatOf(id)
}

// SetConfig changes the match config before the game starts. Host + Lobby only.
func (s *Session) SetConfig(host PlayerID, cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if host != s.host {
		return ErrNotHost
	}
	s.cfg = cfg
	return nil
}

// Start deals a fresh game: it builds the engine.Config from the roster (join order
// = clockwise seating, R-2.13), shuffles a canonical deck by seed at the D-11
// boundary, and runs engine.NewGame. Host + Lobby + ≥2 players only. On success the
// stage becomes Playing (or Finished if the game somehow ends immediately).
func (s *Session) Start(host PlayerID, seed int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if host != s.host {
		return ErrNotHost
	}
	if len(s.order) < 2 {
		return ErrTooFewPlayers
	}
	players := make([]engine.Player, len(s.order))
	for i, id := range s.order {
		players[i] = engine.Player{Name: s.names[id]}
	}
	ecfg := engine.Config{Rules: s.cfg.Rules, Mode: s.cfg.Mode, Players: players}
	deck := shuffle.Deck(engine.NewDeck(s.cfg.Rules), seed)
	st, _, err := engine.NewGame(ecfg, deck)
	if err != nil {
		return err
	}
	s.state = st
	s.stage = Playing
	if st.Phase == engine.Finished {
		s.stage = Finished
	}
	return nil
}
