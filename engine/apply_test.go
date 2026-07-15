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

func TestApplyCloseByCount(t *testing.T) {
	// 2 live, threshold 2. Con has 8♠; seat 0 beats with 10♠ → len 2 == 2 → close.
	// Both keep other cards, so nobody exits; closer (0) opens next; con → discard.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Spades, 8}, {Spades, 10}}, ns.Discard)
	require.Equal(t, SeatID(0), ns.Turn) // closer opens (R-5.7)
	require.Equal(t, Playing, ns.Phase)
	require.Equal(t, []Event{
		CardPlayed{Seat: 0, Card: Card{Spades, 10}},
		ConClosed{By: 0},
		ConSwept{Cards: []Card{{Spades, 8}, {Spades, 10}}},
	}, events)
}

func TestApplyCloseExitsCloserAndEndsGame(t *testing.T) {
	// 2 live, threshold 2. Seat 1 opened 8♠ with its LAST card; seat 0 beats with
	// its LAST card 10♠ → close. Sweep empties the table → both are handless with
	// no card in con → both exit. liveCount 0 → game over. Order: clockwise from
	// closer (0) → [0, 1]; loser is the last-placed (1).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}},
		1: {},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, Finished, ns.Phase)
	require.Equal(t, []SeatID{0, 1}, ns.Finish) // 0 first (winner), 1 last (loser)
	require.Contains(t, events, PlayerFinished{Seat: 0, Place: 1})
	require.Contains(t, events, PlayerFinished{Seat: 1, Place: 2})
	require.Contains(t, events, GameFinished{Finish: []SeatID{0, 1}})
}

func TestApplyCloserExitedNextOpens(t *testing.T) {
	// 3 live, threshold 3. Con has 8♠,9♠ (by seats 2,1); seat 0 beats with its
	// LAST card 10♠ → len 3 == 3 → close. Seat 0 is now handless, table swept →
	// seat 0 exits. Closer exited → next live clockwise (seat 1) opens (R-5.7.2).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}},
		1: {{Clubs, 7}},
		2: {{Hearts, 9}},
	}, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 9}, By: 1},
	}, 0)

	ns, _, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, Playing, ns.Phase)
	require.False(t, ns.Live[0])
	require.Equal(t, []SeatID{0}, ns.Finish)
	require.Equal(t, SeatID(1), ns.Turn) // R-5.7.2
}

func TestApplyQueenHeartsImmediateClose(t *testing.T) {
	// 3 live, threshold 3, con has just 1 card. Seat 0 beats with Дама♥ → closes
	// immediately regardless of count (R-3.7.1). Closer (0) keeps a card and opens.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Diamonds, 6}},
		1: {{Clubs, 8}},
		2: {{Hearts, 9}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Spades, 8}, {Hearts, Queen}}, ns.Discard)
	require.Equal(t, SeatID(0), ns.Turn)
	require.Contains(t, events, ConClosed{By: 0})
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

func TestApplyPodkladkaWest(t *testing.T) {
	// 3 live. Con bottom is 7♥ (by seat 2); seat 0 tucks 6♥ under → whole con
	// (6♥ + 7♥) goes to the next live seat (1) who opens next (R-5.7.1). Seat 0
	// shed its last card but seat 1 ate the con, so nobody's cards remain on an
	// (now empty) table; seat 0 becomes handless with no card in con → exits.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 8}},
		2: {{Diamonds, 9}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 2}}, 0)

	ns, events, err := Apply(s, PodkladkaWest{})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.Empty(t, ns.Discard) // eaten, not swept
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Hearts, 6}, {Hearts, 7}}, ns.Hands[1])
	require.False(t, ns.Live[0]) // shed last card, exits
	require.Equal(t, SeatID(1), ns.Turn)
	require.Contains(t, events, PodkladkaPlayed{Seat: 0, Eater: 1})
	require.Contains(t, events, CardsTaken{Seat: 1, Cards: []Card{{Hearts, 6}, {Hearts, 7}}})
}

func TestApplyTakeBottom(t *testing.T) {
	// 2 live. Con has 8♠ (by seat 1). Seat 0 takes it → hand gains 8♠, con empty,
	// turn passes to seat 1 (who then must заход). Seat 1 still has a card, so no
	// exit; game continues.
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Diamonds, 6}, {Spades, 8}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, Playing, ns.Phase)
	require.Equal(t, []Event{CardsTaken{Seat: 0, Cards: []Card{{Spades, 8}}}}, events)
}

func TestApplyTakeBottomExitsOwnerAndEndsGame(t *testing.T) {
	// 2 live. Seat 1 is handless-but-live: its only card (8♠) is the con bottom
	// (R-5.9). Seat 0 takes it → seat 1 now has empty hand and no card in con →
	// exits. liveCount 1 → game over; seat 0 is the loser (last place).
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 6}},
		1: {},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Equal(t, Finished, ns.Phase)
	require.False(t, ns.Live[1])
	require.Equal(t, []SeatID{1, 0}, ns.Finish) // 1 out first (winner), 0 loser
	require.Contains(t, events, PlayerFinished{Seat: 1, Place: 1})
	require.Contains(t, events, GameFinished{Finish: []SeatID{1, 0}})
}
