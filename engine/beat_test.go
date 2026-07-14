package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanBeat(t *testing.T) {
	cases := []struct {
		name string
		top  Card
		c    Card
		want bool
	}{
		// ♠ pika — only a higher spade; trump does NOT beat it (R-3.3, I-7).
		{"spade beaten by higher spade", Card{Spades, 9}, Card{Spades, 10}, true},
		{"spade not beaten by lower spade", Card{Spades, 10}, Card{Spades, 9}, false},
		{"spade not beaten by equal spade", Card{Spades, 9}, Card{Spades, 9}, false},
		{"spade not beaten by trump", Card{Spades, 9}, Card{Diamonds, Ace}, false},
		{"spade not beaten by heart", Card{Spades, 9}, Card{Hearts, King}, false},

		// ♦ trump — only a higher diamond (R-3.1).
		{"diamond beaten by higher diamond", Card{Diamonds, 9}, Card{Diamonds, 10}, true},
		{"diamond not beaten by lower diamond", Card{Diamonds, 10}, Card{Diamonds, 9}, false},
		{"diamond not beaten by spade", Card{Diamonds, 9}, Card{Spades, Ace}, false},
		{"diamond not beaten by heart", Card{Diamonds, 9}, Card{Hearts, Ace}, false},

		// ♥ / ♣ — higher same suit OR any diamond (R-3.1, R-3.2).
		{"heart beaten by higher heart", Card{Hearts, 9}, Card{Hearts, 10}, true},
		{"heart not beaten by lower heart", Card{Hearts, 10}, Card{Hearts, 9}, false},
		{"heart beaten by low trump", Card{Hearts, Ace}, Card{Diamonds, 6}, true},
		{"heart not beaten by club", Card{Hearts, 9}, Card{Clubs, King}, false},
		{"heart not beaten by spade", Card{Hearts, 9}, Card{Spades, King}, false},
		{"club beaten by higher club", Card{Clubs, 9}, Card{Clubs, 10}, true},
		{"club beaten by low trump", Card{Clubs, Ace}, Card{Diamonds, 6}, true},
		{"club not beaten by heart", Card{Clubs, 9}, Card{Hearts, King}, false},

		// Дама ♥ beats anything (R-3.7.1).
		{"queen hearts beats spade", Card{Spades, Ace}, Card{Hearts, Queen}, true},
		{"queen hearts beats trump", Card{Diamonds, Ace}, Card{Hearts, Queen}, true},
		{"queen hearts beats club", Card{Clubs, Ace}, Card{Hearts, Queen}, true},

		// 6♥ (lowest heart) beats nothing it could be played against (R-3.6).
		{"lowest heart beats nothing (spade)", Card{Spades, 7}, Card{Hearts, 6}, false},
		{"lowest heart beats nothing (heart)", Card{Hearts, 7}, Card{Hearts, 6}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, CanBeat(c.top, c.c))
		})
	}
}
