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

func TestClaimShukhPreservesDeclaredFlag(t *testing.T) {
	// R-6.3 sticky: an offender who already declared «одна карта» (OwesOneCard
	// false at 1 card) must NOT be re-flagged as owing after their Ш-2 is reversed.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}}, // lone Дама♥, already declared
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.OwesOneCard[0] = false // declared

	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}}) // Middle Ш-2 заход
	require.NoError(t, err)
	require.False(t, ns.OwesOneCard[0]) // hand went to 0

	ns2, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2}) // reverse
	require.NoError(t, err)
	require.False(t, ns2.OwesOneCard[0]) // snapshot restored hand=1, flag must stay false (not re-set true)
}

func TestAskCountAssessesSh11(t *testing.T) {
	// Seat 0 owes «одна карта» and hasn't declared → asking triggers Ш-11 (R-6.2).
	// The other players pay into seat 0's Shukh zone (no skip for Ш-11).
	s := middle(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.OwesOneCard[0] = true
	require.Contains(t, LegalActions(s, 1), AskCount{Target: 0})

	ns, events, err := Apply(s, AskCount{Target: 0})
	require.NoError(t, err)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh11})
	require.NotNil(t, ns.Pending)
	require.Equal(t, []SeatID{1}, ns.Pending.Owed)
	require.False(t, ns.Pending.Skip) // Ш-11 has no turn-skip
}

func TestAskCountRejectedWhenNoObligation(t *testing.T) {
	// No obligation → false trigger → rejected (its Ш-8/Ш-9 punishment is Спец 2).
	s := middle(map[SeatID][]Card{0: {{Diamonds, 9}, {Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err := Apply(s, AskCount{Target: 0})
	require.Error(t, err)
}
