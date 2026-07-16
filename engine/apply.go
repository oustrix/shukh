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
	if !isLegal(s, a) {
		return s, nil, &IllegalAction{Code: "illegal_action", Rule: "§5"}
	}
	turn := s.Turn
	ns := s.clone()
	var events []Event

	// A non-claim action taken while a catch-window is open settles it (R-1.4.1):
	// the offending action «прижилось» and stays. ClaimShukh instead reverses it.
	if _, isClaim := a.(ClaimShukh); !isClaim && ns.Unsettled != nil {
		ns.Unsettled = nil
	}

	// Pre-action hand sizes (§15.6), captured from the input s before any mutation,
	// for reconcileOneCard's before/after transition check.
	before := handSizes(s.Hands)

	switch act := a.(type) {
	case PlayCard:
		wasEmpty := len(ns.Table) == 0
		ns.Hands[turn] = removeCard(ns.Hands[turn], act.Card)
		ns.Table = append(ns.Table, TableCard{Card: act.Card, By: turn})
		events = append(events, CardPlayed{Seat: turn, Card: act.Card})
		if wasEmpty {
			switch {
			case IsQueenHearts(act.Card):
				// Middle Дама♥ заход (R-3.7.2): allowed but нелегально → open the
				// Ш-2 catch-window over the pre-action snapshot (§15.3) and pass the
				// turn so the next player can settle it (or someone may claim).
				ns.Unsettled = &Unsettled{Prev: s, Seat: turn, Code: Sh2}
			case ns.Endgame.Active && ns.Rules.IsLowestHeart(act.Card):
				// Middle endgame 6(2)♥ заход = «использование» (R-9.4.3): allowed but
				// нелегально → open the Ш-12 catch-window (§15.3).
				ns.Unsettled = &Unsettled{Prev: s, Seat: turn, Code: Sh12}
			}
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
		if len(ns.Table) == 0 {
			ns.markShukhTakeable() // P-4: taking the last card empties the con
		}
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
		ns.markShukhTakeable() // P-4: the con that held any ШУХ has ended
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
	case ClaimShukh:
		// Reverse: restore the pre-action snapshot (§15.3), then assess the ШУХ. The
		// turn-skip (Ш-2/Ш-12), the 6(2)♥ discard obligation and the Endgame.Asked
		// latch (заход-caught Ш-12, R-9.4.3) are all derived from the code inside
		// assessShukh.
		ns = s.Unsettled.Prev.clone()
		events = append(events, ActionReverted{Seat: s.Unsettled.Seat})
		ns.assessShukh(s.Unsettled.Seat, s.Unsettled.Code, &events)
		before = handSizes(ns.Hands) // reconcile against the restored snapshot, not the post-offense sizes
	case GiveShukhCard:
		// A §8 payment: the current head payer moves one non-last card into the
		// offender's Shukh zone (I-3, not the offender's hand). When the last
		// obligated payer has paid, the deferred effect (skip/§9.4 obligation)
		// finally applies and the gate closes.
		p := ns.Pending
		giver := p.Owed[0]
		ns.Hands[giver] = removeCard(ns.Hands[giver], act.Card)
		ns.Shukh[p.Offender] = append(ns.Shukh[p.Offender], act.Card) // I-3: Shukh, not hand
		events = append(events, ShukhPaid{Offender: p.Offender, From: giver, Card: act.Card})
		p.Owed = p.Owed[1:]
		if len(p.Owed) == 0 {
			ns.applyShukhEffect(p.Offender, p.Skip, p.ThenDiscardWest, &events)
			ns.Pending = nil
		}
	case TakeShukhCards:
		if !ns.ShukhTakeable[act.Seat] {
			// Middle early take (LegalActions only offers this in Middle when not yet
			// takeable): allowed but нелегально → open the Ш-3 window over the
			// snapshot, then take the cards (the offense «прижилось» unless claimed).
			ns.Unsettled = &Unsettled{Prev: s, Seat: act.Seat, Code: Sh3}
		}
		taken := ns.Shukh[act.Seat]
		ns.Shukh[act.Seat] = nil
		ns.ShukhTakeable[act.Seat] = false
		ns.Hands[act.Seat] = append(ns.Hands[act.Seat], taken...)
		events = append(events, ShukhCardsTaken{Seat: act.Seat, Cards: taken})
	case DeclareOneCard:
		ns.OwesOneCard[act.Seat] = false
		events = append(events, OneCardDeclared{Seat: act.Seat})
	case AskCount:
		ns.assessShukh(act.Target, Sh11, &events) // R-6.2 (Sh11 has no skip/discard/latch)
		// R-6.2/R-6.3: the count question discharges the «одна карта» obligation —
		// otherwise the same undeclared single card could be re-penalized repeatedly.
		// Payment goes to the Shukh zone, not Target's hand, so Target's hand size
		// never changes here (now == before == 1) → reconcileOneCard's edge-trigger
		// does not fire below, and this false sticks.
		ns.OwesOneCard[act.Target] = false
	case DiscardWest:
		west := Card{Suit: Hearts, Rank: ns.Rules.LowestRank()} // 6(2)♥
		ns.Hands[turn] = removeCard(ns.Hands[turn], west)
		ns.Discard = append(ns.Discard, west)
		ns.Endgame.MustDiscard = false
		events = append(events, WestDiscarded{Seat: turn})
		// Discarding may have emptied the discarder's hand → it exits (R-9.1); the
		// con is already empty so nothing holds it back. Mirror TakeBottomAndPass /
		// PodkladkaWest: resolve the exit before handing off the turn.
		ns.resolveExits([]SeatID{turn}, &events)
		if ns.Phase != Finished {
			ns.settleTurn(ns.nextLive(turn), &events)
		}
	case AskAboutWest:
		// isLegal now guarantees Target holds 6(2)♥ (R-9.4.2), so this always assesses;
		// assessShukh latches Endgame.Asked for Ш-12 (R-9.4.2/R-9.4.3).
		ns.assessShukh(act.Target, Sh12, &events)
	case ClaimSubjective:
		ns.Adjudication = &Adjudication{
			Claimant: act.Claimant,
			Target:   act.Target,
			Code:     act.Code,
			Votes:    map[SeatID]bool{},
		}
		events = append(events, VoteOpened{Claimant: act.Claimant, Target: act.Target, Code: act.Code})
	case Vote:
		ns.Adjudication.Votes[act.Voter] = act.Support
		if len(ns.Adjudication.Votes) == len(ns.Seats) {
			ns.resolveAdjudication(&events)
		}
	default:
		// All turn-actions produced by LegalActions are wired above; this is a
		// safety net for a genuinely-unknown Action (e.g. a bug in LegalActions or
		// a future action type not yet handled here) — reject rather than
		// silently no-op.
		return s, nil, &IllegalAction{Code: "not_implemented", Rule: "§5"}
	}
	ns.reconcileOneCard(before)
	return ns, events, nil
}

