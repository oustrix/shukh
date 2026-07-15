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
	case PodkladkaWest:
		west := Card{Suit: Hearts, Rank: ns.Rules.LowestRank()} // 6(2)♥
		ns.Hands[turn] = removeCard(ns.Hands[turn], west)
		eater := ns.nextLive(turn)
		eaten := append([]Card{west}, cardsOf(ns.Table)...) // 6(2)♥ tucked under the con (R-3.6.2)
		ns.Hands[eater] = append(ns.Hands[eater], eaten...)
		ns.Table = nil
		events = append(events,
			PodkladkaPlayed{Seat: turn, Eater: eater},
			CardsTaken{Seat: eater, Cards: eaten},
		)
		// The table is emptied (eaten): any handless player whose cards were in it
		// exits; the eater cannot (it just gained the con). Eater opens (R-5.7.1).
		ns.resolveExits(ns.seatsFrom(turn), &events)
		if ns.Phase != Finished {
			ns.settleTurn(eater, &events)
		}
	default:
		// All turn-actions produced by LegalActions are wired above; this is a
		// safety net for a genuinely-unknown Action (e.g. a bug in LegalActions or
		// a future action type not yet handled here) — reject rather than
		// silently no-op.
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
	ns.OwesOneCard = make(map[SeatID]bool, len(s.OwesOneCard))
	for k, v := range s.OwesOneCard {
		ns.OwesOneCard[k] = v
	}
	ns.ShukhTakeable = make(map[SeatID]bool, len(s.ShukhTakeable))
	for k, v := range s.ShukhTakeable {
		ns.ShukhTakeable[k] = v
	}
	if s.Pending != nil {
		cp := *s.Pending
		cp.Owed = append([]SeatID(nil), s.Pending.Owed...)
		ns.Pending = &cp
	}
	if s.Unsettled != nil {
		cp := *s.Unsettled // Prev is a snapshot we never mutate; sharing its maps is safe
		ns.Unsettled = &cp
	}
	return ns
}

// gatesClosed reports whether no catch-window or payment gate is open — i.e. the
// game is in a normal position where turn-actions and fresh social actions are
// available (§15.8: at most one of Unsettled/Pending is active at a time).
func (s State) gatesClosed() bool { return s.Unsettled == nil && s.Pending == nil }

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

// forcedQueenSkip reports the Guard-only lone-Дама♥ opener case (§14.4): the con
// is empty and seat's only card is Дама♥, so its sole "move" would be the
// forbidden Дама♥ заход (R-3.7.2). Such a turn is skipped in Guard.
func (s State) forcedQueenSkip(seat SeatID) bool {
	h := s.Hands[seat]
	return len(s.Table) == 0 && len(h) == 1 && IsQueenHearts(h[0])
}

// settleTurn resolves a candidate opener into the actual next Turn: if the
// candidate exited during this resolution it advances to the next live seat
// (R-5.7.2), then skips past a seat stuck in the Guard lone-Дама♥ opener case
// (§14.4, emitting TurnSkipped). It thus always lands on a live, playable seat.
// At most one seat can qualify for the Дама♥ skip (a single Дама♥ exists).
func (s *State) settleTurn(candidate SeatID, events *[]Event) {
	seat := candidate
	if !s.Live[seat] {
		seat = s.nextLive(seat)
	}
	// The lone-Дама♥ opener skip is a Guard-only device (§14.4). In Middle the
	// Дама♥ заход is allowed and caught as Ш-2, so Turn may legitimately rest on
	// such a seat; do not skip.
	for s.Mode == Guard && s.forcedQueenSkip(seat) {
		*events = append(*events, TurnSkipped{Seat: seat})
		seat = s.nextLive(seat)
	}
	s.Turn = seat
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
	swept := cardsOf(s.Table)
	s.Discard = append(s.Discard, swept...)
	s.Table = nil
	*events = append(*events, ConClosed{By: closer}, ConSwept{Cards: swept})

	s.resolveExits(s.seatsFrom(closer), events)
	if s.Phase == Finished {
		return
	}
	// The closer opens next (R-5.7); if it just exited, settleTurn falls back to
	// the next live seat (R-5.7.2).
	s.settleTurn(closer, events)
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
	return slices.ContainsFunc(s.Table, func(tc TableCard) bool { return tc.By == seat })
}

// cardsOf projects a con (or any TableCard slice) to its bare cards, bottom→top.
func cardsOf(tcs []TableCard) []Card {
	out := make([]Card, len(tcs))
	for i, tc := range tcs {
		out[i] = tc.Card
	}
	return out
}
