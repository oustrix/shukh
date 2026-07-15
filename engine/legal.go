package engine

import "slices"

// LegalActions lists the actions `seat` may take right now in Guard (§14.2). Only
// the player to move has actions this iteration (social/ШУХ actions arrive in
// iteration 4); other seats and a finished game yield nil. It is the executable
// specification of §3/§5 that Apply validates against.
func LegalActions(s State, seat SeatID) []Action {
	if s.Phase == Finished {
		return nil
	}
	// An open Middle catch-window: any live seat ≠ offender may claim it; the
	// offender's next-in-turn player may instead settle it with a normal move.
	if s.Unsettled != nil {
		var out []Action
		if seat != s.Unsettled.Seat && s.Live[seat] {
			out = append(out, ClaimShukh{Target: s.Unsettled.Seat, Code: s.Unsettled.Code})
		}
		if seat == s.Turn {
			out = append(out, turnActions(s, seat)...) // the settling move
		}
		return out
	}
	if seat != s.Turn {
		return nil
	}
	return turnActions(s, seat)
}

// turnActions lists the turn-actions the seat to move may take in a normal
// position (§3/§5): a заход onto an empty con (R-5.2), the forced take when
// handless-but-live (R-5.9), or a бой / take / западло on a non-empty con. It
// assumes seat == s.Turn and no gate is open (callers gate this).
func turnActions(s State, seat SeatID) []Action {
	hand := s.Hands[seat]

	if len(s.Table) == 0 {
		// Заход: any card but Дама♥ (R-5.2). A lone Дама♥ yields nil — the Guard
		// skip (§14.4) keeps Turn from ever resting here.
		var out []Action
		for _, c := range hand {
			// Дама♥ заход (R-3.7.2): Guard blocks it (§14.4); Middle allows it and
			// catches it as Ш-2 via the Unsettled window (§15.3).
			if IsQueenHearts(c) && s.Mode == Guard {
				continue
			}
			out = append(out, PlayCard{Card: c})
		}
		return out
	}

	if len(hand) == 0 {
		// Handless but live: a card of theirs hangs in the open con (R-5.9); the
		// only move is to take the bottom.
		return []Action{TakeBottomAndPass{}}
	}

	top := s.Table[len(s.Table)-1].Card
	var out []Action
	for _, c := range hand {
		if CanBeat(top, c) { // §3 (Дама♥ beats anything — its legal use is a бой)
			out = append(out, PlayCard{Card: c})
		}
	}
	out = append(out, TakeBottomAndPass{}) // R-5.3b: always available on a non-empty con
	if s.Rules.IsSecondLowestHeart(s.Table[0].Card) && slices.ContainsFunc(hand, s.Rules.IsLowestHeart) {
		out = append(out, PodkladkaWest{}) // R-5.3c/R-3.6.2
	}
	return out
}