// isLegal validates a against LegalActions for the seat responsible for it (P-1).
// Turn-actions check s.Turn; actor-carrying actions check their seat; the payer
// action checks the current payer; actor-agnostic social actions are validated by
// precondition (their legality does not depend on who raises them).
func isLegal(s State, a Action) bool {
	switch act := a.(type) {
	case ClaimShukh:
		return s.Unsettled != nil && s.Unsettled.Seat == act.Target && s.Unsettled.Code == act.Code
	case GiveShukhCard:
		if s.Pending == nil || len(s.Pending.Owed) == 0 {
			return false
		}
		return slices.Contains(LegalActions(s, s.Pending.Owed[0]), a)
	case TakeShukhCards:
		return slices.Contains(LegalActions(s, act.Seat), a)
	case DeclareOneCard:
		return slices.Contains(LegalActions(s, act.Seat), a)
	case AskCount:
		return s.gatesClosed() && s.OwesOneCard[act.Target]
	case AskAboutWest:
		// Actor-agnostic (P-1), like AskCount: legality is by Target's state alone,
		// not by who is asking (the asker seat is not modeled). Only the actual
		// 6(2)♥ holder is a legal target (R-9.4.2): otherwise the holder could
		// pre-emptively ask a non-holder, burn the single global Asked flag, and
		// dodge Ш-12 — no info leak in the 2-player endgame (you see your own hand).
		return s.gatesClosed() && s.Endgame.Active && !s.Endgame.Asked && s.Live[act.Target] &&
			slices.ContainsFunc(s.Hands[act.Target], s.Rules.IsLowestHeart)
	case ClaimSubjective:
		return s.gatesClosed() && act.Code.isSubjective() &&
			s.Live[act.Claimant] && s.Live[act.Target] && act.Claimant != act.Target
	case Vote:
		return s.Adjudication != nil && s.voterEligible(act.Voter) && !s.hasVoted(act.Voter)
	default:
		return slices.Contains(LegalActions(s, s.Turn), a)
	}
}

