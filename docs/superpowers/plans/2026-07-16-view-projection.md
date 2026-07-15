# View per-seat projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `engine.View(state, seat)` — the last missing Layer‑0 public API method — a pure per‑seat projection of game state that structurally cannot leak hidden cards.

**Architecture:** New `engine/view.go` defines value types `View` and `OpponentView` plus a pure function `View(s State, seat SeatID) View`. It follows the existing engine convention: `State` by value, no panics, no I/O. Own hand is exposed in full; opponents are counts only (no card field exists); public zones (con, discard size, talon size) and outcome (live/finish) are copied out so callers cannot mutate `State` internals. Reuses the existing unexported helper `seatsFrom` for clockwise opponent ordering.

**Tech Stack:** Go 1.26, `github.com/stretchr/testify/require` for tests, `slices` stdlib for copies.

## Global Constraints

- Layer 0 purity (D‑6/D‑7): `engine` package has **no I/O, no networking, no time, no randomness, no panics**. Copy verbatim from `engine/card.go` package doc.
- Visibility semantics are **D‑9** (see spec §2). Own hand full; opponents' cards hidden, counts public; con public; discard closed (size only, R‑2.9); ШУХ pile face down, only the "awaiting" counter public (I‑3).
- Signature convention matches `LegalActions(s State, seat SeatID)`: `State` **by value**, no `error` return.
- Base branch `worktree-engine-view` is off `main` (364edfa): `State` has fields `Rules, Mode, Seats, Phase, Talon, Hands, Table, Discard, Shukh, Turn, Live, Finish` and **not** the iteration‑4 fields (`OwesOneCard`, `ShukhTakeable`, `Pending`, `Unsettled`, `Endgame`). Do **not** reference iteration‑4 fields.
- Tests live in `package engine_test` (external), build state via `engine.NewGame(cfg, deck)` with `shuffle.Deck(engine.NewDeck(rs), seed)`, exactly like `engine/newgame_test.go`.
- Commit message trailer on every commit:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- **Create `engine/view.go`** — `View`, `OpponentView` types + `View(s State, seat SeatID) View`. One responsibility: read‑only per‑seat projection. Does not import anything beyond `slices` (stdlib).
- **Create `engine/view_test.go`** — `package engine_test`; all View tests.

Reused existing (unexported, same package — accessible from `view.go`):
- `func (s State) seatsFrom(pivot SeatID) []SeatID` (`engine/apply.go:212`) — all seats clockwise from `pivot` inclusive. Opponents = `s.seatsFrom(seat)[1:]`.
- `func (s State) clone() State` (`engine/apply.go:86`) — deep copy, used in tests to prove `View` does not mutate input.

Existing types consumed (all in `package engine`): `State`, `SeatID`, `Card`, `TableCard`, `RuleSet`, `EnforcementMode`, `Phase`.

---

### Task 1: `View`/`OpponentView` types + full projection

**Files:**
- Create: `engine/view.go`
- Test: `engine/view_test.go`

**Interfaces:**
- Consumes: `engine.State`, `engine.SeatID`, `engine.Card`, `engine.TableCard`, `engine.RuleSet`, `engine.EnforcementMode`, `engine.Phase`; unexported `State.seatsFrom`.
- Produces:
  - `type SeatView struct { Rules RuleSet; Mode EnforcementMode; Phase Phase; You SeatID; Turn SeatID; Hand []Card; ShukhPending int; Opponents []OpponentView; Table []TableCard; Discard int; Talon int; Live map[SeatID]bool; Finish []SeatID }`
  - `type OpponentView struct { Seat SeatID; HandCount int; ShukhPending int; Live bool }`
  - `func View(s State, seat SeatID) SeatView`
  - **Naming:** Go forbids a type and function sharing a name in one package, so the
    projection type is `SeatView` and the function stays `View` (keeps the four API
    verbs `NewGame / LegalActions / Apply / View` parallel).

- [ ] **Step 1: Write the failing test**

Create `engine/view_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestViewProjectsSelfAndOpponents -v`
Expected: FAIL — `undefined: engine.View` (does not compile).

- [ ] **Step 3: Write minimal implementation**

Create `engine/view.go`:

