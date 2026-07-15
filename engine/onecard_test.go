package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwesOneCardSetWhenHandBecomesOne(t *testing.T) {
	// Seat 0 plays down to one card (заход leaves 1) → OwesOneCard set (R-6.1a).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Spades, 7}})
	require.NoError(t, err)
	require.True(t, ns.OwesOneCard[0])
	require.False(t, ns.OwesOneCard[1])
}

func TestOwesOneCardClearedByMove(t *testing.T) {
	// R-6.3 «успел походить» falls out for free: playing the 1-card hand drops to
	// 0 → hand != 1 → flag auto-clears. (Here seat 0 takes a card, going to 2.)
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 7}, By: 1}}, 0)
	s.OwesOneCard[0] = true
	ns, _, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.False(t, ns.OwesOneCard[0]) // now 2 cards
}

func TestDeclareOneCardClearsFlag(t *testing.T) {
	// Declaring clears the obligation; the flag stays clear while the hand is 1.
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 1)
	s.OwesOneCard[0] = true
	require.Equal(t, []Action{DeclareOneCard{Seat: 0}}, LegalActions(s, 0))

	ns, events, err := Apply(s, DeclareOneCard{Seat: 0})
	require.NoError(t, err)
	require.False(t, ns.OwesOneCard[0])
	require.Contains(t, events, OneCardDeclared{Seat: 0})
	require.Nil(t, LegalActions(ns, 0)) // nothing more to declare; not its turn
}
