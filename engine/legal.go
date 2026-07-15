package engine

import "slices"

// LegalActions lists the actions `seat` may take right now in Guard (§14.2). Only
// the player to move has actions this iteration (social/ШУХ actions arrive in
// iteration 4); other seats and a finished game yield nil. It is the executable
// specification of §3/§5 that Apply validates against.
func LegalActions(s State, seat SeatID) []Action {
	if s.Phase == Finished || seat != s.Turn {
		return nil
	}
	hand := s.Hands[seat]

	if len(s.Table) == 0 {
		// Заход: any card but Дама♥ (R-5.2). A lone Дама♥ yields nil — the Guard
		// skip (§14.4) keeps Turn from ever resting here.
		var out []Action
		for _, c := range hand {
			if !IsQueenHearts(c) {
				out = append(out, PlayCard{Card: c})
			}
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
	if s.Rules.IsSecondLowestHeart(s.Table[0].Card) {
		west := Card{Suit: Hearts, Rank: s.Rules.LowestRank()}
		if slices.Contains(hand, west) {
			out = append(out, PodkladkaWest{}) // R-5.3c/R-3.6.2
		}
	}
	return out
}
