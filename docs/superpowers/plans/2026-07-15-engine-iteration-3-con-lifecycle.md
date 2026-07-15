# Engine Iteration 3 — Con Lifecycle §5 + Exit §9.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the pure engine play a full con lifecycle — заход, бой, взять, западло, закрытие (счёт + Дама♥), отбой, очередь, выход — in Guard mode, driven by `LegalActions`/`Apply`, until one player remains (a valid `Finish`).

**Architecture:** `Apply(State, Action)` is a pure copy-on-write reducer that validates the action against `LegalActions` (the executable spec), mutates a clone, and runs a deterministic `resolve` pipeline (place card → maybe close+sweep → exits → set opener → advance turn). Table cards carry an owner (`TableCard{Card, By}`) so exit (R-9.1/R-5.9) and take-bottom (R-5.8) are decidable. The Guard lone-`Дама♥` opener deadlock is resolved by skipping that turn (§14.4). `CheckInvariants` gains I-6 + a beat-stack oracle; a seeded fuzz test plays random legal games to termination.

**Tech Stack:** Go 1.26.1, standard library (`slices`, `fmt`), `github.com/stretchr/testify/require`, `math/rand/v2` (test-only, via the `shuffle` package). No new dependencies.

## Global Constraints

- **Authoritative spec:** `docs/superpowers/specs/2026-07-15-engine-core-design.md` §14 (iteration-3 decisions). Rules of truth: `docs/shukh-rules.md` (`R-§.n`, `Ш-n`, `I-n`).
- **Layer 0 purity (D-6/D-7):** `engine` imports no I/O, networking, time, or `math/rand`. Only the fuzz *test* uses randomness, via `package engine_test` importing `shuffle` (avoids the `shuffle`→`engine` import cycle).
- **Mode:** iteration 3 is **Guard** only. No ШУХ (§7–§8), no «одна карта» (§6), no endgame §9.2, no Middle `Unsettled`, no `View`.
- **`Apply` is pure:** never mutates its input; returns a new `State` + `[]Event`, or a typed `*IllegalAction`. No `panic` on any input.
- **Turn model:** actions apply as `s.Turn` (no seat field on turn-actions). `Apply` rejects anything not in `LegalActions(s, s.Turn)`.
- **Existing style:** tests are white-box `package engine` using `testify/require`; doc-comment every exported symbol with its rule references; run tests with `go test ./engine/ -run <Name> -v`.
- **Test data model:** unit tests build focused states with the `playing(...)` helper (Task 2) and assert transition specifics; they do **not** require full I-1 (partial decks). Full I-1 coverage comes from the fuzz test (Task 11), which plays real dealt games.

---

## File Structure

**Modify:**
- `engine/state.go` — add `TableCard`; change `State.Table` to `[]TableCard`; remove `GameStarted` (moves to `event.go`).
- `engine/invariants.go` — count `TableCard.Card` for I-1; add I-6 + beat-stack oracle.
- `engine/invariants_test.go` — fix the foreign-card test to use `[]TableCard`.

**Create:**
- `engine/action.go` — `Action` sealed interface + `PlayCard`, `TakeBottomAndPass`, `PodkladkaWest`.
- `engine/event.go` — all `Event` types (moved `GameStarted` + new lifecycle events).
- `engine/legal.go` — `LegalActions`.
- `engine/apply.go` — `IllegalAction`, `Apply`, and the resolve helpers (`clone`, `liveCount`, `nextLive`, `seatsFrom`, `handInCon`, `forcedQueenSkip`, `settleTurn`, `resolveExits`, `closeCon`, `removeCard`).
- `engine/legal_test.go`, `engine/apply_test.go` — unit tests + the shared `playing(...)` helper.
- `engine/fuzz_test.go` — `package engine_test` seeded fuzz to termination.

---

### Task 1: TableCard model + I-1 counting

**Files:**
- Modify: `engine/state.go` (Table field + new type)
- Modify: `engine/invariants.go` (I-1 counts `tc.Card`)
- Test: `engine/invariants_test.go` (fix foreign-card test)

**Interfaces:**
- Produces: `type TableCard struct { Card Card; By SeatID }`; `State.Table []TableCard`.

- [ ] **Step 1: Change the failing test to the new Table type**

In `engine/invariants_test.go`, the existing `TestCheckInvariantsI1ForeignCard` sets `s.Table = []Card{{Hearts, 2}}`. Change it to:

```go
func TestCheckInvariantsI1ForeignCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Table = []TableCard{{Card: Card{Hearts, 2}}} // 2♥ is not in a 36-card deck
	require.Error(t, CheckInvariants(s))
}
```

- [ ] **Step 2: Run it to verify it fails to compile**

Run: `go test ./engine/ -run TestCheckInvariantsI1ForeignCard -v`
Expected: BUILD FAIL — `undefined: TableCard` / `cannot use ... as []Card`.

- [ ] **Step 3: Add the `TableCard` type and change the `Table` field**

In `engine/state.go`, add above the `State` struct:

```go
// TableCard is a card on the con together with the seat that played it. The
// owner is needed to decide exit (R-9.1/R-5.9) and to route "take the bottom"
// (R-5.8) — the con is physically a stack of specific players' cards.
type TableCard struct {
	Card Card
	By   SeatID
}
```

Then change the `Table` field of `State`:

```go
	Table   []TableCard       // the con, bottom→top; empty at start of game
```

- [ ] **Step 4: Count `TableCard.Card` in I-1**

In `engine/invariants.go`, replace `count(s.Table)` with a loop (the `count` helper takes `[]Card`):

```go
	count(s.Talon)
	for _, h := range s.Hands {
		count(h)
	}
	for _, tc := range s.Table {
		seen[tc.Card]++
	}
	count(s.Discard)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS (all existing tests still green; `TableCard` compiles).

- [ ] **Step 6: Commit**

```bash
git add engine/state.go engine/invariants.go engine/invariants_test.go
git commit -m "$(cat <<'EOF'
refactor(engine): Table carries card ownership (TableCard)

