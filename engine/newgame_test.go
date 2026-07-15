package engine_test

import (
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"

	"github.com/stretchr/testify/require"
)

func players(n int) []engine.Player {
	ps := make([]engine.Player, n)
	for i := range ps {
		ps[i] = engine.Player{Name: "p"}
	}
	return ps
}

func TestNewGameBuildsStartingState(t *testing.T) {
	rs := engine.RuleSet{DeckSize: engine.Deck36}
	cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: players(3)}
	deck := shuffle.Deck(engine.NewDeck(rs), 12345)

	st, ev, err := engine.NewGame(cfg, deck)
	require.NoError(t, err)
	require.NoError(t, engine.CheckInvariants(st))
	require.Equal(t, engine.Playing, st.Phase)
	require.Empty(t, st.Talon)
	require.Empty(t, st.Table)
	require.Empty(t, st.Discard)
	require.Len(t, st.Seats, 3)
	for _, s := range st.Seats {
		require.True(t, st.Live[s])
	}
	require.Len(t, ev, 1)
	require.Equal(t, engine.GameStarted{Turn: st.Turn}, ev[0])
	require.Contains(t, st.Hands[st.Turn], engine.Card{Suit: engine.Diamonds, Rank: 9}, "opener holds 9♦ (R-5.1)")
}

func TestNewGameDeterministic(t *testing.T) {
	rs := engine.RuleSet{DeckSize: engine.Deck52}
	cfg := engine.Config{Rules: rs, Mode: engine.Guard, Players: players(4)}
	deck := engine.NewDeck(rs) // same ordered deck twice
	a, _, err := engine.NewGame(cfg, deck)
	require.NoError(t, err)
	b, _, err := engine.NewGame(cfg, deck)
	require.NoError(t, err)
	require.Equal(t, a.Hands, b.Hands, "NewGame is a pure function of (cfg, deck)")
	require.Equal(t, a.Turn, b.Turn)
}

func TestNewGameRejectsBadConfig(t *testing.T) {
	rs := engine.RuleSet{DeckSize: engine.Deck36}
	deck := engine.NewDeck(rs)
	for _, cfg := range []engine.Config{
		{Rules: rs, Mode: engine.Middle, Players: players(1)},
		{Rules: rs, Mode: engine.Middle, Players: players(9)},
		{Rules: rs, Mode: engine.Culture, Players: players(2)},
	} {
		_, _, err := engine.NewGame(cfg, deck)
		require.Error(t, err)
	}
}

func TestNewGameRejectsBadDeck(t *testing.T) {
	rs := engine.RuleSet{DeckSize: engine.Deck36}
	cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: players(2)}
	full := engine.NewDeck(rs)

	// too short
	_, _, err := engine.NewGame(cfg, full[:len(full)-1])
	require.Error(t, err)

	// right length but a duplicate replaces a distinct card
	dup := append([]engine.Card(nil), full...)
	dup[0] = dup[1]
	_, _, err = engine.NewGame(cfg, dup)
	require.Error(t, err)

	// foreign card (2♥ not in a 36-card deck)
	foreign := append([]engine.Card(nil), full...)
	foreign[0] = engine.Card{Suit: engine.Hearts, Rank: 2}
	_, _, err = engine.NewGame(cfg, foreign)
	require.Error(t, err)
}

func TestNewGameFuzzDealingInvariants(t *testing.T) {
	for _, ds := range []int{engine.Deck36, engine.Deck52} {
		rs := engine.RuleSet{DeckSize: ds}
		full := engine.NewDeck(rs)
		for p := 2; p <= 8; p++ {
			cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: players(p)}
			for seed := int64(0); seed < 200; seed++ {
				deck := shuffle.Deck(full, seed)
				st, ev, err := engine.NewGame(cfg, deck)
				require.NoError(t, err)
				require.NoError(t, engine.CheckInvariants(st))
				require.Len(t, ev, 1)
				require.Empty(t, st.Talon)

				total := 0
				seen := map[engine.Card]bool{}
				for _, h := range st.Hands {
					for _, c := range h {
						require.False(t, seen[c], "card %v dealt twice (deck %d, %d players, seed %d)", c, ds, p, seed)
						seen[c] = true
						total++
					}
				}
				require.Equal(t, ds, total)
				require.Contains(t, st.Hands[st.Turn], engine.Card{Suit: engine.Diamonds, Rank: 9})
			}
		}
	}
}
