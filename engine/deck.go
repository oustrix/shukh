package engine

// NewDeck returns a full, ordered (unshuffled) deck for the given rules: 36
// cards (ranks 6..A) or 52 cards (ranks 2..A), four suits each (R-2.1, R-2.2).
// The caller is responsible for shuffling with an external seed — the engine
// keeps no randomness of its own (decision D-7).
func NewDeck(rs RuleSet) []Card {
	suits := [4]Suit{Spades, Hearts, Diamonds, Clubs}
	low := rs.LowestRank()
	deck := make([]Card, 0, rs.deckCount())
	for _, s := range suits {
		for r := low; r <= Ace; r++ {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	return deck
}

// deckCount is the number of cards in the deck for these rules: four suits ×
// (Ace − LowestRank + 1) ranks (R-2.1, R-2.2). This equals len(NewDeck(rs))
// without building the slice.
func (rs RuleSet) deckCount() int {
	return 4 * int(Ace-rs.LowestRank()+1)
}

// inDeck reports whether c belongs to the deck for these rules — a valid suit and
// a rank between LowestRank and Ace (R-2.2). Equivalent to membership in
// NewDeck(rs), but a pure arithmetic check with no allocation.
func (rs RuleSet) inDeck(c Card) bool {
	return c.Suit <= Clubs && c.Rank >= rs.LowestRank() && c.Rank <= Ace
}