Change State.Table from []Card to []TableCard{Card, By} so exit (R-9.1/R-5.9)
and take-bottom (R-5.8) can be decided. I-1 now counts tc.Card.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Action types + `LegalActions`

**Files:**
- Create: `engine/action.go`
- Create: `engine/legal.go`
- Create: `engine/legal_test.go` (with the shared `playing` helper)

**Interfaces:**
- Consumes: `TableCard`, `CanBeat`, `IsQueenHearts`, `RuleSet.IsSecondLowestHeart`, `RuleSet.LowestRank`.
- Produces: `type Action interface{ isAction() }`; `PlayCard{Card Card}`, `TakeBottomAndPass{}`, `PodkladkaWest{}`; `func LegalActions(s State, seat SeatID) []Action`; test helper `func playing(hands map[SeatID][]Card, table []TableCard, turn SeatID) State`.

- [ ] **Step 1: Write the failing test**

Create `engine/legal_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// playing builds a minimal in-progress Guard state: seats 0..n-1 (all live),
// 36-card rules, given hands/table, and whose turn it is. It does NOT enforce
// I-1 — unit tests assert transition specifics on focused states.
func playing(hands map[SeatID][]Card, table []TableCard, turn SeatID) State {
	n := len(hands)
	seats := make([]SeatID, n)
	live := make(map[SeatID]bool, n)
	for i := 0; i < n; i++ {
		seats[i] = SeatID(i)
		live[SeatID(i)] = true
	}
	return State{
		Rules: RuleSet{DeckSize: Deck36},
		Mode:  Guard,
		Seats: seats,
		Phase: Playing,
		Hands: hands,
		Table: table,
		Shukh: map[SeatID][]Card{},
		Live:  live,
		Turn:  turn,
	}
}

func TestLegalActionsZahod(t *testing.T) {
	// Empty con → any card but Дама♥ is a legal заход (R-5.2).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Hearts, Queen}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 7}},
		PlayCard{Card{Diamonds, 9}},
	}, LegalActions(s, 0))
}

func TestLegalActionsBeatAndTake(t *testing.T) {
	// Top is 8♠; only a higher ♠ beats it (I-7). Take is always available.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Spades, 6}, {Diamonds, 14}},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 10}},
		TakeBottomAndPass{},
	}, LegalActions(s, 0))
}

func TestLegalActionsPodkladkaWest(t *testing.T) {
	// Bottom is 7♥ (7(3)♥ for a 36-deck) and hand has 6♥ (6(2)♥) → западло offered
	// alongside take (R-3.6.2/R-5.3c).
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 1}}, 0)
	require.ElementsMatch(t, []Action{
		TakeBottomAndPass{},
		PodkladkaWest{},
	}, LegalActions(s, 0))
}

func TestLegalActionsHandlessForcedTake(t *testing.T) {
	// Empty hand but a card hangs in the open con → only move is take (R-5.9).
	s := playing(map[SeatID][]Card{
		0: {},
		1: {{Clubs, 7}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 0}}, 0)
	require.Equal(t, []Action{TakeBottomAndPass{}}, LegalActions(s, 0))
}

func TestLegalActionsNotYourTurnOrFinished(t *testing.T) {
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	require.Nil(t, LegalActions(s, 1)) // not seat 1's turn
	s.Phase = Finished
	require.Nil(t, LegalActions(s, 0)) // game over
}

func TestLegalActionsLoneQueenIsEmpty(t *testing.T) {
	// Empty con, only card is Дама♥ → no legal заход (Guard blocks it, §14.4).
	s := playing(map[SeatID][]Card{0: {{Hearts, Queen}}, 1: {{Clubs, 8}}}, nil, 0)
	require.Empty(t, LegalActions(s, 0))
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestLegalActions -v`
Expected: BUILD FAIL — `undefined: Action`, `undefined: PlayCard`, `undefined: LegalActions`.

- [ ] **Step 3: Define the action types**

Create `engine/action.go`:

```go
package engine

// Action is a move a player can apply to the game via Apply. It is a sealed
// interface — only engine types implement it. Turn-actions (§5) apply as the
// player to move (State.Turn); they carry no seat.
type Action interface{ isAction() }

// PlayCard puts Card into the con: a заход onto an empty con (any card but Дама♥,
// R-5.2) or a бой of the top card by §3.
type PlayCard struct{ Card Card }

// TakeBottomAndPass takes the single bottom card of the con into hand and passes
// the turn (R-5.3b/R-5.8); forced for a handless-but-live player (R-5.9).
type TakeBottomAndPass struct{}

// PodkladkaWest tucks 6(2)♥ under a 7(3)♥ bottom; the whole con goes to the next
// player's hand and they open next (R-3.6.2/R-5.7.1).
type PodkladkaWest struct{}

func (PlayCard) isAction()         {}
func (TakeBottomAndPass) isAction() {}
func (PodkladkaWest) isAction()     {}
```

- [ ] **Step 4: Implement `LegalActions`**

Create `engine/legal.go`:

