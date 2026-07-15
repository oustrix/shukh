package engine_test

import (
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"

	"github.com/stretchr/testify/require"
)

// viewGame builds a fresh 3‑player Deck36 game for View tests.
func viewGame(t *testing.T) engine.State {
	t.Helper()
	rs := engine.RuleSet{DeckSize: engine.Deck36}
	cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: viewPlayers(3)}
	st, _, err := engine.NewGame(cfg, shuffle.Deck(engine.NewDeck(rs), 12345))
	require.NoError(t, err)
	return st
}

func viewPlayers(n int) []engine.Player {
	ps := make([]engine.Player, n)
	for i := range ps {
		ps[i] = engine.Player{Name: "p"}
	}
	return ps
}

func TestViewProjectsSelfAndOpponents(t *testing.T) {
	st := viewGame(t)
	seat := st.Turn

	v := engine.View(st, seat)

	// Config + identity are public.
	require.Equal(t, st.Rules, v.Rules)
	require.Equal(t, st.Mode, v.Mode)
	require.Equal(t, st.Phase, v.Phase)
	require.Equal(t, seat, v.You)
	require.Equal(t, st.Turn, v.Turn)

	// Own hand is visible in full (as a set).
	require.ElementsMatch(t, st.Hands[seat], v.Hand)
	require.Equal(t, len(st.Shukh[seat]), v.ShukhPending)

	// Opponents: clockwise starting after `seat`, self excluded.
	require.Len(t, v.Opponents, len(st.Seats)-1)
	require.Equal(t, engine.SeatID((int(seat)+1)%len(st.Seats)), v.Opponents[0].Seat)
	for _, o := range v.Opponents {
		require.NotEqual(t, seat, o.Seat, "self is not an opponent")
		require.Equal(t, len(st.Hands[o.Seat]), o.HandCount)
		require.Equal(t, len(st.Shukh[o.Seat]), o.ShukhPending)
		require.Equal(t, st.Live[o.Seat], o.Live)
	}
}
