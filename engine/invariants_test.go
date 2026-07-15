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
