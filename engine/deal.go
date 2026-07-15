package engine

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
