package engine

import "slices"

// IllegalAction is the typed error Apply returns for a move that is not legal in
// the current state/mode (spec §4). Code is a short machine tag; Rule points at
// the governing rule/section.
type IllegalAction struct {
	Code string
	Rule string
}

func (e *IllegalAction) Error() string {
	return "engine: illegal action [" + e.Code + "] (" + e.Rule + ")"
}

// Apply is the pure con-lifecycle reducer (§5). It validates a against
// LegalActions(s, s.Turn), then applies it to a clone and runs the resolve
// pipeline (§14.3). The input is never mutated; on rejection it returns s
// unchanged plus an *IllegalAction.
func Apply(s State, a Action) (State, []Event, error) {
	if s.Phase == Finished {
		return s, nil, &IllegalAction{Code: "game_over", Rule: "R-10.1"}
	}
	turn := s.Turn
	if !slices.Contains(LegalActions(s, turn), a) {
		return s, nil, &IllegalAction{Code: "illegal_action", Rule: "§5"}
	}
	ns := s.clone()
	var events []Event

	switch act := a.(type) {
	case PlayCard:
		wasEmpty := len(ns.Table) == 0
		ns.Hands[turn] = removeCard(ns.Hands[turn], act.Card)
		ns.Table = append(ns.Table, TableCard{Card: act.Card, By: turn})
		events = append(events, CardPlayed{Seat: turn, Card: act.Card})
		if wasEmpty {
			// Заход: never closes (threshold ≥ 2); pass the turn.
			ns.settleTurn(ns.nextLive(turn), &events)
		} else {
			// Бой: close/no-close handled in later tasks.
			ns.settleTurn(ns.nextLive(turn), &events)
		}
	}
	return ns, events, nil
}

// clone returns a deep-enough copy for copy-on-write: all mutated maps and slices
// are fresh. Seats is immutable for the game and shared.
func (s State) clone() State {
	ns := s
	ns.Hands = make(map[SeatID][]Card, len(s.Hands))
	for k, v := range s.Hands {
		ns.Hands[k] = append([]Card(nil), v...)
	}
	ns.Table = append([]TableCard(nil), s.Table...)
	ns.Discard = append([]Card(nil), s.Discard...)
	ns.Shukh = make(map[SeatID][]Card, len(s.Shukh))
	for k, v := range s.Shukh {
		ns.Shukh[k] = append([]Card(nil), v...)
	}
	ns.Live = make(map[SeatID]bool, len(s.Live))
	for k, v := range s.Live {
		ns.Live[k] = v
	}
	ns.Finish = append([]SeatID(nil), s.Finish...)
	return ns
}

// liveCount is the number of players still in the game (R-5.5.1).
func (s State) liveCount() int {
	n := 0
	for _, v := range s.Live {
		if v {
			n++
		}
	}
	return n
}

// nextLive returns the first live seat strictly clockwise after `after`. Seats
// are the identity 0..n-1 (NewGame), so index arithmetic tracks SeatID. If no
// other seat is live it returns `after`.
func (s State) nextLive(after SeatID) SeatID {
	n := len(s.Seats)
	for k := 1; k <= n; k++ {
		seat := s.Seats[(int(after)+k)%n]
		if s.Live[seat] {
			return seat
		}
	}
	return after
}

// settleTurn points Turn at candidate. (Task 9 upgrades it to skip the Guard
// lone-Дама♥ opener, §14.4.)
func (s *State) settleTurn(candidate SeatID, events *[]Event) {
	s.Turn = candidate
}

// removeCard returns hand without its first occurrence of c. The caller has
// already validated (via LegalActions) that c is present.
func removeCard(hand []Card, c Card) []Card {
	for i, x := range hand {
		if x == c {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}
