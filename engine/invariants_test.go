package engine

import "testing"

import "github.com/stretchr/testify/require"

// fullState builds a minimal valid State whose hands hold the entire deck (as if
// just dealt), so I-1 holds.
func fullState(rs RuleSet) State {
	deck := NewDeck(rs)
	return State{
		Rules: rs,
		Seats: []SeatID{0, 1},
		Hands: map[SeatID][]Card{0: deck, 1: {}},
		Shukh: map[SeatID][]Card{},
	}
}

func TestCheckInvariantsI1Holds(t *testing.T) {
	require.NoError(t, CheckInvariants(fullState(RuleSet{DeckSize: Deck36})))
	require.NoError(t, CheckInvariants(fullState(RuleSet{DeckSize: Deck52})))
}

func TestCheckInvariantsI1MissingCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = s.Hands[0][:len(s.Hands[0])-1] // drop one card
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI1DuplicateCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	dup := s.Hands[0][0]
	s.Hands[1] = []Card{dup} // same card now in two zones
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI1ForeignCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Table = []TableCard{{Card: Card{Hearts, 2}}} // 2♥ is not in a 36-card deck
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsBeatStackOK(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	// A legal stack: 8♠ then higher ♠ 10♠. Move them out of a hand to keep I-1.
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 8})
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 10})
	s.Table = []TableCard{
		{Card: Card{Spades, 8}, By: 0},
		{Card: Card{Spades, 10}, By: 0},
	}
	require.NoError(t, CheckInvariants(s))
}

func TestCheckInvariantsBeatStackViolation(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 8})
	s.Hands[0] = removeCard(s.Hands[0], Card{Diamonds, 14})
	// ♦ over ♠ is illegal (I-7): the beat-stack oracle must reject it.
	s.Table = []TableCard{
		{Card: Card{Spades, 8}, By: 0},
		{Card: Card{Diamonds, 14}, By: 0},
	}
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI6QueenOnTable(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = removeCard(s.Hands[0], Card{Hearts, Queen})
	s.Table = []TableCard{{Card: Card{Hearts, Queen}, By: 0}} // I-6: never rests on table
	require.Error(t, CheckInvariants(s))
}