```go
package engine

import "slices"

// LegalActions lists the actions `seat` may take right now in Guard (§14.2). Only
// the player to move has actions this iteration (social/ШУХ actions arrive in
// iteration 4); other seats and a finished game yield nil. It is the executable
// specification of §3/§5 that Apply validates against.
func LegalActions(s State, seat SeatID) []Action {
	if s.Phase == Finished || seat != s.Turn {
		return nil
	}
	hand := s.Hands[seat]

	if len(s.Table) == 0 {
		// Заход: any card but Дама♥ (R-5.2). A lone Дама♥ yields nil — the Guard
		// skip (§14.4) keeps Turn from ever resting here.
		var out []Action
		for _, c := range hand {
			if !IsQueenHearts(c) {
				out = append(out, PlayCard{Card: c})
			}
		}
		return out
	}

	if len(hand) == 0 {
		// Handless but live: a card of theirs hangs in the open con (R-5.9); the
		// only move is to take the bottom.
		return []Action{TakeBottomAndPass{}}
	}

	top := s.Table[len(s.Table)-1].Card
	var out []Action
	for _, c := range hand {
		if CanBeat(top, c) { // §3 (Дама♥ beats anything — its legal use is a бой)
			out = append(out, PlayCard{Card: c})
		}
	}
	out = append(out, TakeBottomAndPass{}) // R-5.3b: always available on a non-empty con
	if s.Rules.IsSecondLowestHeart(s.Table[0].Card) {
		west := Card{Suit: Hearts, Rank: s.Rules.LowestRank()}
		if slices.Contains(hand, west) {
			out = append(out, PodkladkaWest{}) // R-5.3c/R-3.6.2
		}
	}
	return out
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./engine/ -run TestLegalActions -v`
Expected: PASS (all six).

- [ ] **Step 6: Commit**

```bash
git add engine/action.go engine/legal.go engine/legal_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Action types + LegalActions for the con (§3/§5)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: `Apply` skeleton + заход + resolve helpers + events

**Files:**
- Create: `engine/apply.go`
- Create: `engine/event.go`
- Modify: `engine/state.go` (remove `GameStarted`, moved to `event.go`)
- Create: `engine/apply_test.go`

**Interfaces:**
- Consumes: `LegalActions`, `Action` types, `TableCard`, `IsQueenHearts`.
- Produces: `type IllegalAction struct{ Code, Rule string }` (`Error()`); `func Apply(s State, a Action) (State, []Event, error)`; helpers `(State) clone`, `(State) liveCount`, `(State) nextLive`, `(*State) settleTurn` (no-skip yet), `removeCard`; events `CardPlayed{Seat, Card}` + moved `GameStarted{Turn}`.

- [ ] **Step 1: Write the failing test**

Create `engine/apply_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyZahod(t *testing.T) {
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 7}})
	require.NoError(t, err)

	// Card moved hand→table with owner 0; turn passed to seat 1.
	require.Equal(t, []TableCard{{Card: Card{Spades, 7}, By: 0}}, ns.Table)
	require.ElementsMatch(t, []Card{{Diamonds, 9}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, []Event{CardPlayed{Seat: 0, Card: Card{Spades, 7}}}, events)

	// Input is untouched (Apply is pure).
	require.Empty(t, s.Table)
	require.Len(t, s.Hands[0], 2)
}

