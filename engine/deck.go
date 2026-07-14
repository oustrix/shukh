package engine

// NewDeck returns a full, ordered (unshuffled) deck for the given rules: 36
// cards (ranks 6..A) or 52 cards (ranks 2..A), four suits each (R-2.1, R-2.2).
// The caller is responsible for shuffling with an external seed — the engine
// keeps no randomness of its own (decision D-7).
func NewDeck(rs RuleSet) []Card {
	suits := [4]Suit{Spades, Hearts, Diamonds, Clubs}
	low := rs.LowestRank()
	deck := make([]Card, 0, 4*int(Ace-low+1))
	for _, s := range suits {
		for r := low; r <= Ace; r++ {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	return deck
}
