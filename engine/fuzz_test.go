package engine_test

import (
	"math/rand/v2"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"
	"github.com/stretchr/testify/require"
)

// TestFuzzGamesTerminate plays N seeded random *legal* Guard games to the end,
// asserting I-1/I-6/beat-stack after every Apply and a valid final ranking
// (§14.6). Each game is bounded by a generous step budget; exceeding it is a bug
// (a non-terminating lifecycle), not an expected outcome.
func TestFuzzGamesTerminate(t *testing.T) {
	const (
		games    = 200
		maxSteps = 5000
	)
	deckSizes := []int{engine.Deck36, engine.Deck52}
	playerCounts := []int{2, 3, 4, 6}

	for g := 0; g < games; g++ {
		seed := int64(g + 1)
		rng := rand.New(rand.NewPCG(uint64(seed), 0x5c0ffee))
		rs := engine.RuleSet{DeckSize: deckSizes[g%len(deckSizes)]}
		np := playerCounts[g%len(playerCounts)]

		players := make([]engine.Player, np)
		for i := range players {
			players[i] = engine.Player{Name: string(rune('A' + i))}
		}
		cfg := engine.Config{Rules: rs, Mode: engine.Guard, Players: players}
		deck := shuffle.Deck(engine.NewDeck(rs), seed)

		s, _, err := engine.NewGame(cfg, deck)
		require.NoErrorf(t, err, "seed %d: NewGame", seed)
		require.NoErrorf(t, engine.CheckInvariants(s), "seed %d: invariants after deal", seed)

		steps := 0
		for s.Phase != engine.Finished {
			require.Lessf(t, steps, maxSteps, "seed %d: game did not terminate in %d steps", seed, maxSteps)

			// The acting seat is s.Turn only when no gate is open; a §8 payment gate
			// (e.g. Ш-11 via AskCount, which — unlike Ш-2/Ш-3 — can open in Guard
			// without moving Turn) hands the mic to the head payer instead (P-3).
			actor := s.Turn
			if s.Pending != nil && len(s.Pending.Owed) > 0 {
				actor = s.Pending.Owed[0]
			}
			legal := engine.LegalActions(s, actor)
			require.NotEmptyf(t, legal, "seed %d step %d: seat %d has no legal action", seed, steps, actor)

			a := legal[rng.IntN(len(legal))]
			ns, _, err := engine.Apply(s, a)
			require.NoErrorf(t, err, "seed %d step %d: Apply(%T)", seed, steps, a)
			require.NoErrorf(t, engine.CheckInvariants(ns), "seed %d step %d: invariants after %T", seed, steps, a)
			s = ns
			steps++
		}

		// Terminal ranking is a full permutation of all seats, exactly one loser
		// (the last place).
		require.Lenf(t, s.Finish, np, "seed %d: Finish is not a full ranking", seed)
		seen := make(map[engine.SeatID]bool, np)
		for _, seat := range s.Finish {
			require.Falsef(t, seen[seat], "seed %d: seat %d appears twice in Finish", seed, seat)
			seen[seat] = true
		}
		live := 0
		for i := 0; i < np; i++ {
			if s.Live[engine.SeatID(i)] {
				live++
			}
		}
		require.LessOrEqualf(t, live, 1, "seed %d: more than one live seat at end", seed)
	}
}
