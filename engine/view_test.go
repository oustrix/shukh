package engine_test

import (
	"slices"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"

	"github.com/stretchr/testify/require"
)

// viewGame builds a fresh 3‑player Deck36 game for View tests.
func viewGame(t *testing.T) engine.State {
	t.Helper()
	rs := engine.RuleSet{DeckSize: engine.Deck36}
	cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: players(3)}
	st, _, err := engine.NewGame(cfg, shuffle.Deck(engine.NewDeck(rs), 12345))
	require.NoError(t, err)
	return st
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

func TestViewPublicZones(t *testing.T) {
	st := viewGame(t)
	opener := st.Turn

	// Opener plays one card → the con becomes non‑empty (заход, R‑5.2).
	acts := engine.LegalActions(st, opener)
	require.NotEmpty(t, acts)
	play, ok := acts[0].(engine.PlayCard)
	require.True(t, ok, "first legal action of the opener is a PlayCard")
	st, _, err := engine.Apply(st, play) // Apply reads the turn from state (no seat arg)
	require.NoError(t, err)

	v := engine.View(st, opener)

	// The con is public: same cards, same order.
	require.Equal(t, st.Table, v.Table)
	require.Len(t, v.Table, 1)
	require.Equal(t, play.Card, v.Table[0].Card)

	// Discard and talon are sizes only.
	require.Equal(t, len(st.Discard), v.Discard)
	require.Equal(t, len(st.Talon), v.Talon)
	require.Equal(t, 0, v.Talon, "talon is empty after dealing (R‑4.10)")

	// Finish mirrors state (empty this early).
	require.Equal(t, st.Finish, v.Finish)
}

func TestViewDoesNotLeakOrAlias(t *testing.T) {
	st := viewGame(t)
	seat := st.Turn

	// Play one card so the con is non-empty — this lets us exercise the Table copy.
	acts := engine.LegalActions(st, seat)
	require.NotEmpty(t, acts)
	play, ok := acts[0].(engine.PlayCard)
	require.True(t, ok, "opener's first legal action is a PlayCard")
	st, _, err := engine.Apply(st, play)
	require.NoError(t, err)
	require.NotEmpty(t, st.Table, "con is non-empty after the заход")

	v := engine.View(st, seat) // the opener is still live and holds cards

	// Card conservation, computed before any mutation: own hand + opponent counts
	// equals the sum of all hand sizes in state.
	total := len(v.Hand)
	for _, o := range v.Opponents {
		total += o.HandCount
	}
	sum := 0
	for _, s := range st.Seats {
		sum += len(st.Hands[s])
	}
	require.Equal(t, sum, total, "per-seat visible + opponent counts conserve cards")

	// Snapshot the state we will try to corrupt through the returned view.
	wantHand := slices.Clone(st.Hands[seat])
	wantTable := slices.Clone(st.Table)
	wantLive := st.Live[seat]
	require.NotEmpty(t, v.Hand, "opener still holds cards after one play")

	// Mutate existing elements of the returned copies (not appends).
	v.Hand[0] = engine.Card{Suit: engine.Diamonds, Rank: 2}
	v.Table[0] = engine.TableCard{}
	v.Live[seat] = !v.Live[seat]

	// State is untouched: the returned slices/maps were copies.
	require.Equal(t, wantHand, st.Hands[seat], "Hand mutation did not touch state")
	require.Equal(t, wantTable, st.Table, "Table element mutation did not touch state")
	require.Equal(t, wantLive, st.Live[seat], "Live mutation did not touch state")

	// A fresh view reflects the pristine state, not the mutated copies.
	v2 := engine.View(st, seat)
	require.ElementsMatch(t, wantHand, v2.Hand)
	require.Equal(t, wantTable, v2.Table)
	require.Equal(t, wantLive, v2.Live[seat])

	// View did not mutate the input state.
	require.NoError(t, engine.CheckInvariants(st))
}

func TestViewDeterministic(t *testing.T) {
	st := viewGame(t)
	for _, seat := range st.Seats {
		a := engine.View(st, seat)
		b := engine.View(st, seat)
		require.Equal(t, a, b, "View is a pure function of (state, seat)")
	}
}

func TestViewWorksForEverySeatOffTurn(t *testing.T) {
	st := viewGame(t)
	for _, seat := range st.Seats {
		v := engine.View(st, seat)
		require.Equal(t, seat, v.You)
		require.ElementsMatch(t, st.Hands[seat], v.Hand, "each seat sees its own hand regardless of turn")
		require.Len(t, v.Opponents, len(st.Seats)-1)
	}
}