// assessShukh confirms a ШУХ against offender (§8, R-8.5): it emits ShukhAssessed,
// latches the endgame ask-window for codes that close it (Ш-12, R-9.4.2/R-9.4.3),
// then either opens a payment gate (obligated givers exist) or applies the effect
// immediately (nobody owes). The turn-skip (Ш-2/Ш-12) and the 6(2)♥ discard
// obligation (Ш-12) are derived from the code itself (ShukhCode methods).
func (s *State) assessShukh(offender SeatID, code ShukhCode, events *[]Event) {
	*events = append(*events, ShukhAssessed{Offender: offender, Code: code})
	if code.latchesAsked() {
		s.Endgame.Asked = true
	}
	owed := s.owedGivers(offender)
	if len(owed) == 0 {
		s.applyShukhEffect(offender, code.skips(), code.obligesDiscard(), events)
		return
	}
	s.Pending = &Payment{Offender: offender, Owed: owed, Skip: code.skips(), ThenDiscardWest: code.obligesDiscard()}
}

// resolveAdjudication tallies a fully-voted R-8.6 Adjudication (§8, R-8.6): a table
// majority backing the challenge (support*2 > n) moves the penalty onto the
// claimant as Ш-8, otherwise the ШУХ is confirmed on the target. Either way it
// clears the vote and enacts the outcome through the shared §8 machinery
// (assessShukh → payment gate or immediate effect). Precondition: Adjudication != nil
// and every seat has voted.
func (s *State) resolveAdjudication(events *[]Event) {
	adj := s.Adjudication
	support := 0
	for _, v := range adj.Votes {
		if v {
			support++
		}
	}
	overturned := support*2 > len(s.Seats)
	s.Adjudication = nil
	*events = append(*events, VoteResolved{Code: adj.Code, Overturned: overturned})
	if overturned {
		s.assessShukh(adj.Claimant, Sh8, events)
	} else {
		s.assessShukh(adj.Target, adj.Code, events)
	}
}

// owedGivers lists the seats obligated to pay the offender, clockwise from him:
// live, not the offender, holding ≥2 cards (R-8.1/R-8.1.1/I-2 — the last card is
// never given, a 1-card player does not pay).
func (s State) owedGivers(offender SeatID) []SeatID {
	var owed []SeatID
	for _, seat := range s.seatsFrom(offender) {
		if seat != offender && s.Live[seat] && len(s.Hands[seat]) >= 2 {
			owed = append(owed, seat)
		}
	}
	return owed
}

// applyShukhEffect applies a confirmed ШУХ's non-payment consequences (R-8.5):
// mark any non-empty Shukh pile takeable if the con is already over (P-4), skip
// the offender's turn if required (Ш-2/Ш-12), and record the 6(2)♥ obligation
// (Ш-12). The corrected position is then live for play.
func (s *State) applyShukhEffect(offender SeatID, skip, thenDiscardWest bool, events *[]Event) {
	if len(s.Table) == 0 {
		s.markShukhTakeable()
	}
	if thenDiscardWest {
		s.Endgame.MustDiscard = true
	}
	if skip {
		s.settleTurn(s.nextLive(offender), events)
	}
}