func TestApplyRejectsIllegal(t *testing.T) {
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)

	// Card not in hand.
	_, _, err := Apply(s, PlayCard{Card{Hearts, 10}})
	require.Error(t, err)
	var illegal *IllegalAction
	require.ErrorAs(t, err, &illegal)

	// Дама♥ заход is blocked in Guard.
	s2 := playing(map[SeatID][]Card{0: {{Hearts, Queen}, {Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err = Apply(s2, PlayCard{Card{Hearts, Queen}})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestApply -v`
Expected: BUILD FAIL — `undefined: Apply`, `undefined: IllegalAction`, `undefined: CardPlayed`.

- [ ] **Step 3: Move events into `event.go` and add `CardPlayed`**

In `engine/state.go`, delete the `Event` interface? **No** — keep the `Event` interface in `state.go`, but **delete** the `GameStarted` type and its `isEvent()` method (lines defining `GameStarted`). Leave the `Event interface{ isEvent() }` definition where it is.

Create `engine/event.go`:

```go
package engine

// GameStarted is emitted by NewGame once dealing is complete; Turn is the seat
// that opens the first con (holder of 9♦, R-5.1).
type GameStarted struct {
	Turn SeatID
}

// CardPlayed is emitted when a player puts a card into the con — a заход or a бой
// (R-5.2/§3).
type CardPlayed struct {
	Seat SeatID
	Card Card
}

func (GameStarted) isEvent() {}
func (CardPlayed) isEvent()  {}
```

- [ ] **Step 4: Implement `Apply`, the заход branch, and helpers**

Create `engine/apply.go`:

```go
package engine

import "slices"

// IllegalAction is the typed error Apply returns for a move that is not legal in
// the current state/mode (spec §4). Code is a short machine tag; Rule points at
// the governing rule/section.
type IllegalAction struct {
	Code string
	Rule string
}

func (e *IllegalAction) Error() string {
	return "engine: illegal action [" + e.Code + "] (" + e.Rule + ")"
}

// Apply is the pure con-lifecycle reducer (§5). It validates a against
// LegalActions(s, s.Turn), then applies it to a clone and runs the resolve
// pipeline (§14.3). The input is never mutated; on rejection it returns s
// unchanged plus an *IllegalAction.
func Apply(s State, a Action) (State, []Event, error) {
	if s.Phase == Finished {
		return s, nil, &IllegalAction{Code: "game_over", Rule: "R-10.1"}
	}
	turn := s.Turn
	if !slices.Contains(LegalActions(s, turn), a) {
		return s, nil, &IllegalAction{Code: "illegal_action", Rule: "§5"}
	}
	ns := s.clone()
	var events []Event

	switch act := a.(type) {
	case PlayCard:
		wasEmpty := len(ns.Table) == 0
		ns.Hands[turn] = removeCard(ns.Hands[turn], act.Card)
		ns.Table = append(ns.Table, TableCard{Card: act.Card, By: turn})
		events = append(events, CardPlayed{Seat: turn, Card: act.Card})
		if wasEmpty {
			// Заход: never closes (threshold ≥ 2); pass the turn.
			ns.settleTurn(ns.nextLive(turn), &events)
		} else {
			// Бой: close/no-close handled in later tasks.
			ns.settleTurn(ns.nextLive(turn), &events)
		}
	}
	return ns, events, nil
}

// clone returns a deep-enough copy for copy-on-write: all mutated maps and slices
// are fresh. Seats is immutable for the game and shared.
func (s State) clone() State {
	ns := s
	ns.Hands = make(map[SeatID][]Card, len(s.Hands))
	for k, v := range s.Hands {
		ns.Hands[k] = append([]Card(nil), v...)
	}
	ns.Table = append([]TableCard(nil), s.Table...)
	ns.Discard = append([]Card(nil), s.Discard...)
	ns.Shukh = make(map[SeatID][]Card, len(s.Shukh))
	for k, v := range s.Shukh {
		ns.Shukh[k] = append([]Card(nil), v...)
	}
	ns.Live = make(map[SeatID]bool, len(s.Live))
	for k, v := range s.Live {
		ns.Live[k] = v
	}
	ns.Finish = append([]SeatID(nil), s.Finish...)
	return ns
}

// liveCount is the number of players still in the game (R-5.5.1).
func (s State) liveCount() int {
	n := 0
	for _, v := range s.Live {
		if v {
			n++
		}
	}
	return n
}

// nextLive returns the first live seat strictly clockwise after `after`. Seats
// are the identity 0..n-1 (NewGame), so index arithmetic tracks SeatID. If no
// other seat is live it returns `after`.
func (s State) nextLive(after SeatID) SeatID {
	n := len(s.Seats)
	for k := 1; k <= n; k++ {
		seat := s.Seats[(int(after)+k)%n]
		if s.Live[seat] {
			return seat
		}
	}
	return after
}

// settleTurn points Turn at candidate. (Task 9 upgrades it to skip the Guard
// lone-Дама♥ opener, §14.4.)
func (s *State) settleTurn(candidate SeatID, events *[]Event) {
	s.Turn = candidate
}

// removeCard returns hand without its first occurrence of c. The caller has
// already validated (via LegalActions) that c is present.
func removeCard(hand []Card, c Card) []Card {
	for i, x := range hand {
		if x == c {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS (заход + rejection tests green; `deal_test.go` still compiles — `GameStarted` now lives in `event.go`).

- [ ] **Step 6: Commit**

```bash
git add engine/apply.go engine/event.go engine/state.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Apply reducer + заход + resolve helpers

Pure copy-on-write Apply validated against LegalActions; заход branch places a
card and passes the turn. Move Event types to event.go, add CardPlayed.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `Apply` — бой without close

**Files:**
- Modify: `engine/apply.go:Apply` (бой branch already passes turn; add a test to lock it)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: existing `Apply`, `CanBeat`.
- Produces: no new symbols — locks бой-no-close behavior.

- [ ] **Step 1: Write the failing test**

Add to `engine/apply_test.go`:

```go
func TestApplyBeatNoClose(t *testing.T) {
	// 3 live players, threshold 3. Con has 1 card (8♠); seat 0 beats with 10♠.
	// len(table) becomes 2 < 3 → no close, turn passes to seat 1.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
		2: {{Hearts, 9}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 2}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 10}, By: 0},
	}, ns.Table)
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, []Event{CardPlayed{Seat: 0, Card: Card{Spades, 10}}}, events)
	require.Empty(t, ns.Discard)
}
```

- [ ] **Step 2: Run to verify it passes (behavior already present)**

Run: `go test ./engine/ -run TestApplyBeatNoClose -v`
Expected: PASS — the бой branch in Task 3 already places the card and passes the turn when the con is short of the threshold.

> If it fails, the бой branch is wrong: confirm `Apply`'s `else` arm appends `TableCard{act.Card, turn}` and calls `settleTurn(nextLive(turn))`.

- [ ] **Step 3: Commit**

```bash
git add engine/apply_test.go
git commit -m "$(cat <<'EOF'
test(engine): бой below the closing threshold passes the turn

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Closing by count — sweep, exits, termination, R-5.7.2

**Files:**
- Modify: `engine/apply.go` (add close trigger, `closeCon`, `resolveExits`, `seatsFrom`, `handInCon`)
- Modify: `engine/event.go` (`ConClosed`, `ConSwept`, `PlayerFinished`, `GameFinished`)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: `liveCount`, `nextLive`, `settleTurn`.
- Produces: `(*State) closeCon(closer SeatID, events *[]Event)`, `(*State) resolveExits(order []SeatID, events *[]Event)`, `(State) seatsFrom(pivot SeatID) []SeatID`, `(State) handInCon(seat SeatID) bool`; events `ConClosed{By}`, `ConSwept{Cards}`, `PlayerFinished{Seat, Place}`, `GameFinished{Finish}`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/apply_test.go`:

```go
func TestApplyCloseByCount(t *testing.T) {
	// 2 live, threshold 2. Con has 8♠; seat 0 beats with 10♠ → len 2 == 2 → close.
	// Both keep other cards, so nobody exits; closer (0) opens next; con → discard.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Spades, 8}, {Spades, 10}}, ns.Discard)
	require.Equal(t, SeatID(0), ns.Turn) // closer opens (R-5.7)
	require.Equal(t, Playing, ns.Phase)
	require.Equal(t, []Event{
		CardPlayed{Seat: 0, Card: Card{Spades, 10}},
		ConClosed{By: 0},
		ConSwept{Cards: []Card{{Spades, 8}, {Spades, 10}}},
	}, events)
}

func TestApplyCloseExitsCloserAndEndsGame(t *testing.T) {
	// 2 live, threshold 2. Seat 1 opened 8♠ with its LAST card; seat 0 beats with
	// its LAST card 10♠ → close. Sweep empties the table → both are handless with
	// no card in con → both exit. liveCount 0 → game over. Order: clockwise from
	// closer (0) → [0, 1]; loser is the last-placed (1).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}},
		1: {},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, Finished, ns.Phase)
	require.Equal(t, []SeatID{0, 1}, ns.Finish) // 0 first (winner), 1 last (loser)
	require.Contains(t, events, PlayerFinished{Seat: 0, Place: 1})
	require.Contains(t, events, PlayerFinished{Seat: 1, Place: 2})
	require.Contains(t, events, GameFinished{Finish: []SeatID{0, 1}})
}

func TestApplyCloserExitedNextOpens(t *testing.T) {
	// 3 live, threshold 3. Con has 8♠,9♠ (by seats 2,1); seat 0 beats with its
	// LAST card 10♠ → len 3 == 3 → close. Seat 0 is now handless, table swept →
	// seat 0 exits. Closer exited → next live clockwise (seat 1) opens (R-5.7.2).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 10}},
		1: {{Clubs, 7}},
		2: {{Hearts, 9}},
	}, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 9}, By: 1},
	}, 0)

	ns, _, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Equal(t, Playing, ns.Phase)
	require.False(t, ns.Live[0])
	require.Equal(t, []SeatID{0}, ns.Finish)
	require.Equal(t, SeatID(1), ns.Turn) // R-5.7.2
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestApplyClose|TestApplyCloser' -v`
Expected: FAIL — `undefined: ConClosed` etc.; the бой branch does not close yet.

