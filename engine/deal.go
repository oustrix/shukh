package engine

import (
	"fmt"
	"slices"
)

// dealAll plays out the automatic §4 deal: it turns an ordered deck into each
// seat's starting pile (R-4.3…R-4.10). deck[0] is the top of the talon (drawn
// first); seats[0] is the shuffler (R-4.7). A pile is stored bottom→top, so its
// top card — the only one that matters during dealing (R-4.2) — is pile[len-1].
//
// The move set per turn (spec §7):
//   - R-4.4.1: unload — while my top fits an opponent by "+1", give it to the
//     first such opponent clockwise (R-4.6); repeat. Only opponents receive.
//   - R-4.4.2/3: draw — a drawn card that fits an opponent goes to the first such
//     opponent clockwise, then draw again.
//   - R-4.4.4: a drawn card that fits no opponent goes onto my own pile; turn ends.
//   - R-4.9: the LAST card of the talon always goes to the drawer's own pile,
//     even if it would fit an opponent.
//
// "Fits by +1" is Successor(oppTop.Rank) == card.Rank, with Successor wrapping
// Ace→lowest (R-4.5); suit is ignored (R-4.3).
func dealAll(rs RuleSet, seats []SeatID, deck []Card) map[SeatID][]Card {
	n := len(seats)
	piles := make(map[SeatID][]Card, n)
	for _, s := range seats {
		piles[s] = nil
	}

	idx := 0 // index of the next card to draw (top of talon)
	remaining := func() int { return len(deck) - idx }
	draw := func() Card { c := deck[idx]; idx++; return c }

	topOf := func(s SeatID) (Card, bool) {
		p := piles[s]
		if len(p) == 0 {
			return Card{}, false
		}
		return p[len(p)-1], true
	}
	pop := func(s SeatID) { piles[s] = piles[s][:len(piles[s])-1] }
	push := func(s SeatID, c Card) { piles[s] = append(piles[s], c) }

	// firstOpponent returns the first seat clockwise from cur (excluding cur)
	// whose top card is the predecessor of c ("+1" fit, R-4.3/R-4.6).
	firstOpponent := func(c Card, cur int) (SeatID, bool) {
		for k := 1; k < n; k++ {
			s := seats[(cur+k)%n]
			if t, ok := topOf(s); ok && rs.Successor(t.Rank) == c.Rank {
				return s, true
			}
		}
		return 0, false
	}

	// R-4.7: the shuffler takes the first card onto its own pile.
	push(seats[0], draw())

	for cur := 1 % n; remaining() > 0; cur = (cur + 1) % n {
		curSeat := seats[cur]

		// R-4.4.1: unload own pile onto opponents while the top fits.
		for {
			t, ok := topOf(curSeat)
			if !ok {
				break
			}
			opp, found := firstOpponent(t, cur)
			if !found {
				break
			}
			pop(curSeat)
			push(opp, t)
		}

		// R-4.4.2/3/4 + R-4.9: draw until a card lands on the current player.
		for {
			c := draw()
			if remaining() == 0 {
				push(curSeat, c) // R-4.9: last card always to the drawer
				break
			}
			if opp, found := firstOpponent(c, cur); found {
				push(opp, c) // R-4.4.3: to opponent, then draw again
				continue
			}
			push(curSeat, c) // R-4.4.4: terminal, turn ends
			break
		}
	}

	return piles
}

// NewGame validates the config and deck, runs the automatic §4 deal, and returns
// the starting state plus a GameStarted event. deck must be exactly the
// NewDeck(cfg.Rules) multiset (any order — shuffling is the caller's job via the
// shuffle package, D-11). The shuffler is seat 0 (R-4.7). Turn is set to the
// holder of 9♦, who opens the first con (R-5.1).
func NewGame(cfg Config, deck []Card) (State, []Event, error) {
	if err := cfg.Validate(); err != nil {
		return State{}, nil, err
	}
	rs := cfg.Rules
	if err := validateDeck(rs, deck); err != nil {
		return State{}, nil, err
	}

	n := len(cfg.Players)
	seats := make([]SeatID, n)
	for i := range seats {
		seats[i] = SeatID(i)
	}

	hands := dealAll(rs, seats, deck)

	turn, ok := opener(hands)
	if !ok {
		// 9♦ is in every deck size and I-1 puts it in exactly one hand; a miss
		// means a dealing bug.
		return State{}, nil, fmt.Errorf("engine: 9♦ was not dealt to any seat (internal invariant violated)")
	}

	live := make(map[SeatID]bool, n)
	for _, s := range seats {
		live[s] = true
	}

	st := State{
		Rules: rs,
		Mode:  cfg.Mode,
		Seats: seats,
		Phase: Playing,
		Talon: nil,
		Hands: hands,
		Shukh: make(map[SeatID][]Card, n),
		Turn:  turn,
		Live:  live,
	}
	return st, []Event{GameStarted{Turn: turn}}, nil
}

// validateDeck reports whether deck is exactly the full deck for rs: the right
// size, no foreign cards, no duplicates (I-1 precondition for NewGame).
func validateDeck(rs RuleSet, deck []Card) error {
	if n := rs.deckCount(); len(deck) != n {
		return fmt.Errorf("engine: deck has %d cards, want %d for a %d-card game", len(deck), n, rs.DeckSize)
	}
	seen := make(map[Card]bool, len(deck))
	for _, c := range deck {
		if !rs.inDeck(c) {
			return fmt.Errorf("engine: deck contains %v, not part of a %d-card deck", c, rs.DeckSize)
		}
		if seen[c] {
			return fmt.Errorf("engine: deck contains duplicate %v", c)
		}
		seen[c] = true
	}
	return nil
}

// opener returns the seat holding 9♦, who opens the first con (R-5.1). With I-1
// holding, exactly one hand contains it.
func opener(hands map[SeatID][]Card) (SeatID, bool) {
	for s, h := range hands {
		if slices.ContainsFunc(h, IsStarter) {
			return s, true
		}
	}
	return 0, false
}
