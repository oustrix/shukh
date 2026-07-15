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
	full := NewDeck(s.Rules)
	want := make(map[Card]bool, len(full))
	for _, c := range full {
		want[c] = true
	}

	seen := make(map[Card]int, len(full))
	total := 0
	count := func(cs []Card) {
		for _, c := range cs {
			seen[c]++
			total++
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
		if !want[c] {
			return fmt.Errorf("engine: I-1 violated: card %v is not part of a %d-card deck", c, s.Rules.DeckSize)
		}
		if k != 1 {
			return fmt.Errorf("engine: I-1 violated: card %v appears %d times (want 1)", c, k)
		}
	}
	if total != len(full) {
		return fmt.Errorf("engine: I-1 violated: %d cards present across zones, want %d", total, len(full))
	}
	return nil
}
