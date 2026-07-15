// Package engine is the pure, deterministic rules core of the card game «Шух»
// (Layer 0). It contains no I/O, networking, time, or randomness — determinism
// and transport live at higher layers (see docs/architecture.md, D-6/D-7).
package engine

import "strconv"

// Suit is a card suit. Diamonds (♦) is always trump (R-2.5).
type Suit uint8

const (
	Spades   Suit = iota // ♠ пики
	Hearts               // ♥ черви
	Diamonds             // ♦ бубны — козырь (R-2.5)
	Clubs                // ♣ трефы
)

// IsTrump reports whether the suit is the trump suit ♦ (R-2.5).
func (s Suit) IsTrump() bool { return s == Diamonds }

// String renders the suit as its Unicode symbol: ♠ ♥ ♦ ♣ (R-2.4).
func (s Suit) String() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		return "?"
	}
}

// Rank is a card nominal as an absolute face value 2..14 (R-2.2, R-2.3).
// Jack=11, Queen=12, King=13, Ace=14. Which ranks actually exist depends on the
// deck size (RuleSet.LowestRank), but the face values themselves never change.
type Rank uint8

const (
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

// String renders the rank as J/Q/K/A for the face cards, or its decimal value.
func (r Rank) String() string {
	switch r {
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return strconv.Itoa(int(r))
	}
}

// Card is a single playing card.
type Card struct {
	Suit Suit
	Rank Rank
}

// String renders the card as rank followed by suit symbol, e.g. "9♦".
func (c Card) String() string { return c.Rank.String() + c.Suit.String() }
