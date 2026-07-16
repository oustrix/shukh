package game

import "github.com/oustrix/shukh/engine"

// subCapacity bounds a subscriber's buffer. When a consumer falls this many
// pending Updates behind, fanout drops the delta (the client recovers via
// Snapshot) rather than block the game.
const subCapacity = 16

// Subscribe registers id for push Updates and immediately delivers a snapshot.
// The returned func() unsubscribes (closing the channel). Errors if id is not seated.
func (s *Session) Subscribe(id PlayerID) (<-chan Update, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return nil, nil, ErrUnknownPlayer
	}
	// Close-and-replace (L2 reconnect): if a subscriber for id already exists (a
	// still-live socket at reconnect), close and drop it first so its reader goroutine
	// receives a closed channel and exits — otherwise it leaks. The old cancel becomes
	// a no-op: its identity check (cur == oldch) no longer matches s.subs[id].
	if prev, ok := s.subs[id]; ok {
		delete(s.subs, id)
		close(prev)
	}
	ch := make(chan Update, subCapacity)
	s.subs[id] = ch
	ch <- s.project(id, s.roster(), nil) // initial snapshot fits (fresh buffer)
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if cur, ok := s.subs[id]; ok && cur == ch {
			delete(s.subs, id)
			close(ch)
		}
	}
	return ch, cancel, nil
}

// fanout pushes an Update to every subscriber for a single change. events is the
// shared event list the change produced (all seats receive it); the roster is
// identical for every subscriber, so it is built once here and only the per-seat
// View/Legal projection is computed in the loop. Sends are non-blocking: a full
// buffer drops the delta (the client recovers via Snapshot), so a slow consumer
// never blocks the game. Caller holds s.mu.
func (s *Session) fanout(events []engine.Event) {
	roster := s.roster()
	for id, ch := range s.subs {
		select {
		case ch <- s.project(id, roster, events):
		default: // buffer full: delta dropped; client re-Snapshots
		}
	}
}
