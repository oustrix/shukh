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

func TestAskAboutWestNoShukhWhenAlreadyDiscarded(t *testing.T) {
	// Asked but seat 0 no longer holds 6(2)♥ (discarded earlier) → no ШУХ, just
	// closes the безнаказанно window.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	ns, events, err := Apply(s, AskAboutWest{Target: 0})
	require.NoError(t, err)
	require.True(t, ns.Endgame.Asked)
	require.Nil(t, ns.Pending)
	for _, e := range events {
		_, isAssessed := e.(ShukhAssessed)
		require.False(t, isAssessed)
	}
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