- [ ] **Step 3: Add the closing events**

Append to `engine/event.go`:

```go
// ConClosed is emitted when the con reaches a closing condition (count R-5.5 or
// Дама♥ R-3.7.1); By is the closer.
type ConClosed struct {
	By SeatID
}

// ConSwept is emitted when the closed con's cards move to the discard (R-5.6),
// bottom→top.
type ConSwept struct {
	Cards []Card
}

// PlayerFinished is emitted when a player exits the game (R-9.1); Place is the
// 1-based finishing place.
type PlayerFinished struct {
	Seat  SeatID
	Place int
}

// GameFinished is emitted once, when the game ends (R-10.1/R-10.1.1); Finish is
// the complete ranking, winner first, loser last.
type GameFinished struct {
	Finish []SeatID
}

func (ConClosed) isEvent()      {}
func (ConSwept) isEvent()       {}
func (PlayerFinished) isEvent()  {}
func (GameFinished) isEvent()   {}
```

- [ ] **Step 4: Add the close trigger and the resolve helpers**

In `engine/apply.go`, change the бой arm of the `PlayCard` case. Replace:

```go
		} else {
			// Бой: close/no-close handled in later tasks.
			ns.settleTurn(ns.nextLive(turn), &events)
		}
```

with:

```go
		} else if len(ns.Table) == ns.liveCount() {
			ns.closeCon(turn, &events)
		} else {
			ns.settleTurn(ns.nextLive(turn), &events)
		}
```

Then add these helpers to `engine/apply.go`:

```go
// closeCon sweeps the whole con to the discard (R-5.6, auto in Guard), applies
// R-9.1 exits clockwise from the closer, and hands the next заход to the closer —
// or, if the closer just exited, to the next live seat (R-5.7.2). The closing
// card is already on the table.
func (s *State) closeCon(closer SeatID, events *[]Event) {
	swept := make([]Card, len(s.Table))
	for i, tc := range s.Table {
		swept[i] = tc.Card
	}
	s.Discard = append(s.Discard, swept...)
	s.Table = nil
	*events = append(*events, ConClosed{By: closer}, ConSwept{Cards: swept})

	s.resolveExits(s.seatsFrom(closer), events)
	if s.Phase == Finished {
		return
	}
	cand := closer
	if !s.Live[closer] {
		cand = s.nextLive(closer)
	}
	s.settleTurn(cand, events)
}

// resolveExits applies R-9.1 exits for the seats in `order` (clockwise from the
// seat whose action emptied/changed the con), then checks termination
// (R-10.1/R-10.1.1). A live seat exits when its hand is empty and it has no card
// in the open con. When one or zero players remain, the game ends and the loser
// (if any) is appended last to Finish.
func (s *State) resolveExits(order []SeatID, events *[]Event) {
	for _, seat := range order {
		if s.Live[seat] && len(s.Hands[seat]) == 0 && !s.handInCon(seat) {
			s.Live[seat] = false
			s.Finish = append(s.Finish, seat)
			*events = append(*events, PlayerFinished{Seat: seat, Place: len(s.Finish)})
		}
	}
	if s.liveCount() <= 1 {
		s.Phase = Finished
		for _, seat := range s.Seats {
			if s.Live[seat] { // the loser still holds cards (R-10.1)
				s.Finish = append(s.Finish, seat)
			}
		}
		*events = append(*events, GameFinished{Finish: append([]SeatID(nil), s.Finish...)})
	}
}

// seatsFrom returns all seats in clockwise order starting at pivot (inclusive).
func (s State) seatsFrom(pivot SeatID) []SeatID {
	n := len(s.Seats)
	out := make([]SeatID, 0, n)
	for k := 0; k < n; k++ {
		out = append(out, s.Seats[(int(pivot)+k)%n])
	}
	return out
}

// handInCon reports whether any card in the open con was played by seat (R-9.1).
func (s State) handInCon(seat SeatID) bool {
	for _, tc := range s.Table {
		if tc.By == seat {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS (close-by-count, exit-and-end, closer-exited-next-opens, plus all prior).

- [ ] **Step 6: Commit**

```bash
git add engine/apply.go engine/event.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): close a con by count — sweep, exits, termination, R-5.7.2

Beat that reaches the closing threshold sweeps the con to discard, exits any
handless player (R-9.1), ends the game at liveCount ≤ 1 with a full Finish
ranking, and hands the next заход to the closer or, if it exited, the next live
seat (R-5.7.2).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: `Дама♥` immediate close (R-3.7.1)

**Files:**
- Modify: `engine/apply.go:Apply` (add the Дама♥ close trigger)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: `IsQueenHearts`, `closeCon`.
- Produces: no new symbols.

