package engine

import "fmt"

// Supported deck sizes (R-2.1).
const (
	Deck36 = 36
	Deck52 = 52
)

// RuleSet is the fixed rule configuration of a game (chosen before play and
// never changed mid-game, R-2.1). Variant flags for the optional §12 rules
// exist from day one but are false and unimplemented in the MVP (decision D-8).
type RuleSet struct {
	DeckSize       int  // Deck36 | Deck52
	PodkladkaSnizu bool // §12 V-1…V-4 — MVP: must be false
	Jokers         bool // §12 jokers — MVP: must be false
}

// Validate reports whether the RuleSet is usable by this build.
func (rs RuleSet) Validate() error {
	if rs.DeckSize != Deck36 && rs.DeckSize != Deck52 {
		return fmt.Errorf("engine: unsupported deck size %d (want 36 or 52)", rs.DeckSize)
	}
	if rs.PodkladkaSnizu {
		return fmt.Errorf("engine: PodkladkaSnizu (§12 V-1) is not implemented in the MVP")
	}
	if rs.Jokers {
		return fmt.Errorf("engine: Jokers (§12) are not implemented in the MVP")
	}
	return nil
}

// LowestRank returns the lowest nominal present in the active deck: 6 for a
// 36-card deck, 2 for a 52-card deck (R-2.2, R-2.3).
func (rs RuleSet) LowestRank() Rank {
	if rs.DeckSize == Deck52 {
		return 2
	}
	return 6
}

// Successor returns the next rank up, wrapping Ace → LowestRank (R-4.5).
func (rs RuleSet) Successor(r Rank) Rank {
	if r == Ace {
		return rs.LowestRank()
	}
	return r + 1
}