```go
package engine

import (
	"maps"
	"slices"
)

// SeatView is the per‑seat projection of game state (D‑9): exactly what one
// player may see. Opponents' hands are represented by counts only — there is no
// card field on OpponentView, so a hidden card is structurally unrepresentable.
type SeatView struct {
	Rules RuleSet
	Mode  EnforcementMode
	Phase Phase // Playing | Finished

	You  SeatID // whose view this is
	Turn SeatID // whose turn it is (public)

	Hand         []Card // own hand, in full (owner sees it)
	ShukhPending int    // own set‑aside ШУХ cards awaiting pickup (I‑3)

	// Opponents, clockwise starting at the seat after You (R‑2.13); self excluded.
	Opponents []OpponentView

	Table   []TableCard // the con, bottom→top (cards and order are public)
	Discard int         // size of the closed discard pile (contents hidden, R‑2.9)
	Talon   int         // undealt deck remaining (0 after dealing; field is general)

	Live   map[SeatID]bool // who is still in the game
	Finish []SeatID        // finishing order → places (R‑9.2)
}

// OpponentView is the public projection of one other seat: counts only.
type OpponentView struct {
	Seat         SeatID
	HandCount    int  // number of cards in hand (public)
	ShukhPending int  // number of awaiting ШУХ cards (public, I‑3)
	Live         bool
}

// View builds the projection of s for seat (D‑9). It is pure and does not mutate
// s. Unlike LegalActions it is not turn‑gated: every valid seat can always see
// its own hand and the public state. State is taken by value, matching
// LegalActions. Precondition: seat is one of s.Seats (Layer 1 guarantees this);
// an unknown seat yields a well‑formed but meaningless SeatView (empty own hand).
func View(s State, seat SeatID) SeatView {
	opps := s.seatsFrom(seat) // clockwise from seat, inclusive
	v := SeatView{
		Rules:        s.Rules,
		Mode:         s.Mode,
		Phase:        s.Phase,
		You:          seat,
		Turn:         s.Turn,
		Hand:         slices.Clone(s.Hands[seat]),
		ShukhPending: len(s.Shukh[seat]),
		Table:        slices.Clone(s.Table),
		Discard:      len(s.Discard),
		Talon:        len(s.Talon),
		Live:         make(map[SeatID]bool, len(s.Live)),
		Finish:       slices.Clone(s.Finish),
	}
	for _, k := range opps[1:] { // skip seat itself
		v.Opponents = append(v.Opponents, OpponentView{
			Seat:         k,
			HandCount:    len(s.Hands[k]),
			ShukhPending: len(s.Shukh[k]),
			Live:         s.Live[k],
		})
	}
	maps.Copy(v.Live, s.Live)
	return v
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestViewProjectsSelfAndOpponents -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/view.go engine/view_test.go
git commit -m "feat(engine): View per-seat projection — self + opponents (D-9)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Public zones — con, discard, talon, finish

**Files:**
- Modify: `engine/view_test.go` (add test; no change to `view.go` — Task 1 already fills these)

**Interfaces:**
- Consumes: `engine.View` from Task 1.
- Produces: nothing new (regression coverage for the public‑zone fields).

This task proves the con/discard/talon/finish projection after some play, exercising a non‑empty con and a non‑empty discard rather than the empty start state.

- [ ] **Step 1: Write the failing test**

Add to `engine/view_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails, then passes**

Run: `go test ./engine/ -run TestViewPublicZones -v`
Expected: PASS immediately (Task 1 implemented these fields). If it FAILS, fix `view.go` to match the field semantics above before continuing. (This test is a regression guard; a green result on first run is correct here — the deliverable is the coverage, not new production code.)

- [ ] **Step 3: Commit**

