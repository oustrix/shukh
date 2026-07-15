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
		} else if IsQueenHearts(act.Card) || len(ns.Table) == ns.liveCount() {
			ns.closeCon(turn, &events) // Дама♥ closes immediately (R-3.7.1)
		} else {
			ns.settleTurn(ns.nextLive(turn), &events)
		}
	case TakeBottomAndPass:
		taken := ns.Table[0]
		ns.Table = ns.Table[1:]
		ns.Hands[turn] = append(ns.Hands[turn], taken.Card)
		events = append(events, CardsTaken{Seat: turn, Cards: []Card{taken.Card}})
		// Taking can only shrink the con, so it never closes; but removing the
		// bottom card may free its (handless) owner to exit (R-9.1).
		ns.resolveExits([]SeatID{taken.By}, &events)
		if ns.Phase != Finished {
			ns.settleTurn(ns.nextLive(turn), &events)
		}
	default:
		// PodkladkaWest is legal per LegalActions in many states, but its Apply
		// case lands in Task 8. Until then, reject rather than silently no-op (a
		// legal move must never look applied when nothing changed).
		return s, nil, &IllegalAction{Code: "not_implemented", Rule: "§5"}
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

// closeCon sweeps the whole con to the discard (R-5.6, auto in Guard), applies
// R-9.1 exits clockwise from the closer, and hands the next заход to the closer —
// or, if the closer just exited, to the next live seat (R-5.7.2). The closing
// card is already on the table.
func (s *State) closeCon(closer SeatID, events *[]Event) {
	swept := make([]Card, len(s.Table))
	for i, tc := range s.Table {
		swept[i] = tc.Card
	}
	s.Discard = append(s.Discard, swept...)
	s.Table = nil
	*events = append(*events, ConClosed{By: closer}, ConSwept{Cards: swept})

	s.resolveExits(s.seatsFrom(closer), events)
	if s.Phase == Finished {
		return
	}
	cand := closer
	if !s.Live[closer] {
		cand = s.nextLive(closer)
	}
	s.settleTurn(cand, events)
}

// resolveExits applies R-9.1 exits for the seats in `order` (clockwise from the
// seat whose action emptied/changed the con), then checks termination
// (R-10.1/R-10.1.1). A live seat exits when its hand is empty and it has no card
// in the open con. When one or zero players remain, the game ends and the loser
// (if any) is appended last to Finish.
func (s *State) resolveExits(order []SeatID, events *[]Event) {
	for _, seat := range order {
		if s.Live[seat] && len(s.Hands[seat]) == 0 && !s.handInCon(seat) {
			s.Live[seat] = false
			s.Finish = append(s.Finish, seat)
			*events = append(*events, PlayerFinished{Seat: seat, Place: len(s.Finish)})
		}
	}
	if s.liveCount() <= 1 {
		s.Phase = Finished
		for _, seat := range s.Seats {
			if s.Live[seat] { // the loser still holds cards (R-10.1)
				s.Finish = append(s.Finish, seat)
			}
		}
		*events = append(*events, GameFinished{Finish: append([]SeatID(nil), s.Finish...)})
	}
}

// seatsFrom returns all seats in clockwise order starting at pivot (inclusive).
func (s State) seatsFrom(pivot SeatID) []SeatID {
	n := len(s.Seats)
	out := make([]SeatID, 0, n)
	for k := 0; k < n; k++ {
		out = append(out, s.Seats[(int(pivot)+k)%n])
	}
	return out
}

// handInCon reports whether any card in the open con was played by seat (R-9.1).
func (s State) handInCon(seat SeatID) bool {
	for _, tc := range s.Table {
		if tc.By == seat {
			return true
		}
	}
	return false
}
