package engine

// CanBeat reports whether card c may legally beat the current top card of the
// con, per §3. It is rule-set independent: ranks are absolute face values and
// the suit matrix is identical for both deck sizes.
//
//	♠ (пика)   — only a higher ♠; trump does NOT beat it (R-3.3, I-7)
//	♦ (козырь) — only a higher ♦ (R-3.1)
//	♥ / ♣      — a higher card of the same suit OR any ♦ (R-3.1, R-3.2)
//	Дама ♥     — beats any card (R-3.7.1)
//
// CanBeat assumes top is a legitimate beatable top. Дама ♥ never persists as a
// top card because it immediately closes the con (R-3.7.1); the con lifecycle
// (a later iteration) guarantees this.
func CanBeat(top, c Card) bool {
	if IsQueenHearts(c) {
		return true // R-3.7.1 — highest card, beats anything
	}
	switch top.Suit {
	case Spades:
		return c.Suit == Spades && c.Rank > top.Rank // R-3.3, I-7
	case Diamonds:
		return c.Suit == Diamonds && c.Rank > top.Rank // R-3.1
	default: // Hearts or Clubs
		if c.Suit == Diamonds {
			return true // R-3.2 — trump of any rank beats a non-spade non-trump
		}
		return c.Suit == top.Suit && c.Rank > top.Rank // R-3.1
	}
}