```bash
git add engine/view_test.go
git commit -m "test(engine): View public zones — con, discard, talon, finish

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Safety guarantees — no leak, immutability, determinism

**Files:**
- Modify: `engine/view_test.go` (add tests)
- Possibly modify: `engine/view.go` (only if an immutability test exposes a shared backing slice/map)

**Interfaces:**
- Consumes: `engine.View`, `engine.OpponentView`, unexported `State.clone` (same package is not accessible from `engine_test`; use a public deep‑equality check instead — see below).
- Produces: nothing new.

Note: `clone` is unexported, so `engine_test` cannot call it. Prove "View does not mutate input" by snapshotting the observable public state before/after and via `engine.CheckInvariants`, and prove "returned slices are copies" by mutating them and re‑reading a fresh `View`.

- [ ] **Step 1: Write the failing test**

Add to `engine/view_test.go`:

```go
func TestViewDoesNotLeakOrAlias(t *testing.T) {
	st := viewGame(t)
	seat := st.Turn

	// Total cards across all hands are conserved: own hand (visible) + opponents'
	// counts equals the sum of all hand sizes in state.
	v := engine.View(st, seat)
	total := len(v.Hand)
	for _, o := range v.Opponents {
		total += o.HandCount
	}
	sum := 0
	for _, s := range st.Seats {
		sum += len(st.Hands[s])
	}
	require.Equal(t, sum, total, "per‑seat visible + opponent counts conserve cards")

	// Returned Hand is a copy: mutating it must not change the next View.
	if len(v.Hand) > 0 {
		v.Hand[0] = engine.Card{Suit: engine.Diamonds, Rank: 2}
	}
	v.Table = append(v.Table, engine.TableCard{})
	v.Live[seat] = !v.Live[seat]

	v2 := engine.View(st, seat)
	require.ElementsMatch(t, st.Hands[seat], v2.Hand, "Hand mutation did not touch state")
	require.Equal(t, st.Live[seat], v2.Live[seat], "Live mutation did not touch state")
	require.Len(t, v2.Table, len(st.Table), "Table append did not touch state")

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
```

- [ ] **Step 2: Run tests to verify**

Run: `go test ./engine/ -run TestView -v`
Expected: PASS. If `TestViewDoesNotLeakOrAlias` FAILS on the alias assertions, the culprit is a missing copy in `view.go` — ensure `Hand`, `Table`, `Finish` use `slices.Clone` and `Live` is a fresh map (as written in Task 1), then re‑run.

- [ ] **Step 3: Full suite + vet**

Run: `go vet ./... && go test ./...`
Expected: PASS — `ok github.com/oustrix/shukh/engine`, `ok github.com/oustrix/shukh/shuffle`.

- [ ] **Step 4: Commit**

```bash
git add engine/view_test.go engine/view.go
git commit -m "test(engine): View safety — no leak, immutable output, determinism

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Wire View into the architecture changelog

**Files:**
- Modify: `docs/architecture.md` (changelog §6 only — one bullet)

**Interfaces:**
- Consumes / Produces: none (documentation).

- [ ] **Step 1: Append a changelog bullet**

Add to the end of `docs/architecture.md` §6 «Журнал изменений» (keep the existing date grouping style):

```markdown
- **2026-07-16.** Реализован `View(state, seat)` — последний метод публичного API Слоя 0
  (D‑9): per‑seat проекция, скрытые карты структурно непредставимы (соперники — только
  счётчики). Отдельный worktree, независимо от итерации 4.
```

- [ ] **Step 2: Verify build still green**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add docs/architecture.md
git commit -m "docs(architecture): record View(state, seat) — Layer 0 API complete (D-9)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage (spec §2–§6):**
- §2 visibility table → Task 1 (self hand, opponent counts, ШУХ counters), Task 2 (con public, discard size, talon size), Task 1 (config/turn/phase/live/finish public). ✅
- §3 data shape (variant A, counts‑only opponents) → Task 1 types verbatim. ✅
- §4 contract: `State` by value, no error, not turn‑gated, copies out, no panic → Task 1 signature + Task 3 `TestViewWorksForEverySeatOffTurn`, `TestViewDoesNotLeakOrAlias`, `TestViewDeterministic`. ✅
- §5 tests 1–10 → distributed: 1 self, 2 no‑leak, 3 order, 4 con, 5 discard, 6 ШУХ, 7 talon, 8 immutability, 9 conservation, 10 determinism — all present across Tasks 1–3. ✅
- §6 files: `engine/view.go`, `engine/view_test.go`, architecture changelog → Tasks 1/3/4. ✅

**Placeholder scan:** No TBD/TODO; all code shown in full; commands have expected output. ✅

**Type consistency:** `View`/`OpponentView` field names identical across Task 1 definition and all test references (`Hand`, `ShukhPending`, `Opponents`, `Table`, `Discard`, `Talon`, `Live`, `Finish`, `Seat`, `HandCount`, `Live`). Signature `View(s State, seat SeatID) View` consistent everywhere. Helper `seatsFrom` matches `engine/apply.go:212`. ✅
