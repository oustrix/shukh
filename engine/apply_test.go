package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyZahod(t *testing.T) {
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 7}})
	require.NoError(t, err)

	// Card moved hand→table with owner 0; turn passed to seat 1.
	require.Equal(t, []TableCard{{Card: Card{Spades, 7}, By: 0}}, ns.Table)
	require.ElementsMatch(t, []Card{{Diamonds, 9}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, []Event{CardPlayed{Seat: 0, Card: Card{Spades, 7}}}, events)

	// Input is untouched (Apply is pure).
	require.Empty(t, s.Table)
	require.Len(t, s.Hands[0], 2)
}

func TestApplyRejectsIllegal(t *testing.T) {
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)

	// Card not in hand.
	_, _, err := Apply(s, PlayCard{Card{Hearts, 10}})
	require.Error(t, err)
	var illegal *IllegalAction
	require.ErrorAs(t, err, &illegal)

	// Дама♥ заход is blocked in Guard.
	s2 := playing(map[SeatID][]Card{0: {{Hearts, Queen}, {Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err = Apply(s2, PlayCard{Card{Hearts, Queen}})
	require.Error(t, err)
}
