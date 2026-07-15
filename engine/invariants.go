package engine

import "fmt"

// CheckInvariants verifies the always-true structural invariants of a stable
// state. This iteration checks I-1 (card conservation): every card of the deck is
// in exactly one zone — Talon, a Hand, Table, Discard, or a Shukh pile — with no
// missing, foreign, or duplicated cards. Later iterations add I-2/I-4/I-5/I-6/I-7.
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
	count(s.Table)
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
	return nil
}
