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

func TestApplyBeatNoClose(t *testing.T) {
	// 3 live players, threshold 3. Con has 1 card (8♠); seat 0 beats with 10♠.
	// len(table) becomes 2 < 3 → no close, turn passes to seat 1.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
		2: {{Hearts, 9}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 2}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 10}, By: 0},
	}, ns.Table)
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, []Event{CardPlayed{Seat: 0, Card: Card{Spades, 10}}}, events)
	require.Empty(t, ns.Discard)
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

func TestApplyRejectsUnimplementedActions(t *testing.T) {
	// TakeBottomAndPass and PodkladkaWest are legal per LegalActions but not yet
	// wired into Apply (Tasks 7/8). Until then Apply must reject them with a typed
	// error, never silently no-op. (This test is superseded when those tasks land.)
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 1}}, 0)

	for _, a := range []Action{TakeBottomAndPass{}, PodkladkaWest{}} {
		ns, events, err := Apply(s, a)
		var illegal *IllegalAction
		require.ErrorAs(t, err, &illegal, "%T must be rejected, not silently applied", a)
		require.Nil(t, events)
		require.Empty(t, ns.Table[:0]) // input untouched: table still holds the 7♥
	}
	require.Len(t, s.Table, 1) // Apply did not mutate the input
}
