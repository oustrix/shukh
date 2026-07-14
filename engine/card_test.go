package engine

import "testing"

func TestSuitIsTrump(t *testing.T) {
	if !Diamonds.IsTrump() {
		t.Errorf("Diamonds must be trump (R-2.5)")
	}
	for _, s := range []Suit{Spades, Hearts, Clubs} {
		if s.IsTrump() {
			t.Errorf("%v must not be trump", s)
		}
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
		if got := c.card.String(); got != c.want {
			t.Errorf("Card{%v,%d}.String() = %q, want %q", c.card.Suit, c.card.Rank, got, c.want)
		}
	}
}
