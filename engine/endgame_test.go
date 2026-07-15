package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndgameActivatesAtTwoLive(t *testing.T) {
	// 3 live, threshold 3. Seat 0 beats the third card and closes; seat 2 was
	// handless with its card in the con → exits → 2 live → endgame active.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 11}, {Diamonds, 6}},
		1: {{Clubs, 7}},
		2: {},
	}, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 10}, By: 1},
	}, 0)
	ns, _, err := Apply(s, PlayCard{Card{Spades, 11}})
	require.NoError(t, err)
	require.Equal(t, 2, ns.liveCount())
	require.True(t, ns.Endgame.Active)
}

func TestDiscardWestSendsSixHeartsToDiscard(t *testing.T) {
	// Endgame, seat 0 to open, holds 6♥ (6(2)♥) → DiscardWest sends it to отбой
	// (R-9.3) and passes the turn.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.Contains(t, LegalActions(s, 0), DiscardWest{})

	ns, events, err := Apply(s, DiscardWest{})
	require.NoError(t, err)
	require.Contains(t, ns.Discard, Card{Hearts, 6})
	require.ElementsMatch(t, []Card{{Spades, 7}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Contains(t, events, WestDiscarded{Seat: 0})
}

func TestEndgameForbidsWestPodkladka(t *testing.T) {
	// In the endgame 6(2)♥ must go to отбой, never under 7(3)♥ into a hand (R-9.4.3).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 1}}, 0)
	s.Endgame = EndgameState{Active: true}
	require.NotContains(t, LegalActions(s, 0), PodkladkaWest{})
}

func TestEndgameGuardBlocksSixHeartsZahod(t *testing.T) {
	// Guard: заход with 6(2)♥ in the endgame is «использование» (R-9.4.3) → blocked.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.NotContains(t, LegalActions(s, 0), PlayCard{Card{Hearts, 6}})
	require.Contains(t, LegalActions(s, 0), DiscardWest{})
}

func TestAskAboutWestAssessesSh12(t *testing.T) {
	// Endgame, unasked. Seat 1 asks seat 0, who still holds 6(2)♥ → Ш-12: skip +
	// obligation to discard (R-9.4.2/R-9.4.3). Asked becomes true.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.Contains(t, LegalActions(s, 1), AskAboutWest{Target: 0})

	ns, events, err := Apply(s, AskAboutWest{Target: 0})
	require.NoError(t, err)
	require.True(t, ns.Endgame.Asked)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh12})
	require.NotNil(t, ns.Pending)
	require.True(t, ns.Pending.Skip)
	require.True(t, ns.Pending.ThenDiscardWest)
}

func TestAskAboutWestIllegalWhenTargetLacksWest(t *testing.T) {
	// R-9.4.2: AskAboutWest is legal only against the actual 6(2)♥ holder. In a
	// 2-player endgame the card's location is common knowledge (you see your own
	// hand), so restricting the ask leaks nothing — but it closes a self-shield
	// exploit: without this, the holder could pre-emptively ask a non-holder,
	// burn the single global Asked flag, and dodge Ш-12 for free.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}}, // no 6(2)♥ — not the holder
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.NotContains(t, LegalActions(s, 1), AskAboutWest{Target: 0})

	_, _, err := Apply(s, AskAboutWest{Target: 0})
	require.Error(t, err)
}

func TestClaimShukhSh12LatchesAsked(t *testing.T) {
	// R-9.4.3: a заход-caught Ш-12 (via ClaimShukh) must latch Endgame.Asked too,
	// same as a direct AskAboutWest — otherwise, before the forced DiscardWest, an
	// opponent could AskAboutWest the offender for a second Ш-12 on the same card.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}

	ns, _, err := Apply(s, PlayCard{Card{Hearts, 6}}) // Middle 6(2)♥ заход
	require.NoError(t, err)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, Sh12, ns.Unsettled.Code)

	ns2, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh12})
	require.NoError(t, err)
	require.True(t, ns2.Endgame.Asked)

	// The reversed заход opened a payment gate (seat 1 owes, ≥2 cards). Drive the
	// payment to completion so the gate closes — otherwise a trailing AskAboutWest
	// would be rejected by the still-open gate (gatesClosed()==false), which would
	// NOT isolate the Asked-latch this fix is about.
	require.NotNil(t, ns2.Pending)
	ns3, _, err := Apply(ns2, GiveShukhCard{Card: Card{Clubs, 8}}) // seat 1's non-last card
	require.NoError(t, err)
	require.Nil(t, ns3.Pending)                                    // gate closed
	require.True(t, ns3.Endgame.Asked)                             // latch persisted through payment
	require.Contains(t, ns3.Hands[0], Card{Hearts, 6})            // offender still holds 6(2)♥ (no DiscardWest yet)

	// Now gatesClosed()==true, so this rejection is due to Asked==true alone — the
	// fix's real guarantee: no second ask-triggered Ш-12 before the forced DiscardWest.
	_, _, err = Apply(ns3, AskAboutWest{Target: 0})
	require.Error(t, err)
}

func TestMiddleSixHeartsZahodCaughtAsSh12(t *testing.T) {
	// Middle endgame: seat 0 заходит with 6(2)♥ («использование», R-9.4.3) → Ш-12
	// window. A claim reverses it (6(2)♥ back in hand), skips, and obligates the
	// discard.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	ns, _, err := Apply(s, PlayCard{Card{Hearts, 6}})
	require.NoError(t, err)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, Sh12, ns.Unsettled.Code)

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh12})
	require.NoError(t, err)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh12})
	require.ElementsMatch(t, []Card{{Hearts, 6}, {Spades, 7}}, ns2.Hands[0]) // reversed
	require.True(t, ns2.Pending.ThenDiscardWest)
}
