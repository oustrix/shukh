package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuitIsTrump(t *testing.T) {
	require.True(t, Diamonds.IsTrump(), "Diamonds must be trump (R-2.5)")
	for _, s := range []Suit{Spades, Hearts, Clubs} {
		require.False(t, s.IsTrump(), "%v must not be trump", s)
	}
}

func TestCardString(t *testing.T) {
	cases := []struct {
		card Card
		want string
	}{
		{Card{Diamonds, 9}, "9♦"},
		{Card{Hearts, Queen}, "Q♥"},
		{Card{Spades, 10}, "10♠"},
		{Card{Clubs, Ace}, "A♣"},
		{Card{Hearts, Jack}, "J♥"},
		{Card{Spades, King}, "K♠"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, c.card.String())
	}
}
