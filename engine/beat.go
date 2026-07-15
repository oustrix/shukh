package engine

// CanBeat reports whether card c may legally beat the current top card of the
// con, per §3. It is rule-set independent: ranks are absolute face values and
// the suit matrix is identical for both deck sizes.
//
//	♠ (пика)   — only a higher ♠; trump does NOT beat it (R-3.3, I-7)
//	♦ (козырь) — only a higher ♦ (R-3.1)
//	♥ / ♣      — a higher card of the same suit OR any ♦ (R-3.1, R-3.2)
//	Дама ♥     — beats any card (R-3.7.1); nothing beats Дама ♥ as top (R-3.7)
//
// Дама ♥ tops a stable con only transiently, during an open Middle Ш-2 catch-
// window (§15.3) — in Guard it closes the con the instant it is played (R-3.7.1).
// Either way nothing may beat it, so the next player is forced to take it.
func CanBeat(top, c Card) bool {
	if IsQueenHearts(c) {
		return true // R-3.7.1 — highest card, beats anything
	}
	if IsQueenHearts(top) {
		return false // R-3.7 — nothing beats Дама♥ (it only tops a con during a Ш-2 window)
	}
	switch top.Suit {
	case Spades:
		return c.Suit == Spades && c.Rank > top.Rank // R-3.3, I-7
	case Diamonds:
		return c.Suit.IsTrump() && c.Rank > top.Rank // R-3.1
	default: // Hearts or Clubs
		if c.Suit.IsTrump() {
			return true // R-3.2 — trump of any rank beats a non-spade non-trump
		}
		return c.Suit == top.Suit && c.Rank > top.Rank // R-3.1
	}
}