- [ ] **Step 1: Write the failing test**

Add to `engine/apply_test.go`:

```go
func TestApplyQueenHeartsImmediateClose(t *testing.T) {
	// 3 live, threshold 3, con has just 1 card. Seat 0 beats with Дама♥ → closes
	// immediately regardless of count (R-3.7.1). Closer (0) keeps a card and opens.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Diamonds, 6}},
		1: {{Clubs, 8}},
		2: {{Hearts, 9}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Spades, 8}, {Hearts, Queen}}, ns.Discard)
	require.Equal(t, SeatID(0), ns.Turn)
	require.Contains(t, events, ConClosed{By: 0})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestApplyQueenHeartsImmediateClose -v`
Expected: FAIL — with a 1-card con below threshold 3, the current trigger passes the turn instead of closing; `ns.Table` is not empty.

- [ ] **Step 3: Add the Дама♥ trigger**

In `engine/apply.go`, change the close condition. Replace:

```go
		} else if len(ns.Table) == ns.liveCount() {
			ns.closeCon(turn, &events)
```

with:

```go
		} else if IsQueenHearts(act.Card) || len(ns.Table) == ns.liveCount() {
			ns.closeCon(turn, &events) // Дама♥ closes immediately (R-3.7.1)
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/apply.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Дама♥ closes the con immediately (R-3.7.1)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: `TakeBottomAndPass` — take, owner exit, empty-con handoff

**Files:**
- Modify: `engine/apply.go:Apply` (add `case TakeBottomAndPass`)
- Modify: `engine/event.go` (`CardsTaken`)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: `resolveExits`, `nextLive`, `settleTurn`.
- Produces: event `CardsTaken{Seat, Cards}`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/apply_test.go`:

```go
func TestApplyTakeBottom(t *testing.T) {
	// 2 live. Con has 8♠ (by seat 1). Seat 0 takes it → hand gains 8♠, con empty,
	// turn passes to seat 1 (who then must заход). Seat 1 still has a card, so no
	// exit; game continues.
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.ElementsMatch(t, []Card{{Diamonds, 6}, {Spades, 8}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Equal(t, Playing, ns.Phase)
	require.Equal(t, []Event{CardsTaken{Seat: 0, Cards: []Card{{Spades, 8}}}}, events)
}

func TestApplyTakeBottomExitsOwnerAndEndsGame(t *testing.T) {
	// 2 live. Seat 1 is handless-but-live: its only card (8♠) is the con bottom
	// (R-5.9). Seat 0 takes it → seat 1 now has empty hand and no card in con →
	// exits. liveCount 1 → game over; seat 0 is the loser (last place).
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 6}},
		1: {},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Equal(t, Finished, ns.Phase)
	require.False(t, ns.Live[1])
	require.Equal(t, []SeatID{1, 0}, ns.Finish) // 1 out first (winner), 0 loser
	require.Contains(t, events, PlayerFinished{Seat: 1, Place: 1})
	require.Contains(t, events, GameFinished{Finish: []SeatID{1, 0}})
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run TestApplyTakeBottom -v`
Expected: FAIL — `undefined: CardsTaken`; `TakeBottomAndPass` has no `Apply` case (no-op, table unchanged).

- [ ] **Step 3: Add the `CardsTaken` event**

Append to `engine/event.go`:

```go
// CardsTaken is emitted when a player takes cards into hand: one bottom card
// (R-5.8) or a whole eaten con after западло (R-3.6.2).
type CardsTaken struct {
	Seat  SeatID
	Cards []Card
}

func (CardsTaken) isEvent() {}
```

- [ ] **Step 4: Add the `TakeBottomAndPass` case**

In `engine/apply.go`, add a case to the `switch act := a.(type)` (after the `PlayCard` case):

```go
	case TakeBottomAndPass:
		taken := ns.Table[0]
		ns.Table = ns.Table[1:]
		ns.Hands[turn] = append(ns.Hands[turn], taken.Card)
		events = append(events, CardsTaken{Seat: turn, Cards: []Card{taken.Card}})
		// Taking can only shrink the con, so it never closes; but removing the
		// bottom card may free its (handless) owner to exit (R-9.1).
		ns.resolveExits([]SeatID{taken.By}, &events)
		if ns.Phase != Finished {
			ns.settleTurn(ns.nextLive(turn), &events)
		}
```

> `act` is unused in this case; that is fine — Go does not flag an unused type-switch binding.

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/apply.go engine/event.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): take the bottom card (R-5.8) with owner exit (R-5.9/R-9.1)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: `PodkladkaWest` — западло, eater opens, others exit

**Files:**
- Modify: `engine/apply.go:Apply` (add `case PodkladkaWest`)
- Modify: `engine/event.go` (`PodkladkaPlayed`)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: `RuleSet.LowestRank`, `removeCard`, `nextLive`, `resolveExits`, `seatsFrom`, `settleTurn`, `CardsTaken`.
- Produces: event `PodkladkaPlayed{Seat, Eater}`.

- [ ] **Step 1: Write the failing test**

Add to `engine/apply_test.go`:

```go
func TestApplyPodkladkaWest(t *testing.T) {
	// 3 live. Con bottom is 7♥ (by seat 2); seat 0 tucks 6♥ under → whole con
	// (6♥ + 7♥) goes to the next live seat (1) who opens next (R-5.7.1). Seat 0
	// shed its last card but seat 1 ate the con, so nobody's cards remain on an
	// (now empty) table; seat 0 becomes handless with no card in con → exits.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 8}},
		2: {{Diamonds, 9}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 2}}, 0)

	ns, events, err := Apply(s, PodkladkaWest{})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.Empty(t, ns.Discard) // eaten, not swept
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Hearts, 6}, {Hearts, 7}}, ns.Hands[1])
	require.False(t, ns.Live[0]) // shed last card, exits
	require.Equal(t, SeatID(1), ns.Turn)
	require.Contains(t, events, PodkladkaPlayed{Seat: 0, Eater: 1})
	require.Contains(t, events, CardsTaken{Seat: 1, Cards: []Card{{Hearts, 6}, {Hearts, 7}}})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestApplyPodkladkaWest -v`
