package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// middle builds a playing() state switched to Middle.
func middle(hands map[SeatID][]Card, table []TableCard, turn SeatID) State {
	s := playing(hands, table, turn)
	s.Mode = Middle
	return s
}

func TestApplyMiddleQueenZahodSetsUnsettled(t *testing.T) {
	// Middle, empty con: seat 0 заходит with Дама♥ — allowed but нелегально. It
	// lands on the table, Unsettled snapshots the pre-action state, and the turn
	// passes to seat 1 (who can only take it — Дама♥ is unbeatable).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)

	ns, events, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	require.Equal(t, []TableCard{{Card: Card{Hearts, Queen}, By: 0}}, ns.Table)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, SeatID(0), ns.Unsettled.Seat)
	require.Equal(t, Sh2, ns.Unsettled.Code)
	require.Equal(t, SeatID(1), ns.Turn)
	// Seat 1 can only take Дама♥ (unbeatable → no бой) or claim the Ш-2: R-8.9 lets
	// the next-to-act player also catch the ШУХ (required for heads-up).
	require.ElementsMatch(t, []Action{ClaimShukh{Target: 0, Code: Sh2}, TakeBottomAndPass{}}, LegalActions(ns, 1))
	_ = events
}

func TestApplyMiddleQueenZahodSettledByNextMove(t *testing.T) {
	// If nobody claims, the next player's move «прижимает» the заход (R-1.4.1):
	// seat 1 takes the Дама♥, the window closes, and it becomes a normal position.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)

	ns2, _, err := Apply(ns, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)                              // settled
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Hearts, Queen}}, ns2.Hands[1])
	// I-6-restored on the stable position is asserted by the Task 12 fuzz (partial-deck unit states don't satisfy I-1).
}

func TestApplyClaimShukhReversesQueenZahodAndSkips(t *testing.T) {
	// A claim in the window reverses the заход (Дама♥ back in seat 0's hand) and
	// assesses Ш-2 → skip seat 0's turn. (No payment in this task.)
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)
	require.Empty(t, ns2.Table)                                 // reversed off the table
	require.ElementsMatch(t, []Card{{Hearts, Queen}, {Spades, 7}}, ns2.Hands[0])
	require.Equal(t, SeatID(1), ns2.Turn)                       // seat 0 skipped (Ш-2)
	require.Contains(t, events, ActionReverted{Seat: 0})
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh2})
	// I-6-restored on the stable position is asserted by the Task 12 fuzz (partial-deck unit states don't satisfy I-1).
}

func TestApplyClaimShukhRejectedWithoutWindow(t *testing.T) {
	s := middle(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err := Apply(s, ClaimShukh{Target: 0, Code: Sh2}) // no window
	require.Error(t, err)
	// Wrong code while a window is open is also rejected.
	ns, _, _ := Apply(middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}}, 1: {{Clubs, 8}},
	}, nil, 0), PlayCard{Card{Hearts, Queen}})
	_, _, err = Apply(ns, ClaimShukh{Target: 0, Code: Sh3})
	require.Error(t, err)
}
