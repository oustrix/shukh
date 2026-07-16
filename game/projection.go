package game

import "github.com/oustrix/shukh/engine"

// SeatMeta is the public identity of a seat: its index and display name.
type SeatMeta struct {
	Seat engine.SeatID
	Name string
}

// Update is what a subscriber receives after each change (mirrors the web
// GameSnapshot + event stream, S-6). View is nil in the Lobby; once Playing it is
// the per-seat projection (D-9). Legal is the seat's legal actions; Events are the
// engine events produced by the change that triggered this Update (empty for a
// plain Snapshot).
type Update struct {
	Stage  Lifecycle
	Roster []SeatMeta
	View   *engine.SeatView
	Legal  []engine.Action
	Events []engine.Event
}

// roster builds the public seat list in clockwise (join) order. Caller holds s.mu.
func (s *Session) roster() []SeatMeta {
	out := make([]SeatMeta, len(s.order))
	for i, id := range s.order {
		out[i] = SeatMeta{Seat: engine.SeatID(i), Name: s.names[id]}
	}
	return out
}

// project builds an Update for id from a pre-built roster and the given events.
// Caller holds s.mu and has verified id is seated. roster is shared read-only
// across every Update of one change, so the caller builds it once (see fanout).
func (s *Session) project(id PlayerID, roster []SeatMeta, events []engine.Event) Update {
	seat, _ := s.seatOf(id)
	up := Update{
		Stage:  s.stage,
		Roster: roster,
		Events: events,
	}
	if s.stage != Lobby {
		v := engine.View(s.state, seat)
		up.View = &v
		up.Legal = engine.LegalActions(s.state, seat)
	}
	return up
}

// Snapshot returns the current projection for id (no events). Errors if id is not
// seated.
func (s *Session) Snapshot(id PlayerID) (Update, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return Update{}, ErrUnknownPlayer
	}
	return s.project(id, s.roster(), nil), nil
}
