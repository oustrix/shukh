// Package shuffle produces a deterministic, seeded permutation of a deck. It is
// the randomness boundary of the game (decision D-11): the pure engine (Layer 0)
// takes an already-ordered []Card, and this package — which may import
// math/rand/v2 — is where the seed becomes an order. It imports engine, never the
// reverse.
package shuffle

import (
	"math/rand/v2"

	"github.com/oustrix/shukh/engine"
)

// Deck returns a shuffled copy of cards using a Fisher–Yates shuffle seeded by
// seed. The result is deterministic for a given (cards, seed) pair within a Go
// version; the input slice is not mutated. Cross-Go-version stability of the
// stream is not guaranteed (math/rand/v2) — reproducibility of a specific game is
// carried by the resulting []Card, not the seed, at higher layers.
func Deck(cards []engine.Card, seed int64) []engine.Card {
	out := make([]engine.Card, len(cards))
	copy(out, cards)
	r := rand.New(rand.NewPCG(uint64(seed), 0))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
