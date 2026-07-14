package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDeckSizeAndUniqueness(t *testing.T) {
	for _, rs := range []RuleSet{{DeckSize: Deck36}, {DeckSize: Deck52}} {
		deck := NewDeck(rs)
		require.Len(t, deck, rs.DeckSize, "deck %d: card count mismatch", rs.DeckSize)

		seen := make(map[Card]bool, len(deck))
		for _, c := range deck {
			require.False(t, seen[c], "deck %d: duplicate card %v", rs.DeckSize, c)
			seen[c] = true

			low := rs.LowestRank()
			require.GreaterOrEqual(t, c.Rank, low, "deck %d: card %v rank below minimum", rs.DeckSize, c)
			require.LessOrEqual(t, c.Rank, Ace, "deck %d: card %v rank above maximum", rs.DeckSize, c)
		}
	}
}

func TestNewDeckRankBoundaries(t *testing.T) {
	has := func(deck []Card, c Card) bool {
		for _, x := range deck {
			if x == c {
				return true
			}
		}
		return false
	}

	d36 := NewDeck(RuleSet{DeckSize: Deck36})
	require.False(t, has(d36, Card{Hearts, 2}), "36 deck must not contain 2♥")
	require.True(t, has(d36, Card{Hearts, 6}), "36 deck must contain 6♥")

	d52 := NewDeck(RuleSet{DeckSize: Deck52})
	require.True(t, has(d52, Card{Hearts, 2}), "52 deck must contain 2♥")
	require.True(t, has(d52, Card{Clubs, 5}), "52 deck must contain 5♣")
}
