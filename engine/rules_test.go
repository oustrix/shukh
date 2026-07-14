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
