package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleSetValidate(t *testing.T) {
	cases := []struct {
		name string
		rs   RuleSet
		ok   bool
	}{
		{"36 ok", RuleSet{DeckSize: Deck36}, true},
		{"52 ok", RuleSet{DeckSize: Deck52}, true},
		{"bad size", RuleSet{DeckSize: 40}, false},
		{"zero size", RuleSet{}, false},
		{"podkladka unsupported", RuleSet{DeckSize: Deck36, PodkladkaSnizu: true}, false},
		{"jokers unsupported", RuleSet{DeckSize: Deck36, Jokers: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.rs.Validate()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLowestRank(t *testing.T) {
	got := (RuleSet{DeckSize: Deck36}).LowestRank()
	require.Equal(t, Rank(6), got)

	got = (RuleSet{DeckSize: Deck52}).LowestRank()
	require.Equal(t, Rank(2), got)
}

func TestSuccessor(t *testing.T) {
	rs36 := RuleSet{DeckSize: Deck36}
	rs52 := RuleSet{DeckSize: Deck52}
	cases := []struct {
		rs   RuleSet
		in   Rank
		want Rank
	}{
		{rs36, 8, 9},
		{rs36, King, Ace},
		{rs36, Ace, 6}, // wrap Ace → lowest (R-4.5)
		{rs52, Ace, 2}, // wrap Ace → lowest for 52 deck
		{rs52, 2, 3},
	}
	for _, c := range cases {
		got := c.rs.Successor(c.in)
		require.Equal(t, c.want, got)
	}
}

func TestSpecialCards(t *testing.T) {
	rs36 := RuleSet{DeckSize: Deck36}
	rs52 := RuleSet{DeckSize: Deck52}

	// 6(2)♥ — lowest heart is 6 in a 36 deck, 2 in a 52 deck (R-3.6).
	require.True(t, rs36.IsLowestHeart(Card{Hearts, 6}), "6♥ must be the lowest heart in a 36 deck")
	require.False(t, rs36.IsLowestHeart(Card{Hearts, 2}), "2♥ is not the lowest heart in a 36 deck")
	require.True(t, rs52.IsLowestHeart(Card{Hearts, 2}), "2♥ must be the lowest heart in a 52 deck")
	require.False(t, rs36.IsLowestHeart(Card{Spades, 6}), "6♠ is not a heart")

	// 7(3)♥ — second lowest heart (R-3.6.2).
	require.True(t, rs36.IsSecondLowestHeart(Card{Hearts, 7}), "7♥ must be the second-lowest heart in a 36 deck")
	require.True(t, rs52.IsSecondLowestHeart(Card{Hearts, 3}), "3♥ must be the second-lowest heart in a 52 deck")

	// Дама ♥ and 9♦ are deck-independent (R-3.7, R-5.1).
	require.True(t, IsQueenHearts(Card{Hearts, Queen}), "Q♥ must be recognized")
	require.False(t, IsQueenHearts(Card{Spades, Queen}), "Q♠ is not Дама ♥")
	require.True(t, IsStarter(Card{Diamonds, 9}), "9♦ must be the starter")
	require.False(t, IsStarter(Card{Hearts, 9}), "9♥ is not the starter")
}