// markShukhTakeable makes every non-empty Shukh pile takeable (R-8.3): called when
// the con that held the ШУХ ends — i.e. the table is (or has just become) empty
// (P-4).
func (s *State) markShukhTakeable() {
	for seat, pile := range s.Shukh {
		if len(pile) > 0 {
			s.ShukhTakeable[seat] = true
		}
	}
}

// handSizes snapshots each seat's hand size — the basis reconcileOneCard compares
// against to detect a transition into/out of exactly one card (§15.6).
func handSizes(hands map[SeatID][]Card) map[SeatID]int {
	m := make(map[SeatID]int, len(hands))
	for seat, h := range hands {
		m[seat] = len(h)
	}
	return m
}

// reconcileOneCard updates OwesOneCard by hand-size transition (§15.6): a seat
// crossing INTO exactly one card owes a declaration (R-6.1); a seat leaving one
// card clears it (R-6.3 «успел походить» — any move away from 1 auto-clears);
// staying at one card is left as-is so a prior DeclareOneCard sticks. `before`
// maps each seat to its pre-action hand size.
func (s *State) reconcileOneCard(before map[SeatID]int) {
	for seat := range s.Hands {
		now := len(s.Hands[seat])
		switch {
		case now == 1 && before[seat] != 1:
			s.OwesOneCard[seat] = true
		case now != 1:
			s.OwesOneCard[seat] = false
		}
	}
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
	if s.Adjudication != nil {
		cp := *s.Adjudication
		cp.Votes = make(map[SeatID]bool, len(s.Adjudication.Votes))
		for k, v := range s.Adjudication.Votes {
			cp.Votes[k] = v
		}
		ns.Adjudication = &cp
	}
	return ns
}

// gatesClosed reports whether no catch-window or payment gate is open — i.e. the
// game is in a normal position where turn-actions and fresh social actions are
// available (§15.8: at most one of Unsettled/Pending is active at a time).
func (s State) gatesClosed() bool {
	return s.Unsettled == nil && s.Pending == nil && s.Adjudication == nil
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
	s.markShukhTakeable() // P-4: the con that held any ШУХ has ended

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
// (R-10.1/R-10.1.1). A live seat exits when its hand is empty and it has no card in
// the open con — UNLESS it still owes ШУХ-cards: a penalty must bite (R-9.1), so at
// the moment it would leave it is forced to absorb its owed pile into hand (R-8.3)
// and play those cards off first. When one or zero players remain, the game ends
// and the loser (if any) is appended to Finish.
func (s *State) resolveExits(order []SeatID, events *[]Event) {
	for _, seat := range order {
		if !s.Live[seat] || len(s.Hands[seat]) != 0 || s.handInCon(seat) {
			continue
		}
		if pile := s.Shukh[seat]; len(pile) > 0 {
			// Owes a penalty: absorb it into hand instead of exiting, then play it
			// off before leaving (R-9.1/R-8.3). Forced here — a would-be-exiting seat
			// cannot dodge the penalty by emptying its hand first. This deliberately
			// bypasses the ShukhTakeable timing (R-8.3): the absorb is tied to the
			// exit moment, not to con closure (R-9.1.1).
			s.Shukh[seat] = nil
			s.ShukhTakeable[seat] = false
			s.Hands[seat] = append(s.Hands[seat], pile...)
			*events = append(*events, ShukhCardsTaken{Seat: seat, Cards: pile})
			continue
		}
		s.Live[seat] = false
		s.Finish = append(s.Finish, seat)
		*events = append(*events, PlayerFinished{Seat: seat, Place: len(s.Finish)})
	}
	if s.liveCount() == 2 {
		s.Endgame.Active = true // §9.2: 6(2)♥ now подлежит сбросу (R-9.3)
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