Expected: FAIL — `undefined: PodkladkaPlayed`; no `Apply` case for `PodkladkaWest`.

- [ ] **Step 3: Add the `PodkladkaPlayed` event**

Append to `engine/event.go`:

```go
// PodkladkaPlayed is emitted for the западло move (R-3.6.2): Seat tucked 6(2)♥
// under the con and Eater received the whole con.
type PodkladkaPlayed struct {
	Seat  SeatID
	Eater SeatID
}

func (PodkladkaPlayed) isEvent() {}
```

- [ ] **Step 4: Add the `PodkladkaWest` case**

In `engine/apply.go`, add after the `TakeBottomAndPass` case:

```go
	case PodkladkaWest:
		west := Card{Suit: Hearts, Rank: ns.Rules.LowestRank()} // 6(2)♥
		ns.Hands[turn] = removeCard(ns.Hands[turn], west)
		eater := ns.nextLive(turn)
		eaten := make([]Card, 0, len(ns.Table)+1)
		eaten = append(eaten, west) // tucked under the bottom (R-3.6.2)
		for _, tc := range ns.Table {
			eaten = append(eaten, tc.Card)
		}
		ns.Hands[eater] = append(ns.Hands[eater], eaten...)
		ns.Table = nil
		events = append(events,
			PodkladkaPlayed{Seat: turn, Eater: eater},
			CardsTaken{Seat: eater, Cards: eaten},
		)
		// The table is emptied (eaten): any handless player whose cards were in it
		// exits; the eater cannot (it just gained the con). Eater opens (R-5.7.1).
		ns.resolveExits(ns.seatsFrom(turn), &events)
		if ns.Phase != Finished {
			ns.settleTurn(eater, &events)
		}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/apply.go engine/event.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): западло — 6(2)♥ under 7(3)♥, eater opens (R-3.6.2/R-5.7.1)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Guard skip of the forced lone-`Дама♥` opener (§14.4)

**Files:**
- Modify: `engine/apply.go` (`forcedQueenSkip` + upgrade `settleTurn`)
- Modify: `engine/event.go` (`TurnSkipped`)
- Test: `engine/apply_test.go`

**Interfaces:**
- Consumes: `IsQueenHearts`, `nextLive`.
- Produces: `(State) forcedQueenSkip(seat SeatID) bool`; upgraded `settleTurn`; event `TurnSkipped{Seat}`.

- [ ] **Step 1: Write the failing test**

Add to `engine/apply_test.go`:

```go
func TestApplyGuardSkipsLoneQueenOpener(t *testing.T) {
	// 2 live. Con has 8♠ (by seat 1). Seat 0 takes it → con empty, turn would pass
	// to seat 1, whose only card is Дама♥ → its sole "move" is the forbidden
	// заход. Guard skips seat 1 (§14.4) back to seat 0, who opens. TurnSkipped fires.
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 6}},
		1: {{Hearts, Queen}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)

	ns, events, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Equal(t, Playing, ns.Phase)
	require.Equal(t, SeatID(0), ns.Turn) // seat 1 skipped, back to 0
	require.Contains(t, events, TurnSkipped{Seat: 1})
	require.NotEmpty(t, LegalActions(ns, ns.Turn)) // Turn is always resolvable
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestApplyGuardSkipsLoneQueenOpener -v`
Expected: FAIL — `undefined: TurnSkipped`; `settleTurn` sets `Turn` to seat 1 (a stuck seat) instead of skipping.

- [ ] **Step 3: Add the `TurnSkipped` event**

Append to `engine/event.go`:

```go
// TurnSkipped is emitted when a Guard turn is skipped because the seat's only
// possible move is the forbidden lone-Дама♥ заход (§14.4). Guard-only: Middle and
// Culture instead allow it and catch it as Ш-2.
type TurnSkipped struct {
	Seat SeatID
}

