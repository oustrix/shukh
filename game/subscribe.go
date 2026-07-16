package game

import "github.com/oustrix/shukh/engine"

// subCapacity bounds a subscriber's buffer. A consumer slower than this many
// pending Updates is marked stale (its delta is dropped) and must re-Snapshot; the
// game never blocks on a slow client.
const subCapacity = 16

type subscriber struct {
	ch    chan Update
	stale bool
}

// Subscribe registers id for push Updates and immediately delivers a snapshot.
// The returned func() unsubscribes (closing the channel). Errors if id is not seated.
func (s *Session) Subscribe(id PlayerID) (<-chan Update, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return nil, nil, ErrUnknownPlayer
	}
	sub := &subscriber{ch: make(chan Update, subCapacity)}
	s.subs[id] = sub
	sub.ch <- s.project(id, nil) // initial snapshot fits (fresh buffer)
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if cur, ok := s.subs[id]; ok && cur == sub {
			delete(s.subs, id)
			close(sub.ch)
		}
	}
	return sub.ch, cancel, nil
}

// fanout pushes a per-seat Update to every subscriber. perSeat maps a player to the
// events relevant to it (typically the same event list for all). A non-blocking
// send drops the delta and marks the subscriber stale when its buffer is full; the
// client recovers via Snapshot. Caller holds s.mu.
func (s *Session) fanout(perSeat map[PlayerID][]engine.Event) {
	for id, sub := range s.subs {
		up := s.project(id, perSeat[id])
		select {
		case sub.ch <- up:
			sub.stale = false
		default:
			sub.stale = true // buffer full: client must re-Snapshot
		}
	}
}
