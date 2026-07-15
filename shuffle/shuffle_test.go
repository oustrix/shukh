package shuffle_test

import (
	"sort"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"

	"github.com/stretchr/testify/require"
)

func key(c engine.Card) int { return int(c.Suit)*100 + int(c.Rank) }

func sortedKeys(cs []engine.Card) []int {
	ks := make([]int, len(cs))
	for i, c := range cs {
		ks[i] = key(c)
	}
	sort.Ints(ks)
	return ks
}

func TestDeckDeterministic(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	a := shuffle.Deck(full, 42)
	b := shuffle.Deck(full, 42)
	require.Equal(t, a, b, "same seed must yield the same permutation")
}

func TestDeckIsPermutation(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	got := shuffle.Deck(full, 7)
	require.Len(t, got, len(full))
	require.Equal(t, sortedKeys(full), sortedKeys(got), "shuffle must preserve the multiset of cards")
}

func TestDeckDoesNotMutateInput(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	before := append([]engine.Card(nil), full...)
	_ = shuffle.Deck(full, 99)
	require.Equal(t, before, full, "input slice must be untouched")
}

func TestDeckDifferentSeedsDiffer(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	require.NotEqual(t, shuffle.Deck(full, 1), shuffle.Deck(full, 2),
		"different seeds should (overwhelmingly likely) differ for a 36-card deck")
}
