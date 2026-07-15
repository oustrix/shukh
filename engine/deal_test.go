package engine

import "testing"

import "github.com/stretchr/testify/require"

// Golden A (3 players, shuffler P0): opponent-forwarding + self-terminal, and the
// last card (8♥) goes to the drawer (P0) even though it would fit P2 — R-4.9.
//
// Trace: P0 seeds 8♠ (R-4.7). P1 draws 9♠→P0, Q♠→self. P2 draws 10♠→P0, K♠→P1,
// 7♥→self. P0 draws last 8♥→self (R-4.9).
func TestDealAllGoldenForwarding(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1, 2}
	deck := []Card{
		{Spades, 8}, {Spades, 9}, {Spades, Queen},
		{Spades, 10}, {Spades, King}, {Hearts, 7}, {Hearts, 8},
	}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, 8}, {Spades, 9}, {Spades, 10}, {Hearts, 8}}, got[0])
	require.Equal(t, []Card{{Spades, Queen}, {Spades, King}}, got[1])
	require.Equal(t, []Card{{Hearts, 7}}, got[2])
}

// Golden B (2 players, shuffler P0): the R-4.4.1 unload. On P1's final turn its own
// top 8♠ fits P0 (top 7♠), so P1 MUST move 8♠ to P0 before drawing; then P1 draws
// the last card 9♥ to self (R-4.9).
//
// Trace: P0 seeds 6♠. P1 draws 8♠→self. P0 draws 7♠→self. P1 unloads 8♠→P0, draws
// last 9♥→self.
func TestDealAllGoldenUnload(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1}
	deck := []Card{{Spades, 6}, {Spades, 8}, {Spades, 7}, {Hearts, 9}}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, 6}, {Spades, 7}, {Spades, 8}}, got[0])
	require.Equal(t, []Card{{Hearts, 9}}, got[1])
}

// Golden C (2 players, shuffler P0): the Ace→6 wrap (R-4.5). P1 draws 6♥, which
// fits P0's Ace top (Successor(Ace)=6), so 6♥ goes onto the Ace; then 9♠ is the
// last card → self.
func TestDealAllGoldenAceWrap(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1}
	deck := []Card{{Spades, Ace}, {Hearts, 6}, {Spades, 9}}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, Ace}, {Hearts, 6}}, got[0])
	require.Equal(t, []Card{{Spades, 9}}, got[1])
}

// Conservation: dealAll never loses or duplicates a card (I-1 at the dealing level).
func TestDealAllConservesCards(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1, 2}
	deck := []Card{
		{Spades, 8}, {Spades, 9}, {Spades, Queen},
		{Spades, 10}, {Spades, King}, {Hearts, 7}, {Hearts, 8},
	}
	got := dealAll(rs, seats, deck)
	seen := map[Card]int{}
	total := 0
	for _, s := range seats {
		for _, c := range got[s] {
			seen[c]++
			total++
		}
	}
	require.Equal(t, len(deck), total)
	for _, c := range deck {
		require.Equal(t, 1, seen[c], "card %v must appear exactly once", c)
	}
}
