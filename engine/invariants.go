package engine

import "fmt"

// CheckInvariants verifies the always-true structural invariants of a stable
// state. This iteration checks I-1 (card conservation): every card of the deck is
// in exactly one zone — Talon, a Hand, Table, Discard, or a Shukh pile — with no
// missing, foreign, or duplicated cards; plus the con's structural shape — I-6
// (Дама♥ never rests on the table) and the beat-stack oracle (⇒ I-7, over a ♠ only
// a higher ♠). Later iterations add I-2/I-3/I-4/I-5. The con-shape invariants (I-6,
// beat-stack ⇒ I-7) are checked only when Unsettled == nil; during a Middle
// catch-window only I-1 is asserted (§15.3).
//
// It returns a typed error describing the first violation, or nil. Callers run it
// after every Apply that yields a stable position (spec §10).
func CheckInvariants(s State) error {
	rs := s.Rules
	seen := make(map[Card]int, rs.deckCount())
	count := func(cs []Card) {
		for _, c := range cs {
			seen[c]++
		}
	}
	count(s.Talon)
	for _, h := range s.Hands {
		count(h)
	}
	count(cardsOf(s.Table))
	count(s.Discard)
	for _, z := range s.Shukh {
		count(z)
	}

	for c, k := range seen {
		if !rs.inDeck(c) {
			return fmt.Errorf("engine: I-1 violated: card %v is not part of a %d-card deck", c, rs.DeckSize)
		}
		if k != 1 {
			return fmt.Errorf("engine: I-1 violated: card %v appears %d times (want 1)", c, k)
		}
	}
	// Every seen card is now distinct (k==1) and in-deck, so the distinct count
	// equals the total; if it matches the deck size, no card is missing.
	if len(seen) != rs.deckCount() {
		return fmt.Errorf("engine: I-1 violated: %d distinct cards present across zones, want %d", len(seen), rs.deckCount())
	}

	// I-6 (strengthened, spec §14.5) + beat-stack oracle: the con is a legal
	// stack — each card legally beats the one below it (⇒ I-7) — and Дама♥ never
	// rests on the table. Literal I-6 (never the заходная card, R-3.7.2) is
	// subsumed: R-3.7.1 makes Дама♥ close+sweep the con the instant it is played,
	// so it never sits on a stable Table. Gated on Unsettled == nil per the function
	// doc — during a Middle catch-window a Дама♥/6(2)♥ may transiently rest here.
	if s.Unsettled == nil {
		for i, tc := range s.Table {
			if IsQueenHearts(tc.Card) {
				return fmt.Errorf("engine: I-6 violated: Дама♥ present on the con")
			}
			if i > 0 && !CanBeat(s.Table[i-1].Card, tc.Card) {
				return fmt.Errorf("engine: beat-stack violated: %v does not legally beat %v", tc.Card, s.Table[i-1].Card)
			}
		}
	}

	// §15.8: at most one adjudication device is open at a time (catch-window,
	// payment gate, or R-8.6 vote) — they are enacted and cleared serially.
	if open := s.openGates(); open > 1 {
		return fmt.Errorf("engine: §15.8 violated: %d adjudication gates open at once", open)
	}

	return nil
}