func (TurnSkipped) isEvent() {}
```

- [ ] **Step 4: Add `forcedQueenSkip` and upgrade `settleTurn`**

In `engine/apply.go`, add:

```go
// forcedQueenSkip reports the Guard-only lone-Дама♥ opener case (§14.4): the con
// is empty and seat's only card is Дама♥, so its sole "move" would be the
// forbidden Дама♥ заход (R-3.7.2). Such a turn is skipped in Guard.
func (s State) forcedQueenSkip(seat SeatID) bool {
	h := s.Hands[seat]
	return len(s.Table) == 0 && len(h) == 1 && IsQueenHearts(h[0])
}
```

Replace the existing `settleTurn` body:

```go
func (s *State) settleTurn(candidate SeatID, events *[]Event) {
	s.Turn = candidate
}
```

with:

```go
// settleTurn points Turn at candidate, skipping past a live seat stuck in the
// Guard lone-Дама♥ opener case (§14.4) and emitting TurnSkipped for it. At most
// one seat can qualify (a single Дама♥ exists), so this skips at most once.
func (s *State) settleTurn(candidate SeatID, events *[]Event) {
	seat := candidate
	for s.forcedQueenSkip(seat) {
		*events = append(*events, TurnSkipped{Seat: seat})
		seat = s.nextLive(seat)
	}
	s.Turn = seat
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/apply.go engine/event.go engine/apply_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Guard skips the forced lone-Дама♥ opener (§14.4)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: `CheckInvariants` — I-6 + beat-stack oracle

**Files:**
- Modify: `engine/invariants.go`
- Test: `engine/invariants_test.go`

**Interfaces:**
- Consumes: `CanBeat`, `IsQueenHearts`, `TableCard`.
- Produces: extended `CheckInvariants` (I-6 + beat-stack).

- [ ] **Step 1: Write the failing tests**

Add to `engine/invariants_test.go`:

```go
func TestCheckInvariantsBeatStackOK(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	// A legal stack: 8♠ then higher ♠ 10♠. Move them out of a hand to keep I-1.
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 8})
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 10})
	s.Table = []TableCard{
		{Card: Card{Spades, 8}, By: 0},
		{Card: Card{Spades, 10}, By: 0},
	}
	require.NoError(t, CheckInvariants(s))
}

func TestCheckInvariantsBeatStackViolation(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = removeCard(s.Hands[0], Card{Spades, 8})
	s.Hands[0] = removeCard(s.Hands[0], Card{Diamonds, 14})
	// ♦ over ♠ is illegal (I-7): the beat-stack oracle must reject it.
	s.Table = []TableCard{
		{Card: Card{Spades, 8}, By: 0},
		{Card: Card{Diamonds, 14}, By: 0},
	}
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI6QueenOnTable(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = removeCard(s.Hands[0], Card{Hearts, Queen})
	s.Table = []TableCard{{Card: Card{Hearts, Queen}, By: 0}} // I-6: never rests on table
	require.Error(t, CheckInvariants(s))
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestCheckInvariantsBeatStack|TestCheckInvariantsI6' -v`
Expected: FAIL — the two violation tests currently pass I-1 (cards conserved) so `CheckInvariants` returns nil; the oracle is missing.

- [ ] **Step 3: Add I-6 + the beat-stack oracle**

In `engine/invariants.go`, just before `return nil`, add:

```go
	// I-6 + beat-stack oracle (§14.5): the con is a legal stack — each card
	// legally beats the one below it (⇒ I-7) — and Дама♥ never rests on the table
	// (it closes the con immediately, R-3.7.1).
	for i, tc := range s.Table {
		if IsQueenHearts(tc.Card) {
			return fmt.Errorf("engine: I-6 violated: Дама♥ present on the con")
		}
		if i > 0 && !CanBeat(s.Table[i-1].Card, tc.Card) {
			return fmt.Errorf("engine: beat-stack violated: %v does not legally beat %v", tc.Card, s.Table[i-1].Card)
		}
	}
	return nil
```

(Remove the old bare `return nil` that this replaces.)

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/invariants.go engine/invariants_test.go
git commit -m "$(cat <<'EOF'
feat(engine): CheckInvariants — I-6 + beat-stack oracle (⇒ I-7)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Seeded fuzz — random legal game to termination

**Files:**
- Create: `engine/fuzz_test.go` (`package engine_test` — avoids the shuffle→engine cycle)

**Interfaces:**
- Consumes: `NewGame`, `Config`, `Player`, `RuleSet`, `LegalActions`, `Apply`, `CheckInvariants`, `State`, `Finished`, `shuffle.Deck`, `engine.NewDeck`.
- Produces: `TestFuzzGamesTerminate`.

- [ ] **Step 1: Write the fuzz test**

Create `engine/fuzz_test.go`:

```go
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

			legal := engine.LegalActions(s, s.Turn)
			require.NotEmptyf(t, legal, "seed %d step %d: Turn %d has no legal action", seed, steps, s.Turn)

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
```

- [ ] **Step 2: Run the fuzz test**

Run: `go test ./engine/ -run TestFuzzGamesTerminate -v`
Expected: PASS — 200 games across deck sizes 36/52 and 2/3/4/6 players terminate with valid rankings; no invariant violation.

> If a game exceeds `maxSteps` or an invariant fails, the failure message carries the `seed` and `step` — replay that exact seed with a focused test to debug (games are deterministic in `(seed, deck, choices)`).

- [ ] **Step 3: Run the full package + vet**

Run: `go test ./... && go vet ./...`
Expected: PASS, no vet complaints.

- [ ] **Step 4: Commit**

```bash
git add engine/fuzz_test.go
git commit -m "$(cat <<'EOF'
test(engine): seeded fuzz — random legal games run to a valid Finish

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage (§14):**
- §14.1 TableCard ownership → Task 1. ✅
- §14.2 actions + LegalActions + "Turn always resolvable" → Task 2 (+ asserted in Tasks 9, 11). ✅
- §14.3 resolve pipeline: заход (T3), бой (T4), close-by-count + sweep + exits + termination + R-5.7.2 (T5), Дама♥ close (T6), take + owner-exit + empty-con (T7), западло + eater opens (T8), termination R-10.1.1 (T5 exit-to-zero test). ✅
- §14.4 Guard Дама♥ skip → Task 9. ✅
- §14.5 I-6 + beat-stack oracle → Task 10. ✅
- §14.6 tests + fuzz → Tasks 2–10 (unit) + Task 11 (fuzz). ✅
- Rules R-5.7.2 (closer/eater exited → next live) → Task 5 (`TestApplyCloserExitedNextOpens`) + covered for западло by `nextLive` eater selection. R-10.1.1 (simultaneous zero) → Task 5 (`TestApplyCloseExitsCloserAndEndsGame`). ✅

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; every test shows real assertions. ✅

**3. Type consistency:** `TableCard{Card, By}`, `Apply(State, Action) (State, []Event, error)`, `settleTurn(SeatID, *[]Event)`, `resolveExits([]SeatID, *[]Event)`, `closeCon(SeatID, *[]Event)`, `nextLive(SeatID) SeatID`, `seatsFrom(SeatID) []SeatID`, `handInCon(SeatID) bool`, `forcedQueenSkip(SeatID) bool`, event field names (`Seat`, `Card`, `By`, `Cards`, `Place`, `Finish`, `Eater`) — used identically across producing and consuming tasks. `GameStarted` moves from `state.go` to `event.go` in Task 3; `deal.go`'s reference stays valid (same package). ✅

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-engine-iteration-3-con-lifecycle.md`. Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
