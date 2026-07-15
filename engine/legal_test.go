package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// playing builds a minimal in-progress Guard state: seats 0..n-1 (all live),
// 36-card rules, given hands/table, and whose turn it is. It does NOT enforce
// I-1 — unit tests assert transition specifics on focused states.
func playing(hands map[SeatID][]Card, table []TableCard, turn SeatID) State {
	n := len(hands)
	seats := make([]SeatID, n)
	live := make(map[SeatID]bool, n)
	for i := 0; i < n; i++ {
		seats[i] = SeatID(i)
		live[SeatID(i)] = true
	}
	return State{
		Rules:         RuleSet{DeckSize: Deck36},
		Mode:          Guard,
		Seats:         seats,
		Phase:         Playing,
		Hands:         hands,
		Table:         table,
		Shukh:         map[SeatID][]Card{},
		ShukhTakeable: map[SeatID]bool{},
		Live:          live,
		Turn:          turn,
	}
}

func TestLegalActionsZahod(t *testing.T) {
	// Empty con → any card but Дама♥ is a legal заход (R-5.2).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Hearts, Queen}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 7}},
		PlayCard{Card{Diamonds, 9}},
	}, LegalActions(s, 0))
}

func TestLegalActionsBeatAndTake(t *testing.T) {
	// Top is 8♠; only a higher ♠ beats it (I-7). Take is always available.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Spades, 6}, {Diamonds, 14}},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 10}},
		TakeBottomAndPass{},
	}, LegalActions(s, 0))
}

func TestLegalActionsPodkladkaWest(t *testing.T) {
	// Bottom is 7♥ (7(3)♥ for a 36-deck) and hand has 6♥ (6(2)♥) → западло offered
	// alongside take (R-3.6.2/R-5.3c).
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 1}}, 0)
	require.ElementsMatch(t, []Action{
		TakeBottomAndPass{},
		PodkladkaWest{},
	}, LegalActions(s, 0))
}

func TestLegalActionsHandlessForcedTake(t *testing.T) {
	// Empty hand but a card hangs in the open con → only move is take (R-5.9).
	s := playing(map[SeatID][]Card{
		0: {},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 0}}, 0)
	require.Equal(t, []Action{TakeBottomAndPass{}}, LegalActions(s, 0))
}

func TestLegalActionsNotYourTurnOrFinished(t *testing.T) {
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	require.Nil(t, LegalActions(s, 1)) // not seat 1's turn
	s.Phase = Finished
	require.Nil(t, LegalActions(s, 0)) // game over
}

func TestLegalActionsLoneQueenIsEmpty(t *testing.T) {
	// Empty con, only card is Дама♥ → no legal заход (Guard blocks it, §14.4).
	s := playing(map[SeatID][]Card{0: {{Hearts, Queen}}, 1: {{Clubs, 8}}}, nil, 0)
	require.Empty(t, LegalActions(s, 0))
}

func TestLegalActionsMiddleAllowsQueenZahod(t *testing.T) {
	// Empty con. In Guard the Дама♥ заход is blocked (§14.4); in Middle it is
	// allowed and caught as Ш-2 (§15.3), so it appears among legal заходы.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Hearts, Queen}},
		1: {{Clubs, 8}},
	}, nil, 0)

	require.ElementsMatch(t, []Action{PlayCard{Card{Spades, 7}}}, LegalActions(s, 0)) // Guard

	s.Mode = Middle
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 7}},
		PlayCard{Card{Hearts, Queen}},
	}, LegalActions(s, 0))
}
