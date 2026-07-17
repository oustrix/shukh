package game

import (
	"maps"

	"github.com/oustrix/shukh/engine"
)

// SessionState is the durable snapshot of a Session (Layer-2 RoomStore seam, L2-5):
// pure data with no live machinery (no subs, no mutex). It round-trips through
// Snapshot/Restore so a room can be persisted and rebuilt.
type SessionState struct {
	Config Config
	Host   PlayerID
	Stage  Lifecycle
	Order  []PlayerID
	Names  map[PlayerID]string
	Game   engine.State
}

// Snapshot returns a deep copy of the session's durable state (L2-5). Every map and
// slice is cloned — including engine.State via State.Clone — so the returned value
// shares no aliased storage with the live session and can be persisted or held while
// play continues.
func (s *Session) Snapshot() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionState{
		Config: s.cfg,
		Host:   s.host,
		Stage:  s.stage,
		Order:  append([]PlayerID(nil), s.order...),
		Names:  maps.Clone(s.names),
		Game:   s.state.Clone(),
	}
}

// Restore rebuilds a live Session from a durable snapshot (L2-5). Durable data is
// deep-copied in; the ephemeral machinery (subscriptions) is recreated empty, so the
// restored session starts with no subscribers and is otherwise identical.
func Restore(st SessionState) *Session {
	return &Session{
		cfg:   st.Config,
		host:  st.Host,
		stage: st.Stage,
		order: append([]PlayerID(nil), st.Order...),
		names: maps.Clone(st.Names),
		state: st.Game.Clone(),
		subs:  map[PlayerID]chan Update{},
	}
}
