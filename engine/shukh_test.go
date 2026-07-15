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

func TestApplyShukhPaymentGate(t *testing.T) {
	// 3 live. Seat 0 commits Ш-2 (Дама♥ заход) and is caught. Owed givers are the
	// live seats ≠ 0 with ≥2 cards, clockwise from 0: seat 1 (2 cards) and seat 2
	// (2 cards). Each gives one non-last card into seat 0's Shukh zone; then the
	// skip applies (turn → seat 1).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}, {Clubs, 9}},
		2: {{Spades, 10}, {Spades, 11}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	ns, _, err = Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)

	// Payment gate open: only seat 1 (head of Owed) may give, only non-last cards.
	require.NotNil(t, ns.Pending)
	require.Equal(t, []SeatID{1, 2}, ns.Pending.Owed)
	require.ElementsMatch(t, []Action{
		GiveShukhCard{Card{Clubs, 8}}, GiveShukhCard{Card{Clubs, 9}},
	}, LegalActions(ns, 1))
	require.Nil(t, LegalActions(ns, 0)) // offender does not pay
	require.Nil(t, LegalActions(ns, 2)) // not the head payer yet

	// Seat 1 pays 8♣.
	ns, ev1, err := Apply(ns, GiveShukhCard{Card{Clubs, 8}})
	require.NoError(t, err)
	require.Contains(t, ev1, ShukhPaid{Offender: 0, From: 1, Card: Card{Clubs, 8}})
	require.Equal(t, []SeatID{2}, ns.Pending.Owed)
	require.Equal(t, []Action{GiveShukhCard{Card{Spades, 10}}, GiveShukhCard{Card{Spades, 11}}}, LegalActions(ns, 2))

	// Seat 2 pays 10♠ → gate closes, effect (skip) applies.
	ns, _, err = Apply(ns, GiveShukhCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Nil(t, ns.Pending)
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Spades, 10}}, ns.Shukh[0]) // I-3: in Shukh, not hand
	require.ElementsMatch(t, []Card{{Hearts, Queen}}, ns.Hands[0])          // hand unchanged by payment
	require.Equal(t, SeatID(1), ns.Turn)                                    // Ш-2 skip past seat 0
	require.True(t, ns.ShukhTakeable[0])                                    // con already over (empty table)
	// (No CheckInvariants — partial-deck unit state does not satisfy I-1; I-2/I-3
	// are asserted structurally above, ns.Shukh[0] holds the paid cards, hands hold
	// the rest. Full I-1/I-3 conservation across the Shukh zone is covered by fuzz.)
}

func TestApplyShukhOneCardPlayerDoesNotPay(t *testing.T) {
	// I-2 (R-8.1.1): a player holding exactly one card never pays. Seat 1 has 1
	// card → excluded from Owed; only seat 2 pays.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}},
		2: {{Spades, 10}, {Spades, 11}},
	}, nil, 0)
	ns, _, _ := Apply(s, PlayCard{Card{Hearts, Queen}})
	ns, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Equal(t, []SeatID{2}, ns.Pending.Owed) // seat 1 (1 card) excluded
}

func TestApplyTakeShukhCardsWhenTakeable(t *testing.T) {
	// Seat 0 has a takeable Shukh pile → lifts it into hand (R-8.3). It is a
	// social action: it does not change whose turn it is.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 1)
	s.Shukh[0] = []Card{{Clubs, 9}, {Diamonds, 10}}
	s.ShukhTakeable[0] = true

	ns, events, err := Apply(s, TakeShukhCards{Seat: 0})
	require.NoError(t, err)
	require.ElementsMatch(t, []Card{{Spades, 7}, {Clubs, 9}, {Diamonds, 10}}, ns.Hands[0])
	require.Empty(t, ns.Shukh[0])
	require.False(t, ns.ShukhTakeable[0])
	require.Equal(t, SeatID(1), ns.Turn) // unchanged
	require.Contains(t, events, ShukhCardsTaken{Seat: 0, Cards: []Card{{Clubs, 9}, {Diamonds, 10}}})
}

func TestApplyEarlyTakeGuardBlocked(t *testing.T) {
	// Guard: an untakeable Shukh pile is not offered, so an early take is rejected.
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 1)
	s.Shukh[0] = []Card{{Clubs, 9}}
	s.ShukhTakeable[0] = false
	_, _, err := Apply(s, TakeShukhCards{Seat: 0})
	require.Error(t, err)
}

func TestApplyEarlyTakeMiddleSetsUnsettledSh3(t *testing.T) {
	// Middle: an early take is allowed but нелегально. It moves the cards to hand
	// and opens a Ш-3 window over the snapshot; a claim reverses it (cards back in
	// the Shukh zone) and assesses Ш-3 (no extra effect). Con is open (7♠ on the
	// table), so nothing settles automatically.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}, {Spades, 6}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, []TableCard{{Card: Card{Spades, 5}, By: 1}}, 0)
	s.Shukh[0] = []Card{{Diamonds, 10}}
	s.ShukhTakeable[0] = false

	ns, _, err := Apply(s, TakeShukhCards{Seat: 0})
	require.NoError(t, err)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, Sh3, ns.Unsettled.Code)
	require.ElementsMatch(t, []Card{{Spades, 7}, {Spades, 6}, {Diamonds, 10}}, ns.Hands[0])

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh3})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)
	require.ElementsMatch(t, []Card{{Diamonds, 10}}, ns2.Shukh[0]) // reversed back
	require.ElementsMatch(t, []Card{{Spades, 7}, {Spades, 6}}, ns2.Hands[0])
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh3})
	// Ш-3 has no extra effect (no skip); seat 1 pays into seat 0's Shukh (2 cards).
	require.NotNil(t, ns2.Pending)
	require.Equal(t, []SeatID{1}, ns2.Pending.Owed)
}

func TestCloseConMarksShukhTakeable(t *testing.T) {
	// A Shukh pile laid during an open con becomes takeable when that con closes
	// (P-4). 2 live, threshold 2: seat 0 beats 8♠ with 10♠ → close → mark.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)
	s.Shukh[1] = []Card{{Clubs, 9}}
	require.False(t, s.ShukhTakeable[1])

	ns, _, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.True(t, ns.ShukhTakeable[1])
}

func TestApplyShukhNobodyOwesAppliesImmediately(t *testing.T) {
	// 2 live, opponent has 1 card → nobody owes → effect applies with no gate.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, _ := Apply(s, PlayCard{Card{Hearts, Queen}})
	ns, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Nil(t, ns.Pending)
	require.Empty(t, ns.Shukh[0])
	require.Equal(t, SeatID(1), ns.Turn) // skip applied directly
}
