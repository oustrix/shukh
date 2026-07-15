# Engine Iteration 4 — ШУХ core §7–§8 + «Одна карта» §6 + Endgame §9.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the deterministic ШУХ core to the engine for **Guard + Middle** modes: the Middle catch-window (`Unsettled`) for Ш-2/Ш-3, the §8 payment gate (players give cards into the offender's `Shukh` zone, then take them back after the con), «одна карта» §6 (Ш-11), and endgame `6(2)♥` §9.2 (Ш-12) — every path detectable and auto-judged, no undecided plumbing.

**Architecture:** All new mechanics ride the existing pure copy-on-write `Apply(State, Action)` reducer. A *разрешённо-нелегальное* action in Middle (Дама♥ заход, early ШУХ take, endgame 6(2)♥ заход) snapshots the pre-action state into `State.Unsettled` and passes the turn; any live player may `ClaimShukh` to reverse (restore the snapshot) and assess the ШУХ, or the next player's normal move settles it (R-1.4.1). A confirmed ШУХ opens a serialized `State.Pending` payment gate: each obligated player picks a non-last card (`GiveShukhCard`) into the offender's `Shukh` zone (I-2/I-3), takeable after the con closes (`TakeShukhCards`, R-8.3). «Одна карта» and endgame are auto-judged by `AskCount`/`AskAboutWest`. `CheckInvariants` is gated: I-1 always; I-6/I-7/beat-stack only when `Unsettled == nil`.

**Tech Stack:** Go 1.26.1, standard library (`slices`, `fmt`, `maps`), `github.com/stretchr/testify/require`, `math/rand/v2` (test-only, via the `shuffle` package). No new dependencies.

## Global Constraints

- **Authoritative spec:** `docs/superpowers/specs/2026-07-15-engine-core-design.md` §15 (iteration-4 decisions), building on §14 (iteration 3). Rules of truth: `docs/shukh-rules.md` (`R-§.n`, `Ш-n`, `I-n`) — especially §6, §7, §8, §9.1–§9.2, §11, and R-9.4.3.
- **Modes:** iteration 4 targets **Guard + Middle** (D-10). Culture stays out (needs deferred rollback). Config already rejects Culture.
- **In scope (§15.1):** mode-gating of Дама♥ / early-take; `Unsettled` window for **Ш-2** (Дама♥ заход) and **Ш-3** (early `TakeShukhCards`); §8 payment gate + `Shukh` zone + `TakeShukhCards` (I-2/I-3); «одна карта» §6 + **Ш-11**; endgame §9.2 + **Ш-12** (incl. R-9.4.3).
- **Out of scope (§15.1):** subjective ШУХ Ш-6/Ш-8/Ш-9/Ш-10 + voting `R-8.6` (→ Спец 2, so **no `Adjudication`/`Vote`**); returning-player R-9.5; Culture; **Ш-4/Ш-5** (отбой is auto-swept in `Apply`, so they never arise — the Middle catch-set is exactly `{Ш-2, Ш-3}` plus the auto-judged Ш-11/Ш-12).
- **`Apply` is pure:** never mutates its input; returns a new `State` + `[]Event`, or a typed `*IllegalAction`. No `panic` on any input. The **public API §4 is stable** — `Apply(State, Action)` and `LegalActions(State, SeatID)` signatures do **not** change.
- **Existing style:** tests are white-box `package engine` using `testify/require`; the fuzz test is `package engine_test` (avoids the shuffle→engine cycle); doc-comment every exported symbol with its rule references; commits `feat|test|fix|refactor(engine): …` with the `Co-Authored-By` trailer.
- **Test data model:** unit tests build focused states with the `playing(...)` helper (iteration 3, `legal_test.go`) and set the new fields directly (`s.Mode = Middle`, `s.Shukh = …`, `s.OwesOneCard = …`); they do **not** require full I-1. Full I-1/termination coverage comes from the fuzz test (Task 12).

## Plan Decisions (resolving §13 / §15 open items)

These are pinned here (the design doc delegated the exact forms to the plan). Reviewers hold the implementation to this contract.

- **P-1 — Actor model (keeps `Apply(s, a)` / `LegalActions(s, seat)` stable).** Turn-actions (`PlayCard`, `TakeBottomAndPass`, `PodkladkaWest`, `DiscardWest`) act as `s.Turn`. Actions that need an explicit actor carry it: `DeclareOneCard{Seat}`, `TakeShukhCards{Seat}`. `GiveShukhCard{Card}` acts as the current payer `s.Pending.Owed[0]` (deterministic payer order). `ClaimShukh{Target, Code}`, `AskCount{Target}`, `AskAboutWest{Target}` are **actor-agnostic**: their outcome depends only on the target/window, so they are validated by precondition (any live non-target seat may raise them).
- **P-2 — `ShukhCode` form.** `type ShukhCode int` with named constants `Sh2, Sh3, Sh11, Sh12` (numeric value = the Ш-number). It is the `Reason`/`Code` carried by `ClaimShukh` and reported by `ShukhAssessed`.
- **P-3 — Payment order is deterministic and outcome-irrelevant.** `Payment.Owed` is a FIFO queue in clockwise order from the offender; the head pays first. Each obligated player freely chooses *which* card to give — order of payers does not change the resulting `Shukh` pile, so serializing it is faithful and deterministic.
- **P-4 — `ShukhTakeable` timing (R-8.3).** A pending `Shukh` pile becomes takeable when the con it was laid in ends — modeled as: **whenever the table transitions to empty** (close/sweep, западло-eat, or take-bottom-to-empty), mark every seat with a non-empty `Shukh` pile takeable; and immediately at payment time if the table is already empty. Only one con is open at a time, so this equals "when the current con closes."
- **P-5 — `DiscardWest{}` semantics.** In the endgame, `DiscardWest{}` is a **turn-action** available to `s.Turn` when the con is empty (заход moment) and the seat holds `6(2)♥`: it moves `6(2)♥` to the discard (R-9.3) and passes the turn (it *is* that seat's move for the turn). After a Ш-12, the offender is **obligated**: `Endgame.MustDiscard` restricts their turn to `DiscardWest` until done.
- **P-6 — Endgame 6(2)♥ «использование» (R-9.4.3).** Only the **заход** of `6(2)♥` counts as illegal use in the endgame. In Guard it is blocked; in Middle it is allowed-and-caught as Ш-2/Ш-12 via the `Unsettled` window (Code `Sh12`), whose catch reverses the заход, assesses Ш-12, applies the skip, and sets `Endgame.MustDiscard`. **Западло of `6(2)♥` is forbidden in the endgame in both modes** (`LegalActions` never offers `PodkladkaWest` for it) — the card must go to the discard, not a hand.
- **P-7 — Meaningful-only social actions in `LegalActions`.** The engine surfaces a social action only where it has an effect (`DeclareOneCard{seat}` iff `OwesOneCard[seat]`; `AskCount{t}` iff `OwesOneCard[t]`; `ClaimShukh` iff the `Unsettled` window matches; `AskAboutWest{t}` iff endgame is active and unasked). A "false" trigger is therefore simply rejected by `Apply` (its subjective punishment Ш-8/Ш-9 is Спец 2, §15.5). This also keeps the fuzz driver progressing (no no-op legal moves).

---

## File Structure

**Modify:**
- `engine/state.go` — add `Unsettled`, `Payment`, `EndgameState`, `ShukhCode` types and the five new `State` fields (`Unsettled`, `Pending`, `Endgame`, `OwesOneCard`, `ShukhTakeable`).
- `engine/action.go` — add `DeclareOneCard`, `AskCount`, `ClaimShukh`, `GiveShukhCard`, `TakeShukhCards`, `AskAboutWest`, `DiscardWest`.
- `engine/event.go` — add `ShukhAssessed`, `ShukhPaid`, `ActionReverted`, `OneCardDeclared`, `ShukhCardsTaken`, `WestDiscarded`.
- `engine/beat.go` — make `CanBeat` return false for a `Дама♥` top (it is the highest card; nothing beats it — R-3.7).
- `engine/legal.go` — mode-gate the Дама♥/6(2)♥ заход; add the `Unsettled`/`Pending`/social/endgame branches.
- `engine/apply.go` — mode-gate `settleTurn`; new `isLegal` routing (P-1); `Unsettled` set/settle/reverse; `assessShukh`/`applyShukhEffect`/`markShukhTakeable`; `reconcileOneCard`; the new `Apply` cases; `clone` deep-copies the new fields; endgame detection in `resolveExits`.
- `engine/invariants.go` — gate I-6/beat-stack behind `Unsettled == nil` (I-1 always).
- `engine/deal.go` — `NewGame` initializes `OwesOneCard`/`ShukhTakeable` empty maps.

**Create:**
- `engine/shukh_test.go` — unit tests for the ШУХ core (Ш-2/Ш-3, payment, take).
- `engine/onecard_test.go` — «одна карта» §6 tests.
- `engine/endgame_test.go` — endgame §9.2 tests.
- (extend) `engine/fuzz_test.go` — a Middle fuzz variant driving social actions.

---

# SUB-ITERATION 1 — `Unsettled` window + reverse (Ш-2), mode-gating

### Task 1: New `State` fields, types, `clone`, `NewGame` init, gate helpers, `CheckInvariants` gating

**Files:**
- Modify: `engine/state.go` (types + fields)
- Modify: `engine/apply.go` (`clone` deep-copies new fields; `gatesClosed` helper)
- Modify: `engine/deal.go` (`NewGame` inits new maps)
- Modify: `engine/invariants.go` (gate I-6/beat-stack behind `Unsettled == nil`)
- Test: `engine/invariants_test.go`

**Interfaces:**
- Produces: `type ShukhCode int` (`Sh2, Sh3, Sh11, Sh12`); `type Unsettled struct { Prev State; Seat SeatID; Code ShukhCode }`; `type Payment struct { Offender SeatID; Owed []SeatID; Skip bool; ThenDiscardWest bool }`; `type EndgameState struct { Active, Asked, MustDiscard bool }`; `State` fields `Unsettled *Unsettled`, `Pending *Payment`, `Endgame EndgameState`, `OwesOneCard map[SeatID]bool`, `ShukhTakeable map[SeatID]bool`; `func (s State) gatesClosed() bool`.

- [ ] **Step 1: Write the failing test**

Add to `engine/invariants_test.go`:

```go
func TestCheckInvariantsSkipsBeatStackWhenUnsettled(t *testing.T) {
	// A Дама♥ resting on the table violates I-6 in a STABLE state, but during an
	// open Middle catch-window (Unsettled != nil) only I-1 is asserted (§15.3).
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = removeCard(s.Hands[0], Card{Hearts, Queen})
	s.Table = []TableCard{{Card: Card{Hearts, Queen}, By: 0}}

	// Stable: I-6 fires.
	require.Error(t, CheckInvariants(s))

	// Unsettled: I-1 still holds (cards conserved), so no error.
	s.Unsettled = &Unsettled{Seat: 0, Code: Sh2}
	require.NoError(t, CheckInvariants(s))
}
```

- [ ] **Step 2: Run to verify it fails to compile**

Run: `go test ./engine/ -run TestCheckInvariantsSkipsBeatStackWhenUnsettled -v`
Expected: BUILD FAIL — `undefined: Unsettled`, `undefined: Sh2`.

- [ ] **Step 3: Add the new types and `State` fields**

In `engine/state.go`, add the types (near `EnforcementMode`):

```go
// ShukhCode identifies a ШУХ trigger from the §7 catalogue (its numeric value is
// the Ш-number). Iteration 4 handles the detectable set {Sh2, Sh3, Sh11, Sh12}.
type ShukhCode int

const (
	Sh2  ShukhCode = 2  // зашёл с Дамы♥ (R-3.7.2) — extra effect: skip turn
	Sh3  ShukhCode = 3  // взял ШУХ раньше завершения кона (R-8.3)
	Sh11 ShukhCode = 11 // не объявил «Одна карта!» (R-6.2)
	Sh12 ShukhCode = 12 // эндшпиль: держит/использовал 6(2)♥ (R-9.4) — skip + discard
)

// Unsettled is an open Middle catch-window (§6.1/§15.3): a разрешённо-нелегальное
// action has been applied but not yet settled. Prev is the snapshot of State from
// BEFORE that action — reversing the ШУХ is exactly restoring it (no per-action
// undo). Seat is the offender; Code is what may be claimed (Sh2 | Sh3 | Sh12).
type Unsettled struct {
	Prev State
	Seat SeatID
	Code ShukhCode
}

// Payment is an active §8 payment gate: obligated players give the offender one
// card each into his Shukh zone. Owed is the FIFO queue of live seats ≠ offender
// with ≥2 cards that have not yet paid (R-8.1/R-8.1.1, P-3). Skip applies the
// offender's turn-skip once payment completes (Ш-2/Ш-12); ThenDiscardWest sets the
// Ш-12 6(2)♥-discard obligation (R-9.4.3).
type Payment struct {
	Offender        SeatID
	Owed            []SeatID
	Skip            bool
	ThenDiscardWest bool
}

// EndgameState tracks the §9.2 two-player endgame. Active is set when liveCount
// falls to 2 (R-9.3 kicks in). Asked records that the 6(2)♥ question was asked
// (ending the безнаказанно window, R-9.4.1). MustDiscard obligates the 6(2)♥
// holder to DiscardWest before any other move (post-Ш-12, R-9.4.3).
type EndgameState struct {
	Active      bool
	Asked       bool
	MustDiscard bool
}
```

Then add the five fields to `State` (after `Finish`):

```go
	Endgame       EndgameState    // §9.2 endgame flags
	Pending       *Payment        // active §8 payment gate; nil = none
	Unsettled     *Unsettled      // Middle catch-window; nil = stable
	OwesOneCard   map[SeatID]bool // §6: seat at 1 card, not yet declared/moved (R-6.1)
	ShukhTakeable map[SeatID]bool // §8: seat may lift its Shukh pile into hand (R-8.3)
```

- [ ] **Step 4: Deep-copy the new fields in `clone` and add `gatesClosed`**

In `engine/apply.go`, inside `clone`, before `return ns`, add:

```go
	ns.OwesOneCard = make(map[SeatID]bool, len(s.OwesOneCard))
	for k, v := range s.OwesOneCard {
		ns.OwesOneCard[k] = v
	}
	ns.ShukhTakeable = make(map[SeatID]bool, len(s.ShukhTakeable))
	for k, v := range s.ShukhTakeable {
		ns.ShukhTakeable[k] = v
	}
	if s.Pending != nil {
		cp := *s.Pending
		cp.Owed = append([]SeatID(nil), s.Pending.Owed...)
		ns.Pending = &cp
	}
	if s.Unsettled != nil {
		cp := *s.Unsettled // Prev is a snapshot we never mutate; sharing its maps is safe
		ns.Unsettled = &cp
	}
```

> `Endgame` is a value field and is copied by the `ns := s` struct copy at the top of `clone`. `OwesOneCard`/`ShukhTakeable` are always allocated non-nil here so later writes never panic even when the input state left them nil (e.g. the `playing` test helper).

Add the `gatesClosed` helper to `engine/apply.go`:

```go
// gatesClosed reports whether no catch-window or payment gate is open — i.e. the
// game is in a normal position where turn-actions and fresh social actions are
// available (§15.8: at most one of Unsettled/Pending is active at a time).
func (s State) gatesClosed() bool { return s.Unsettled == nil && s.Pending == nil }
```

- [ ] **Step 5: Initialize the new maps in `NewGame`**

In `engine/deal.go`, in `NewGame`, where the result `State` is assembled, set the two maps to empty (find the `State{…}` literal or the field assignments and add):

```go
	st.OwesOneCard = map[SeatID]bool{}
	st.ShukhTakeable = map[SeatID]bool{}
```

(Match the existing construction style in `deal.go` — if the state is built as a struct literal, add the two fields there; if fields are assigned to a `st` variable, add the two assignments alongside `st.Shukh = …`.)

- [ ] **Step 6: Gate I-6 / beat-stack behind `Unsettled == nil`**

In `engine/invariants.go`, wrap the con-shape loop (the `for i, tc := range s.Table` block that checks I-6 + beat-stack) in a guard. Replace:

```go
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

with:

```go
	// I-6 + beat-stack oracle hold only on a STABLE position (§15.3): during an
	// open Middle catch-window a Дама♥/6(2)♥ may transiently rest on the table.
	// I-1 (card conservation, above) always holds.
	if s.Unsettled == nil {
		for i, tc := range s.Table {
			if IsQueenHearts(tc.Card) {
				return fmt.Errorf("engine: I-6 violated: Дама♥ present on the con")
			}
			if i > 0 && !CanBeat(s.Table[i-1].Card, tc.Card) {
				return fmt.Errorf("engine: beat-stack violated: %v does not legally beat %v", tc.Card, s.Table[i-1].Card)
			}
		}
	}
	return nil
```

Update the `CheckInvariants` doc-comment's first sentence to note the gating (e.g. append: "The con-shape invariants (I-6, beat-stack ⇒ I-7) are checked only when `Unsettled == nil`; during a Middle catch-window only I-1 is asserted (§15.3).").

- [ ] **Step 7: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS (new test green; all iteration-3 tests still green — `deal_test.go`/`newgame_test.go` unaffected by the extra empty maps).

- [ ] **Step 8: Commit**

```bash
git add engine/state.go engine/apply.go engine/deal.go engine/invariants.go engine/invariants_test.go
git commit -m "$(cat <<'EOF'
feat(engine): iteration-4 State fields + CheckInvariants gating (§15.2/§15.3)

Add Unsettled/Payment/EndgameState/ShukhCode and the five new State fields;
clone deep-copies them; NewGame inits the maps. Gate I-6/beat-stack behind
Unsettled == nil so a Middle catch-window asserts only I-1.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Mode-gate the Дама♥ заход — `LegalActions` + `settleTurn`

**Files:**
- Modify: `engine/legal.go` (Дама♥ заход allowed in Middle)
- Modify: `engine/apply.go` (`settleTurn` skips only in Guard)
- Test: `engine/legal_test.go`

**Interfaces:**
- Consumes: `IsQueenHearts`, `forcedQueenSkip`, `Mode`.
- Produces: no new symbols — mode-dependent behavior.

- [ ] **Step 1: Write the failing test**

Add to `engine/legal_test.go`:

```go
func TestLegalActionsMiddleAllowsQueenZahod(t *testing.T) {
	// Empty con. In Guard the Дама♥ заход is blocked (§14.4); in Middle it is
	// allowed and caught as Ш-2 (§15.3), so it appears among legal заходы.
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Hearts, Queen}},
		1: {{Clubs, 8}},
	}, nil, 0)

	require.ElementsMatch(t, []Action{PlayCard{Card{Spades, 7}}}, LegalActions(s, 0)) // Guard

	s.Mode = Middle
	require.ElementsMatch(t, []Action{
		PlayCard{Card{Spades, 7}},
		PlayCard{Card{Hearts, Queen}},
	}, LegalActions(s, 0))
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestLegalActionsMiddleAllowsQueenZahod -v`
Expected: FAIL — Middle currently excludes the Дама♥ заход (the `!IsQueenHearts(c)` guard is unconditional).

- [ ] **Step 3: Mode-gate the заход filter**

In `engine/legal.go`, in the `len(s.Table) == 0` (заход) branch, replace:

```go
		var out []Action
		for _, c := range hand {
			// TODO(iter4): Guard blocks the Дама♥ заход (R-3.7.2); Middle/Culture allow it and catch as Ш-2.
			if !IsQueenHearts(c) {
				out = append(out, PlayCard{Card: c})
			}
		}
		return out
```

with:

```go
		var out []Action
		for _, c := range hand {
			// Дама♥ заход (R-3.7.2): Guard blocks it (§14.4); Middle allows it and
			// catches it as Ш-2 via the Unsettled window (§15.3).
			if IsQueenHearts(c) && s.Mode == Guard {
				continue
			}
			out = append(out, PlayCard{Card: c})
		}
		return out
```

- [ ] **Step 4: Gate the `settleTurn` Дама♥ skip to Guard**

In `engine/apply.go`, in `settleTurn`, replace:

```go
	// TODO(iter4): gate on s.Mode == Guard — Middle/Culture allow the Дама♥ заход and catch it as Ш-2 instead of skipping.
	for s.forcedQueenSkip(seat) {
		*events = append(*events, TurnSkipped{Seat: seat})
		seat = s.nextLive(seat)
	}
```

with:

```go
	// The lone-Дама♥ opener skip is a Guard-only device (§14.4). In Middle the
	// Дама♥ заход is allowed and caught as Ш-2, so Turn may legitimately rest on
	// such a seat; do not skip.
	for s.Mode == Guard && s.forcedQueenSkip(seat) {
		*events = append(*events, TurnSkipped{Seat: seat})
		seat = s.nextLive(seat)
	}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS (Middle offers the Дама♥ заход; Guard unchanged, iteration-3 skip tests green).

- [ ] **Step 6: Commit**

```bash
git add engine/legal.go engine/apply.go engine/legal_test.go
git commit -m "$(cat <<'EOF'
feat(engine): mode-gate the Дама♥ заход — Guard blocks, Middle allows (§15.3)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: `CanBeat` — `Дама♥` is unbeatable (R-3.7)

**Files:**
- Modify: `engine/beat.go`
- Test: `engine/beat_test.go`

**Interfaces:**
- Consumes: `IsQueenHearts`.
- Produces: no new symbols — corrects `CanBeat(Дама♥, *) == false`.

**Rationale:** During a Middle Ш-2 window a `Дама♥` rests as the top card (an illegal заход). The next player must be *forced to take* it (§15.3: «побить нельзя») so the window settles and I-6 is restored. Today `CanBeat(Дама♥, King♥)` returns `true` (the generic hearts rule `c.Rank > top.Rank`) because `Дама♥` never persisted as a top in Guard. It is the highest card (R-3.7); nothing beats it.

- [ ] **Step 1: Write the failing test**

Add to `engine/beat_test.go`:

```go
func TestCanBeatQueenHeartsIsUnbeatable(t *testing.T) {
	// Дама♥ is the highest card (R-3.7): nothing beats it — not King♥/Ace♥, not
	// a trump. (It only ever tops a con transiently during a Middle Ш-2 window.)
	for _, c := range []Card{{Hearts, King}, {Hearts, Ace}, {Diamonds, Ace}, {Spades, Ace}} {
		require.Falsef(t, CanBeat(Card{Hearts, Queen}, c), "%v must not beat Дама♥", c)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./engine/ -run TestCanBeatQueenHeartsIsUnbeatable -v`
Expected: FAIL — `King♥`/`Ace♥` currently "beat" Дама♥ via the same-suit-higher branch.

- [ ] **Step 3: Make `Дама♥` unbeatable**

In `engine/beat.go`, add a guard at the very start of `CanBeat` (after the `IsQueenHearts(c)` early return):

```go
func CanBeat(top, c Card) bool {
	if IsQueenHearts(c) {
		return true // R-3.7.1 — highest card, beats anything
	}
	if IsQueenHearts(top) {
		return false // R-3.7 — nothing beats Дама♥ (it only tops a con during a Ш-2 window)
	}
	switch top.Suit {
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./engine/ -v`
Expected: PASS (all beat-matrix tests still green; iteration-3 fuzz unaffected — Дама♥ never tops a stable con in Guard).

- [ ] **Step 5: Commit**

```bash
git add engine/beat.go engine/beat_test.go
git commit -m "$(cat <<'EOF'
fix(engine): Дама♥ is unbeatable as a top card (R-3.7)

Needed for the Middle Ш-2 window: the next player must be forced to take a
Дама♥ that rests as top, never offered a (non-existent) бой.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `Apply` — Ш-2 window: set `Unsettled`, settle-by-move, `ClaimShukh` reverse + skip

**Files:**
- Modify: `engine/action.go` (`ClaimShukh`)
- Modify: `engine/event.go` (`ShukhAssessed`, `ActionReverted`)
- Modify: `engine/legal.go` (`ClaimShukh` offered during a matching window)
- Modify: `engine/apply.go` (`isLegal` routing; `Unsettled` set on Дама♥ заход; settle-by-move; `ClaimShukh` case; `assessShukh`/`applyShukhEffect` — immediate-effect form, no payment yet)
- Test: `engine/shukh_test.go`

**Interfaces:**
- Consumes: `gatesClosed`, `nextLive`, `settleTurn`, `IsQueenHearts`, `clone`.
- Produces: `ClaimShukh{Target SeatID; Code ShukhCode}`; events `ShukhAssessed{Offender SeatID; Code ShukhCode}`, `ActionReverted{Seat SeatID}`; `func isLegal(s State, a Action) bool`; `func (s *State) assessShukh(offender SeatID, code ShukhCode, skip, thenDiscardWest bool, events *[]Event)`; `func (s *State) applyShukhEffect(offender SeatID, skip, thenDiscardWest bool, events *[]Event)`; `func (s *State) markShukhTakeable()`.

> **Payment note:** in this task `assessShukh` applies the effect **immediately** (no cards change hands) — Task 5 inserts the payment gate before the effect. This keeps Task 4 independently testable (window + reverse + skip) without the §8 machinery.

- [ ] **Step 1: Write the failing tests**

Create `engine/shukh_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// middle builds a playing() state switched to Middle.
func middle(hands map[SeatID][]Card, table []TableCard, turn SeatID) State {
	s := playing(hands, table, turn)
	s.Mode = Middle
	return s
}

func TestApplyMiddleQueenZahodSetsUnsettled(t *testing.T) {
	// Middle, empty con: seat 0 заходит with Дама♥ — allowed but нелегально. It
	// lands on the table, Unsettled snapshots the pre-action state, and the turn
	// passes to seat 1 (who can only take it — Дама♥ is unbeatable).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)

	ns, events, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	require.Equal(t, []TableCard{{Card: Card{Hearts, Queen}, By: 0}}, ns.Table)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, SeatID(0), ns.Unsettled.Seat)
	require.Equal(t, Sh2, ns.Unsettled.Code)
	require.Equal(t, SeatID(1), ns.Turn)
	// Seat 1 (next-to-act) can only take the Дама♥ (it is unbeatable — no бой) or
	// catch the ШУХ (R-8.9: any at-table player may claim, incl. the next player —
	// required for heads-up). No PlayCard бой is offered.
	require.ElementsMatch(t, []Action{
		ClaimShukh{Target: 0, Code: Sh2}, TakeBottomAndPass{},
	}, LegalActions(ns, 1))
}

func TestApplyMiddleQueenZahodSettledByNextMove(t *testing.T) {
	// If nobody claims, the next player's move «прижимает» the заход (R-1.4.1):
	// seat 1 takes the Дама♥, the window closes, and it becomes a normal position.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)

	ns2, _, err := Apply(ns, TakeBottomAndPass{})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)                              // settled
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Hearts, Queen}}, ns2.Hands[1])
	// I-6-restored on the stable position is asserted by the Task 12 fuzz on real
	// dealt games; partial-deck unit states here don't satisfy I-1, so CheckInvariants
	// is not called on them.
}

func TestApplyClaimShukhReversesQueenZahodAndSkips(t *testing.T) {
	// A claim in the window reverses the заход (Дама♥ back in seat 0's hand) and
	// assesses Ш-2 → skip seat 0's turn. (No payment in this task.)
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)
	require.Empty(t, ns2.Table)                                 // reversed off the table
	require.ElementsMatch(t, []Card{{Hearts, Queen}, {Spades, 7}}, ns2.Hands[0])
	require.Equal(t, SeatID(1), ns2.Turn)                       // seat 0 skipped (Ш-2)
	require.Contains(t, events, ActionReverted{Seat: 0})
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh2})
	// (No CheckInvariants here — partial-deck state; I-6-restored covered by fuzz.)
}

func TestApplyClaimShukhRejectedWithoutWindow(t *testing.T) {
	s := middle(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err := Apply(s, ClaimShukh{Target: 0, Code: Sh2}) // no window
	require.Error(t, err)
	// Wrong code while a window is open is also rejected.
	ns, _, _ := Apply(middle(map[SeatID][]Card{
		0: {{Hearts, Queen}, {Spades, 7}}, 1: {{Clubs, 8}},
	}, nil, 0), PlayCard{Card{Hearts, Queen}})
	_, _, err = Apply(ns, ClaimShukh{Target: 0, Code: Sh3})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestApplyMiddleQueen|TestApplyClaimShukh' -v`
Expected: BUILD FAIL — `undefined: ClaimShukh`, `undefined: ShukhAssessed`, `undefined: ActionReverted`.

- [ ] **Step 3: Add the `ClaimShukh` action and its events**

In `engine/action.go`, add:

```go
// ClaimShukh catches an open Middle catch-window (§15.3): Target is the offender
// (State.Unsettled.Seat), Code the claimed ШУХ (must match the window). It
// reverses the offending action and assesses the ШУХ. Actor-agnostic (P-1): any
// live non-target seat may raise it; the outcome depends only on the window.
type ClaimShukh struct {
	Target SeatID
	Code   ShukhCode
}

func (ClaimShukh) isAction() {}
```

In `engine/event.go`, add:

```go
// ShukhAssessed is emitted when a ШУХ is confirmed against Offender (§8); Code is
// the §7 trigger.
type ShukhAssessed struct {
	Offender SeatID
	Code     ShukhCode
}

// ActionReverted is emitted when a claimed ШУХ reverses the offender's last
// action by restoring the pre-action snapshot (§15.3).
type ActionReverted struct {
	Seat SeatID
}

func (ShukhAssessed) isEvent()  {}
func (ActionReverted) isEvent() {}
```

- [ ] **Step 4: Offer `ClaimShukh` in `LegalActions`**

In `engine/legal.go`, at the **top** of `LegalActions` (before the existing `if s.Phase == Finished || seat != s.Turn` guard, which must no longer short-circuit social actions), insert the window branch:

```go
func LegalActions(s State, seat SeatID) []Action {
	if s.Phase == Finished {
		return nil
	}
	// An open Middle catch-window: any live seat ≠ offender may claim it; the
	// offender's next-in-turn player may instead settle it with a normal move.
	if s.Unsettled != nil {
		var out []Action
		if seat != s.Unsettled.Seat && s.Live[seat] {
			out = append(out, ClaimShukh{Target: s.Unsettled.Seat, Code: s.Unsettled.Code})
		}
		if seat == s.Turn {
			out = append(out, turnActions(s, seat)...) // the settling move
		}
		return out
	}
	if seat != s.Turn {
		return nil
	}
	return turnActions(s, seat)
}
```

Then **extract** the existing body (from `hand := s.Hands[seat]` through the final `return out`) into a new helper `turnActions(s State, seat SeatID) []Action` in the same file, and have the non-window paths call it (as shown). The helper body is the current заход/handless/бой logic verbatim (already mode-gated for Дама♥ from Task 2).

> This is the §15 "модельный сдвиг": `LegalActions` now yields moves to seats other than `s.Turn` when a window/gate is open. Later tasks extend the two early branches (payment gate, social actions) — keep them at the top so gates take precedence over ordinary turn logic.

- [ ] **Step 5: Add `isLegal` routing, the `Unsettled` set/settle, the `ClaimShukh` case, and the assess helpers**

In `engine/apply.go`, replace the legality check at the top of `Apply`. Change:

```go
	turn := s.Turn
	if !slices.Contains(LegalActions(s, turn), a) {
		return s, nil, &IllegalAction{Code: "illegal_action", Rule: "§5"}
	}
	ns := s.clone()
	var events []Event

	switch act := a.(type) {
	case PlayCard:
```

to:

```go
	if !isLegal(s, a) {
		return s, nil, &IllegalAction{Code: "illegal_action", Rule: "§5"}
	}
	turn := s.Turn
	ns := s.clone()
	var events []Event

	// A non-claim action taken while a catch-window is open settles it (R-1.4.1):
	// the offending action «прижилось» and stays. ClaimShukh instead reverses it.
	if _, isClaim := a.(ClaimShukh); !isClaim && ns.Unsettled != nil {
		ns.Unsettled = nil
	}

	switch act := a.(type) {
	case PlayCard:
```

Add `isLegal` (P-1 routing) to `engine/apply.go`:

```go
// isLegal validates a against LegalActions for the seat responsible for it (P-1).
// Turn-actions check s.Turn; actor-carrying actions check their seat; the payer
// action checks the current payer; actor-agnostic social actions are validated by
// precondition (their legality does not depend on who raises them).
func isLegal(s State, a Action) bool {
	switch act := a.(type) {
	case ClaimShukh:
		return s.Unsettled != nil && s.Unsettled.Seat == act.Target && s.Unsettled.Code == act.Code
	default:
		return slices.Contains(LegalActions(s, s.Turn), a)
	}
}
```

> Task 5+ extend `isLegal` with the `GiveShukhCard`/`TakeShukhCard`/`DeclareOneCard`/`AskCount`/`AskAboutWest` cases. For now every non-claim action routes through `s.Turn`.

In the `PlayCard` case, detect the Middle Дама♥ заход and open the window instead of passing the turn normally. Change the `wasEmpty` arm:

```go
		if wasEmpty {
			// Заход: never closes (threshold ≥ 2); pass the turn.
			ns.settleTurn(ns.nextLive(turn), &events)
		} else if IsQueenHearts(act.Card) || len(ns.Table) == ns.liveCount() {
```

to:

```go
		if wasEmpty {
			if IsQueenHearts(act.Card) {
				// Middle Дама♥ заход (R-3.7.2): allowed but нелегально → open the
				// Ш-2 catch-window over the pre-action snapshot (§15.3) and pass the
				// turn so the next player can settle it (or someone may claim).
				ns.Unsettled = &Unsettled{Prev: s, Seat: turn, Code: Sh2}
			}
			ns.settleTurn(ns.nextLive(turn), &events)
		} else if IsQueenHearts(act.Card) || len(ns.Table) == ns.liveCount() {
```

> `s` (the pure input) is the exact pre-action snapshot; storing it in `Unsettled.Prev` makes reverse a one-line restore. Only the *empty-con* (заход) Дама♥ reaches here; a Дама♥ **бой** still closes immediately via the `else if` (legal — R-3.7.1).

Add the `ClaimShukh` case to the `switch` (after `PodkladkaWest`, before `default`):

```go
	case ClaimShukh:
		// Reverse: restore the pre-action snapshot (§15.3), then assess the ШУХ.
		ns = s.Unsettled.Prev.clone()
		events = append(events, ActionReverted{Seat: s.Unsettled.Seat})
		ns.assessShukh(s.Unsettled.Seat, s.Unsettled.Code, s.Unsettled.Code == Sh2, false, &events)
```

Add the assess helpers to `engine/apply.go`:

```go
// assessShukh confirms a ШУХ against offender (§8, R-8.5): it emits ShukhAssessed,
// then either opens a payment gate (obligated givers exist) or applies the effect
// immediately (nobody owes). skip requests the offender's turn-skip (Ш-2/Ш-12);
// thenDiscardWest sets the Ш-12 6(2)♥ obligation.
func (s *State) assessShukh(offender SeatID, code ShukhCode, skip, thenDiscardWest bool, events *[]Event) {
	*events = append(*events, ShukhAssessed{Offender: offender, Code: code})
	// Task 5 opens the payment gate here; for now apply the effect directly.
	s.applyShukhEffect(offender, skip, thenDiscardWest, events)
}

// applyShukhEffect applies a confirmed ШУХ's non-payment consequences (R-8.5):
// mark the offender's Shukh pile takeable if the con is already over (P-4), skip
// the offender's turn if required (Ш-2/Ш-12), and record the 6(2)♥ obligation
// (Ш-12). The corrected position is then live for play.
func (s *State) applyShukhEffect(offender SeatID, skip, thenDiscardWest bool, events *[]Event) {
	if len(s.Table) == 0 {
		s.markShukhTakeable()
	}
	if thenDiscardWest {
		s.Endgame.MustDiscard = true
	}
	if skip {
		s.settleTurn(s.nextLive(offender), events)
	}
}

// markShukhTakeable makes every non-empty Shukh pile takeable (R-8.3): called when
// the con that held the ШУХ ends — i.e. the table is (or has just become) empty
// (P-4).
func (s *State) markShukhTakeable() {
	for seat, pile := range s.Shukh {
		if len(pile) > 0 {
			s.ShukhTakeable[seat] = true
		}
	}
}
```

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS (Ш-2 set/settle/reverse; rejection tests; all prior green). The iteration-3 Guard fuzz is unaffected (Guard never sets `Unsettled`).

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/event.go engine/legal.go engine/apply.go engine/shukh_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Ш-2 catch-window — set Unsettled, settle-by-move, ClaimShukh reverse

Middle Дама♥ заход opens an Unsettled window over the pre-action snapshot; the
next player's move settles it (R-1.4.1) or ClaimShukh reverses it and assesses
Ш-2 (skip). LegalActions now yields ClaimShukh to non-offender seats (§15 model
shift). Payment gate lands in the next task.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

# SUB-ITERATION 2 — §8 payment gate + `TakeShukhCards` + Ш-3

### Task 5: Payment gate — `assessShukh` opens `Pending`, `GiveShukhCard`, effect after last payment

**Files:**
- Modify: `engine/action.go` (`GiveShukhCard`)
- Modify: `engine/event.go` (`ShukhPaid`)
- Modify: `engine/legal.go` (payment branch)
- Modify: `engine/apply.go` (`assessShukh` opens the gate; `GiveShukhCard` case; `isLegal` payer routing)
- Test: `engine/shukh_test.go`

**Interfaces:**
- Consumes: `assessShukh`, `applyShukhEffect`, `seatsFrom`, `removeCard`, `nextLive`.
- Produces: `GiveShukhCard{Card Card}`; event `ShukhPaid{Offender, From SeatID; Card Card}`; `func (s State) owedGivers(offender SeatID) []SeatID`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/shukh_test.go`:

```go
func TestApplyShukhPaymentGate(t *testing.T) {
	// 3 live. Seat 0 commits Ш-2 (Дама♥ заход) and is caught. Owed givers are the
	// live seats ≠ 0 with ≥2 cards, clockwise from 0: seat 1 (2 cards) and seat 2
	// (2 cards). Each gives one non-last card into seat 0's Shukh zone; then the
	// skip applies (turn → seat 1).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}, {Clubs, 9}},
		2: {{Spades, 10}, {Spades, 11}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Hearts, Queen}})
	require.NoError(t, err)
	ns, _, err = Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)

	// Payment gate open: only seat 1 (head of Owed) may give, only non-last cards.
	require.NotNil(t, ns.Pending)
	require.Equal(t, []SeatID{1, 2}, ns.Pending.Owed)
	require.ElementsMatch(t, []Action{
		GiveShukhCard{Card{Clubs, 8}}, GiveShukhCard{Card{Clubs, 9}},
	}, LegalActions(ns, 1))
	require.Nil(t, LegalActions(ns, 0)) // offender does not pay
	require.Nil(t, LegalActions(ns, 2)) // not the head payer yet

	// Seat 1 pays 8♣.
	ns, ev1, err := Apply(ns, GiveShukhCard{Card{Clubs, 8}})
	require.NoError(t, err)
	require.Contains(t, ev1, ShukhPaid{Offender: 0, From: 1, Card: Card{Clubs, 8}})
	require.Equal(t, []SeatID{2}, ns.Pending.Owed)
	require.Equal(t, []Action{GiveShukhCard{Card{Spades, 10}}, GiveShukhCard{Card{Spades, 11}}}, LegalActions(ns, 2))

	// Seat 2 pays 10♠ → gate closes, effect (skip) applies.
	ns, _, err = Apply(ns, GiveShukhCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Nil(t, ns.Pending)
	require.ElementsMatch(t, []Card{{Clubs, 8}, {Spades, 10}}, ns.Shukh[0]) // I-3: in Shukh, not hand
	require.ElementsMatch(t, []Card{{Hearts, Queen}}, ns.Hands[0])          // hand unchanged by payment
	require.Equal(t, SeatID(1), ns.Turn)                                    // Ш-2 skip past seat 0
	require.True(t, ns.ShukhTakeable[0])                                    // con already over (empty table)
	// (No CheckInvariants — partial-deck unit state does not satisfy I-1; I-2/I-3
	// are asserted structurally above, ns.Shukh[0] holds the paid cards, hands hold
	// the rest. Full I-1/I-3 conservation across the Shukh zone is covered by fuzz.)
}

func TestApplyShukhOneCardPlayerDoesNotPay(t *testing.T) {
	// I-2 (R-8.1.1): a player holding exactly one card never pays. Seat 1 has 1
	// card → excluded from Owed; only seat 2 pays.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}},
		2: {{Spades, 10}, {Spades, 11}},
	}, nil, 0)
	ns, _, _ := Apply(s, PlayCard{Card{Hearts, Queen}})
	ns, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Equal(t, []SeatID{2}, ns.Pending.Owed) // seat 1 (1 card) excluded
}

func TestApplyShukhNobodyOwesAppliesImmediately(t *testing.T) {
	// 2 live, opponent has 1 card → nobody owes → effect applies with no gate.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, Queen}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, _ := Apply(s, PlayCard{Card{Hearts, Queen}})
	ns, _, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh2})
	require.NoError(t, err)
	require.Nil(t, ns.Pending)
	require.Empty(t, ns.Shukh[0])
	require.Equal(t, SeatID(1), ns.Turn) // skip applied directly
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestApplyShukhPayment|TestApplyShukhOneCard|TestApplyShukhNobody' -v`
Expected: BUILD FAIL — `undefined: GiveShukhCard`, `undefined: ShukhPaid`.

- [ ] **Step 3: Add the `GiveShukhCard` action and `ShukhPaid` event**

In `engine/action.go`:

```go
// GiveShukhCard pays one card into the offender's Shukh zone during a §8 payment
// gate. It applies as the current payer, State.Pending.Owed[0] (P-1/P-3); Card
// must be one of that payer's non-last cards (R-8.1.1/I-2).
type GiveShukhCard struct{ Card Card }

func (GiveShukhCard) isAction() {}
```

In `engine/event.go`:

```go
// ShukhPaid is emitted when a payer gives one card into the offender's Shukh zone
// (R-8.1/R-8.2); From is the giver.
type ShukhPaid struct {
	Offender SeatID
	From     SeatID
	Card     Card
}

func (ShukhPaid) isEvent() {}
```

- [ ] **Step 4: Open the gate in `assessShukh`; add `owedGivers`**

In `engine/apply.go`, change `assessShukh` so it opens the gate when givers exist:

```go
func (s *State) assessShukh(offender SeatID, code ShukhCode, skip, thenDiscardWest bool, events *[]Event) {
	*events = append(*events, ShukhAssessed{Offender: offender, Code: code})
	owed := s.owedGivers(offender)
	if len(owed) == 0 {
		s.applyShukhEffect(offender, skip, thenDiscardWest, events)
		return
	}
	s.Pending = &Payment{Offender: offender, Owed: owed, Skip: skip, ThenDiscardWest: thenDiscardWest}
}

// owedGivers lists the seats obligated to pay the offender, clockwise from him:
// live, not the offender, holding ≥2 cards (R-8.1/R-8.1.1/I-2 — the last card is
// never given, a 1-card player does not pay).
func (s State) owedGivers(offender SeatID) []SeatID {
	var owed []SeatID
	for _, seat := range s.seatsFrom(offender) {
		if seat != offender && s.Live[seat] && len(s.Hands[seat]) >= 2 {
			owed = append(owed, seat)
		}
	}
	return owed
}
```

- [ ] **Step 5: Add the payment branch to `LegalActions` and the `GiveShukhCard` case + `isLegal` routing**

In `engine/legal.go`, at the top of `LegalActions` (before the `Unsettled` branch — a payment gate is never open at the same time as a window, but place payment first so it is unambiguous), add:

```go
	// A §8 payment gate: only the head payer acts, offering each of his non-last
	// cards (R-8.1.1/I-2). All other seats have nothing to do until it closes.
	if s.Pending != nil {
		if len(s.Pending.Owed) == 0 || seat != s.Pending.Owed[0] {
			return nil
		}
		hand := s.Hands[seat]
		if len(hand) < 2 {
			return nil // cannot give the last card
		}
		out := make([]Action, 0, len(hand))
		for _, c := range hand {
			out = append(out, GiveShukhCard{Card: c})
		}
		return out
	}
```

In `engine/apply.go`, extend `isLegal`:

```go
	case GiveShukhCard:
		if s.Pending == nil || len(s.Pending.Owed) == 0 {
			return false
		}
		return slices.Contains(LegalActions(s, s.Pending.Owed[0]), a)
```

Add the `GiveShukhCard` case to the `Apply` switch (before `default`):

```go
	case GiveShukhCard:
		p := ns.Pending
		giver := p.Owed[0]
		ns.Hands[giver] = removeCard(ns.Hands[giver], act.Card)
		ns.Shukh[p.Offender] = append(ns.Shukh[p.Offender], act.Card) // I-3: Shukh, not hand
		events = append(events, ShukhPaid{Offender: p.Offender, From: giver, Card: act.Card})
		p.Owed = p.Owed[1:]
		if len(p.Owed) == 0 {
			ns.applyShukhEffect(p.Offender, p.Skip, p.ThenDiscardWest, &events)
			ns.Pending = nil
		}
```

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS (payment gate, I-2 exclusion, no-owe immediate).

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/event.go engine/legal.go engine/apply.go engine/shukh_test.go
git commit -m "$(cat <<'EOF'
feat(engine): §8 payment gate — GiveShukhCard into the Shukh zone (I-2/I-3)

A confirmed ШУХ opens a serialized Pending gate; each obligated giver (live,
≠offender, ≥2 cards) picks a non-last card into the offender's Shukh zone; the
ШУХ effect (skip) applies after the last payment.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: `TakeShukhCards` — lift the Shukh pile after the con (R-8.3), `markShukhTakeable` on table-empty

**Files:**
- Modify: `engine/action.go` (`TakeShukhCards`)
- Modify: `engine/event.go` (`ShukhCardsTaken`)
- Modify: `engine/legal.go` (offer `TakeShukhCards`)
- Modify: `engine/apply.go` (`TakeShukhCards` case; `isLegal` seat routing; call `markShukhTakeable` on table-emptying transitions)
- Test: `engine/shukh_test.go`

**Interfaces:**
- Consumes: `markShukhTakeable`, `closeCon`, existing take/западло branches.
- Produces: `TakeShukhCards{Seat SeatID}`; event `ShukhCardsTaken{Seat SeatID; Cards []Card}`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/shukh_test.go`:

```go
func TestApplyTakeShukhCardsWhenTakeable(t *testing.T) {
	// Seat 0 has a takeable Shukh pile → lifts it into hand (R-8.3). It is a
	// social action: it does not change whose turn it is.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 1)
	s.Shukh[0] = []Card{{Clubs, 9}, {Diamonds, 10}}
	s.ShukhTakeable[0] = true

	ns, events, err := Apply(s, TakeShukhCards{Seat: 0})
	require.NoError(t, err)
	require.ElementsMatch(t, []Card{{Spades, 7}, {Clubs, 9}, {Diamonds, 10}}, ns.Hands[0])
	require.Empty(t, ns.Shukh[0])
	require.False(t, ns.ShukhTakeable[0])
	require.Equal(t, SeatID(1), ns.Turn) // unchanged
	require.Contains(t, events, ShukhCardsTaken{Seat: 0, Cards: []Card{{Clubs, 9}, {Diamonds, 10}}})
}

func TestCloseConMarksShukhTakeable(t *testing.T) {
	// A Shukh pile laid during an open con becomes takeable when that con closes
	// (P-4). 2 live, threshold 2: seat 0 beats 8♠ with 10♠ → close → mark.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 10}, {Diamonds, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 8}, By: 1}}, 0)
	s.Shukh[1] = []Card{{Clubs, 9}}
	require.False(t, s.ShukhTakeable[1])

	ns, _, err := Apply(s, PlayCard{Card{Spades, 10}})
	require.NoError(t, err)
	require.Empty(t, ns.Table)
	require.True(t, ns.ShukhTakeable[1])
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestApplyTakeShukhCards|TestCloseConMarks' -v`
Expected: BUILD FAIL — `undefined: TakeShukhCards`, `undefined: ShukhCardsTaken`; and (before the mark call) `ShukhTakeable[1]` stays false.

- [ ] **Step 3: Add the action and event**

In `engine/action.go`:

```go
// TakeShukhCards lifts Seat's set-aside Shukh pile into his hand (R-8.3), allowed
// only once the con it was laid in has ended (State.ShukhTakeable[Seat]). Taking
// it early is Ш-3 (Task 7). Carries the actor seat (P-1); a player takes only his
// own pile.
type TakeShukhCards struct{ Seat SeatID }

func (TakeShukhCards) isAction() {}
```

In `engine/event.go`:

```go
// ShukhCardsTaken is emitted when a player lifts his Shukh pile into hand (R-8.3).
type ShukhCardsTaken struct {
	Seat  SeatID
	Cards []Card
}

func (ShukhCardsTaken) isEvent() {}
```

- [ ] **Step 4: Offer `TakeShukhCards` in `LegalActions`**

In `engine/legal.go`, in the normal (gates-closed) region — i.e. add to the `turnActions` result for the acting seat is wrong (it is out-of-turn). Instead add it as a social action available regardless of turn. Extend the **non-window, non-payment** section of `LegalActions`: after the `if seat != s.Turn { return nil }` check would drop non-turn seats, so add a social pre-check. Replace the tail of `LegalActions`:

```go
	if seat != s.Turn {
		return nil
	}
	return turnActions(s, seat)
```

with:

```go
	// Social actions available out of turn (gates closed).
	var social []Action
	if len(s.Shukh[seat]) > 0 && (s.ShukhTakeable[seat] || s.Mode == Middle) {
		// Guard: only when takeable. Middle: also offered early — an early take is
		// allowed and caught as Ш-3 (§15.4).
		social = append(social, TakeShukhCards{Seat: seat})
	}
	if seat != s.Turn {
		return social
	}
	return append(turnActions(s, seat), social...)
```

- [ ] **Step 5: Add the `TakeShukhCards` case, `isLegal` routing, and `markShukhTakeable` calls**

In `engine/apply.go`, extend `isLegal`:

```go
	case TakeShukhCards:
		return slices.Contains(LegalActions(s, act.Seat), a)
```

Add the case to the `Apply` switch (this task handles the **takeable** path; Task 7 adds the early-take Ш-3 branch):

```go
	case TakeShukhCards:
		taken := ns.Shukh[act.Seat]
		ns.Shukh[act.Seat] = nil
		ns.ShukhTakeable[act.Seat] = false
		ns.Hands[act.Seat] = append(ns.Hands[act.Seat], taken...)
		events = append(events, ShukhCardsTaken{Seat: act.Seat, Cards: taken})
```

Call `markShukhTakeable` wherever the table transitions to empty (P-4). In `closeCon`, after `s.Table = nil` and the sweep events, add `s.markShukhTakeable()`:

```go
	s.Discard = append(s.Discard, swept...)
	s.Table = nil
	*events = append(*events, ConClosed{By: closer}, ConSwept{Cards: swept})
	s.markShukhTakeable() // P-4: the con that held any ШУХ has ended
```

In the `PodkladkaWest` case, after `ns.Table = nil`, add `ns.markShukhTakeable()`. In the `TakeBottomAndPass` case, after `ns.Table = ns.Table[1:]`, add: `if len(ns.Table) == 0 { ns.markShukhTakeable() }`.

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/event.go engine/legal.go engine/apply.go engine/shukh_test.go
git commit -m "$(cat <<'EOF'
feat(engine): TakeShukhCards after the con (R-8.3) + takeable marking (P-4)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Ш-3 — early `TakeShukhCards` in Middle opens a window; Guard blocks

**Files:**
- Modify: `engine/apply.go` (`TakeShukhCards` early-take branch)
- Test: `engine/shukh_test.go`

**Interfaces:**
- Consumes: `Unsettled`, `assessShukh`, existing `TakeShukhCards` case.
- Produces: no new symbols — Ш-3 behavior on the existing action.

- [ ] **Step 1: Write the failing tests**

Add to `engine/shukh_test.go`:

```go
func TestApplyEarlyTakeGuardBlocked(t *testing.T) {
	// Guard: an untakeable Shukh pile is not offered, so an early take is rejected.
	s := playing(map[SeatID][]Card{0: {{Spades, 7}}, 1: {{Clubs, 8}}}, nil, 1)
	s.Shukh[0] = []Card{{Clubs, 9}}
	s.ShukhTakeable[0] = false
	_, _, err := Apply(s, TakeShukhCards{Seat: 0})
	require.Error(t, err)
}

func TestApplyEarlyTakeMiddleSetsUnsettledSh3(t *testing.T) {
	// Middle: an early take is allowed but нелегально. It moves the cards to hand
	// and opens a Ш-3 window over the snapshot; a claim reverses it (cards back in
	// the Shukh zone) and assesses Ш-3 (no extra effect). Con is open (7♠ on the
	// table), so nothing settles automatically.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}, {Spades, 6}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, []TableCard{{Card: Card{Spades, 5}, By: 1}}, 0)
	s.Shukh[0] = []Card{{Diamonds, 10}}
	s.ShukhTakeable[0] = false

	ns, _, err := Apply(s, TakeShukhCards{Seat: 0})
	require.NoError(t, err)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, Sh3, ns.Unsettled.Code)
	require.ElementsMatch(t, []Card{{Spades, 7}, {Spades, 6}, {Diamonds, 10}}, ns.Hands[0])

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh3})
	require.NoError(t, err)
	require.Nil(t, ns2.Unsettled)
	require.ElementsMatch(t, []Card{{Diamonds, 10}}, ns2.Shukh[0]) // reversed back
	require.ElementsMatch(t, []Card{{Spades, 7}, {Spades, 6}}, ns2.Hands[0])
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh3})
	// Ш-3 has no extra effect (no skip); seat 1 pays into seat 0's Shukh (2 cards).
	require.NotNil(t, ns2.Pending)
	require.Equal(t, []SeatID{1}, ns2.Pending.Owed)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestApplyEarlyTake' -v`
Expected: FAIL — the current `TakeShukhCards` case unconditionally takes (no window) even when `!ShukhTakeable`.

- [ ] **Step 3: Branch the `TakeShukhCards` case on takeability**

In `engine/apply.go`, replace the `TakeShukhCards` case body with:

```go
	case TakeShukhCards:
		if !ns.ShukhTakeable[act.Seat] {
			// Middle early take (LegalActions only offers this in Middle when not yet
			// takeable): allowed but нелегально → open the Ш-3 window over the
			// snapshot, then take the cards (the offense «прижилось» unless claimed).
			ns.Unsettled = &Unsettled{Prev: s, Seat: act.Seat, Code: Sh3}
		}
		taken := ns.Shukh[act.Seat]
		ns.Shukh[act.Seat] = nil
		ns.ShukhTakeable[act.Seat] = false
		ns.Hands[act.Seat] = append(ns.Hands[act.Seat], taken...)
		events = append(events, ShukhCardsTaken{Seat: act.Seat, Cards: taken})
```

> The `ns.Unsettled = nil` settle-guard at the top of `Apply` does not fire here because this *is* the offending action (it sets the window after); the guard only clears a window left open by a *previous* action. Since `TakeShukhCards` is only offered when `gatesClosed()` (it is in the social section, reached only when `s.Pending == nil` and `s.Unsettled == nil`), there is never a pre-existing window to clear on entry.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/apply.go engine/shukh_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Ш-3 — early TakeShukhCards opens a Middle window; Guard blocks (R-8.3)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

# SUB-ITERATION 3 — «Одна карта» §6 (Ш-11)

### Task 8: `OwesOneCard` reconciliation + `DeclareOneCard`

**Files:**
- Modify: `engine/action.go` (`DeclareOneCard`)
- Modify: `engine/event.go` (`OneCardDeclared`)
- Modify: `engine/legal.go` (offer `DeclareOneCard`)
- Modify: `engine/apply.go` (`reconcileOneCard`; capture pre-action hand sizes; `DeclareOneCard` case; `isLegal` routing)
- Test: `engine/onecard_test.go`

**Interfaces:**
- Consumes: `gatesClosed`.
- Produces: `DeclareOneCard{Seat SeatID}`; event `OneCardDeclared{Seat SeatID}`; `func (s *State) reconcileOneCard(before map[SeatID]int)`.

- [ ] **Step 1: Write the failing tests**

Create `engine/onecard_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwesOneCardSetWhenHandBecomesOne(t *testing.T) {
	// Seat 0 plays down to one card (заход leaves 1) → OwesOneCard set (R-6.1a).
	s := playing(map[SeatID][]Card{
		0: {{Spades, 7}, {Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 0)
	ns, _, err := Apply(s, PlayCard{Card{Spades, 7}})
	require.NoError(t, err)
	require.True(t, ns.OwesOneCard[0])
	require.False(t, ns.OwesOneCard[1])
}

func TestOwesOneCardClearedByMove(t *testing.T) {
	// R-6.3 «успел походить» falls out for free: playing the 1-card hand drops to
	// 0 → hand != 1 → flag auto-clears. (Here seat 0 takes a card, going to 2.)
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Spades, 7}, By: 1}}, 0)
	s.OwesOneCard[0] = true
	ns, _, err := Apply(s, TakeBottomAndPass{})
	require.NoError(t, err)
	require.False(t, ns.OwesOneCard[0]) // now 2 cards
}

func TestDeclareOneCardClearsFlag(t *testing.T) {
	// Declaring clears the obligation; the flag stays clear while the hand is 1.
	s := playing(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}},
	}, nil, 1)
	s.OwesOneCard[0] = true
	require.Equal(t, []Action{DeclareOneCard{Seat: 0}}, LegalActions(s, 0))

	ns, events, err := Apply(s, DeclareOneCard{Seat: 0})
	require.NoError(t, err)
	require.False(t, ns.OwesOneCard[0])
	require.Contains(t, events, OneCardDeclared{Seat: 0})
	require.Nil(t, LegalActions(ns, 0)) // nothing more to declare; not its turn
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestOwesOneCard|TestDeclareOneCard' -v`
Expected: BUILD FAIL — `undefined: DeclareOneCard`, `undefined: OneCardDeclared`; reconciliation not wired.

- [ ] **Step 3: Add the action and event**

In `engine/action.go`:

```go
// DeclareOneCard announces «Одна карта!» for Seat, clearing its one-card
// obligation (R-6.1). Out of turn; carries the actor seat (P-1).
type DeclareOneCard struct{ Seat SeatID }

func (DeclareOneCard) isAction() {}
```

In `engine/event.go`:

```go
// OneCardDeclared is emitted when a player announces «Одна карта!» (R-6.1).
type OneCardDeclared struct {
	Seat SeatID
}

func (OneCardDeclared) isEvent() {}
```

- [ ] **Step 4: Add `reconcileOneCard` and wire pre-action hand sizes**

In `engine/apply.go`, add:

```go
// reconcileOneCard updates OwesOneCard by hand-size transition (§15.6): a seat
// crossing INTO exactly one card owes a declaration (R-6.1); a seat leaving one
// card clears it (R-6.3 «успел походить» — any move away from 1 auto-clears);
// staying at one card is left as-is so a prior DeclareOneCard sticks. `before`
// maps each seat to its pre-action hand size.
func (s *State) reconcileOneCard(before map[SeatID]int) {
	for seat := range s.Hands {
		now := len(s.Hands[seat])
		switch {
		case now == 1 && before[seat] != 1:
			s.OwesOneCard[seat] = true
		case now != 1:
			s.OwesOneCard[seat] = false
		}
	}
}
```

Add a `handSizes` helper (used for both the top-of-`Apply` capture and the `ClaimShukh` re-capture below):

```go
// handSizes snapshots each seat's hand size — the basis reconcileOneCard compares
// against to detect a transition into/out of exactly one card (§15.6).
func handSizes(hands map[SeatID][]Card) map[SeatID]int {
	m := make(map[SeatID]int, len(hands))
	for seat, h := range hands {
		m[seat] = len(h)
	}
	return m
}
```

Capture the pre-action sizes and reconcile at the end of `Apply`. Just after `ns := s.clone()` (and the settle-guard), add:

```go
	before := handSizes(s.Hands)
```

And just before `return ns, events, nil`, add:

```go
	ns.reconcileOneCard(before)
```

> **`ClaimShukh` must re-base `before`.** `reconcileOneCard` runs for **every** action, so a payment that drops a giver to one card (§15.4) also sets its flag. But `ClaimShukh` replaces `ns` with the pre-offense snapshot (`ns = s.Unsettled.Prev.clone()`), whose hand sizes differ from the post-offense `s.Hands` for the **offender's own seat** (the reversed offense changed that hand — e.g. a lone `Дама♥` заход took the offender from 1 card to 0). Reconciling the snapshot against the post-offense `before` would then spuriously flip the offender's `OwesOneCard` to true, clobbering the correct (possibly already-declared) flag the snapshot restored. Since the claim itself changes no hand size, the fix is to re-base `before` from the restored snapshot so the reconcile is a no-op. In the `ClaimShukh` case (Task 4), after `ns.assessShukh(...)`, add:
>
> ```go
> 		before = handSizes(ns.Hands) // reconcile against the restored snapshot, not the post-offense sizes
> ```
>
> (Task 8 introduces `reconcileOneCard`/`before`; this `ClaimShukh` line is added in the same task, alongside the `handSizes` helper.)

- [ ] **Step 5: Offer `DeclareOneCard`; add the case and `isLegal` routing**

In `engine/legal.go`, in the social section (Task 6), add before/after the `TakeShukhCards` offer:

```go
	if s.OwesOneCard[seat] {
		social = append(social, DeclareOneCard{Seat: seat})
	}
```

In `engine/apply.go`, extend `isLegal`:

```go
	case DeclareOneCard:
		return slices.Contains(LegalActions(s, act.Seat), a)
```

Add the `Apply` case (before `default`):

```go
	case DeclareOneCard:
		ns.OwesOneCard[act.Seat] = false
		events = append(events, OneCardDeclared{Seat: act.Seat})
```

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/event.go engine/legal.go engine/apply.go engine/onecard_test.go
git commit -m "$(cat <<'EOF'
feat(engine): «одна карта» — OwesOneCard reconciliation + DeclareOneCard (§6)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: `AskCount` → Ш-11 (auto-judged)

**Files:**
- Modify: `engine/action.go` (`AskCount`)
- Modify: `engine/legal.go` (offer `AskCount` when target owes)
- Modify: `engine/apply.go` (`AskCount` case; `isLegal` routing)
- Test: `engine/onecard_test.go`

**Interfaces:**
- Consumes: `assessShukh`, `OwesOneCard`, `gatesClosed`.
- Produces: `AskCount{Target SeatID}`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/onecard_test.go`:

```go
func TestAskCountAssessesSh11(t *testing.T) {
	// Seat 0 owes «одна карта» and hasn't declared → asking triggers Ш-11 (R-6.2).
	// The other players pay into seat 0's Shukh zone (no skip for Ш-11).
	s := middle(map[SeatID][]Card{
		0: {{Diamonds, 9}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.OwesOneCard[0] = true
	require.Contains(t, LegalActions(s, 1), AskCount{Target: 0})

	ns, events, err := Apply(s, AskCount{Target: 0})
	require.NoError(t, err)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh11})
	require.NotNil(t, ns.Pending)
	require.Equal(t, []SeatID{1}, ns.Pending.Owed)
	require.False(t, ns.Pending.Skip) // Ш-11 has no turn-skip
}

func TestAskCountRejectedWhenNoObligation(t *testing.T) {
	// No obligation → false trigger → rejected (its Ш-8/Ш-9 punishment is Спец 2).
	s := middle(map[SeatID][]Card{0: {{Diamonds, 9}, {Spades, 7}}, 1: {{Clubs, 8}}}, nil, 0)
	_, _, err := Apply(s, AskCount{Target: 0})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run TestAskCount -v`
Expected: BUILD FAIL — `undefined: AskCount`.

- [ ] **Step 3: Add the action**

In `engine/action.go`:

```go
// AskCount asks Target «Сколько карт?». If Target owes an undeclared «одна карта»
// (R-6.2) it assesses Ш-11. Actor-agnostic (P-1) — validated by the target's
// obligation, not by who asks.
type AskCount struct{ Target SeatID }

func (AskCount) isAction() {}
```

- [ ] **Step 4: Offer `AskCount`; add the case and routing**

In `engine/legal.go`, in the social section, add (any seat may ask a seat other than itself that owes):

```go
	for _, t := range s.Seats {
		if t != seat && s.OwesOneCard[t] {
			social = append(social, AskCount{Target: t})
		}
	}
```

In `engine/apply.go`, extend `isLegal`:

```go
	case AskCount:
		return s.gatesClosed() && s.OwesOneCard[act.Target]
```

Add the `Apply` case (before `default`):

```go
	case AskCount:
		ns.assessShukh(act.Target, Sh11, false, false, &events) // R-6.2, no extra effect
```

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/action.go engine/legal.go engine/apply.go engine/onecard_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Ш-11 — AskCount auto-judges «одна карта» (R-6.2)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

# SUB-ITERATION 4 — Endgame §9.2 (Ш-12)

### Task 10: Endgame detection + `DiscardWest`

**Files:**
- Modify: `engine/action.go` (`DiscardWest`)
- Modify: `engine/event.go` (`WestDiscarded`)
- Modify: `engine/apply.go` (`resolveExits` sets `Endgame.Active`; `DiscardWest` case)
- Modify: `engine/legal.go` (endgame заход branch: offer `DiscardWest`, forbid западло of `6(2)♥`, block Guard 6(2)♥ заход, obligation)
- Test: `engine/endgame_test.go`

**Interfaces:**
- Consumes: `IsLowestHeart` (`RuleSet`), `settleTurn`, `nextLive`.
- Produces: `DiscardWest{}`; event `WestDiscarded{Seat SeatID}`.

- [ ] **Step 1: Write the failing tests**

Create `engine/endgame_test.go`:

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndgameActivatesAtTwoLive(t *testing.T) {
	// 3 live, threshold 3. Seat 0 beats the third card and closes; seat 2 was
	// handless with its card in the con → exits → 2 live → endgame active.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 11}, {Diamonds, 6}},
		1: {{Clubs, 7}},
		2: {},
	}, []TableCard{
		{Card: Card{Spades, 8}, By: 2},
		{Card: Card{Spades, 10}, By: 1},
	}, 0)
	ns, _, err := Apply(s, PlayCard{Card{Spades, 11}})
	require.NoError(t, err)
	require.Equal(t, 2, ns.liveCount())
	require.True(t, ns.Endgame.Active)
}

func TestDiscardWestSendsSixHeartsToDiscard(t *testing.T) {
	// Endgame, seat 0 to open, holds 6♥ (6(2)♥) → DiscardWest sends it to отбой
	// (R-9.3) and passes the turn.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.Contains(t, LegalActions(s, 0), DiscardWest{})

	ns, events, err := Apply(s, DiscardWest{})
	require.NoError(t, err)
	require.Contains(t, ns.Discard, Card{Hearts, 6})
	require.ElementsMatch(t, []Card{{Spades, 7}}, ns.Hands[0])
	require.Equal(t, SeatID(1), ns.Turn)
	require.Contains(t, events, WestDiscarded{Seat: 0})
}

func TestEndgameForbidsWestPodkladka(t *testing.T) {
	// In the endgame 6(2)♥ must go to отбой, never under 7(3)♥ into a hand (R-9.4.3).
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}},
		1: {{Clubs, 8}},
	}, []TableCard{{Card: Card{Hearts, 7}, By: 1}}, 0)
	s.Endgame = EndgameState{Active: true}
	require.NotContains(t, LegalActions(s, 0), PodkladkaWest{})
}

func TestEndgameGuardBlocksSixHeartsZahod(t *testing.T) {
	// Guard: заход with 6(2)♥ in the endgame is «использование» (R-9.4.3) → blocked.
	s := playing(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.NotContains(t, LegalActions(s, 0), PlayCard{Card{Hearts, 6}})
	require.Contains(t, LegalActions(s, 0), DiscardWest{})
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestEndgame|TestDiscardWest' -v`
Expected: BUILD FAIL — `undefined: DiscardWest`, `undefined: WestDiscarded`; endgame not detected/gated.

- [ ] **Step 3: Add the action and event; detect endgame**

In `engine/action.go`:

```go
// DiscardWest sends 6(2)♥ to the discard in the two-player endgame (R-9.3). A
// turn-action for the holder at заход time; it passes the turn (P-5).
type DiscardWest struct{}

func (DiscardWest) isAction() {}
```

In `engine/event.go`:

```go
// WestDiscarded is emitted when 6(2)♥ is discarded in the endgame (R-9.3).
type WestDiscarded struct {
	Seat SeatID
}

func (WestDiscarded) isEvent() {}
```

In `engine/apply.go`, in `resolveExits`, after the exit loop and before/around the termination check, set the endgame flag once two players remain:

```go
	if s.liveCount() == 2 {
		s.Endgame.Active = true // §9.2: 6(2)♥ now подлежит сбросу (R-9.3)
	}
	if s.liveCount() <= 1 {
```

- [ ] **Step 4: Add the `DiscardWest` case and `isLegal` routing**

In `engine/apply.go`, `isLegal` needs no special case (it is a turn-action → routes through `s.Turn`). Add the `Apply` case (before `default`):

```go
	case DiscardWest:
		west := Card{Suit: Hearts, Rank: ns.Rules.LowestRank()} // 6(2)♥
		ns.Hands[turn] = removeCard(ns.Hands[turn], west)
		ns.Discard = append(ns.Discard, west)
		ns.Endgame.MustDiscard = false
		events = append(events, WestDiscarded{Seat: turn})
		ns.settleTurn(ns.nextLive(turn), &events)
```

- [ ] **Step 5: Gate the endgame заход in `LegalActions`**

In `engine/legal.go`, in `turnActions`, the заход branch (`len(s.Table) == 0`) must, in the endgame, (a) offer `DiscardWest` to the 6(2)♥ holder, (b) not offer the 6(2)♥ заход in Guard, and honor the post-Ш-12 obligation. Replace the заход branch body with:

```go
	if len(s.Table) == 0 {
		west := Card{Suit: Hearts, Rank: s.Rules.LowestRank()}
		holdsWest := slices.Contains(hand, west)
		// Post-Ш-12 obligation: the holder must DiscardWest before anything else.
		if s.Endgame.MustDiscard && holdsWest {
			return []Action{DiscardWest{}}
		}
		var out []Action
		for _, c := range hand {
			if IsQueenHearts(c) && s.Mode == Guard {
				continue // R-3.7.2 (§14.4)
			}
			if s.Endgame.Active && s.Rules.IsLowestHeart(c) && s.Mode == Guard {
				continue // R-9.4.3: 6(2)♥ заход is illegal use; blocked in Guard
			}
			out = append(out, PlayCard{Card: c})
		}
		if s.Endgame.Active && holdsWest {
			out = append(out, DiscardWest{}) // R-9.3
		}
		return out
	}
```

(The `slices` import is already present in `legal.go`.)

The западло forbid: in the бой branch of `turnActions`, gate the `PodkladkaWest` offer on `!s.Endgame.Active`:

```go
	if !s.Endgame.Active && s.Rules.IsSecondLowestHeart(s.Table[0].Card) && slices.ContainsFunc(hand, s.Rules.IsLowestHeart) {
		out = append(out, PodkladkaWest{}) // R-5.3c/R-3.6.2 — forbidden in the endgame (R-9.4.3)
	}
```

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/event.go engine/apply.go engine/legal.go engine/endgame_test.go
git commit -m "$(cat <<'EOF'
feat(engine): endgame §9.2 — activate at 2 live, DiscardWest, forbid west западло

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: `AskAboutWest` → Ш-12; Middle 6(2)♥ заход caught as Ш-12

**Files:**
- Modify: `engine/action.go` (`AskAboutWest`)
- Modify: `engine/legal.go` (offer `AskAboutWest`; Middle 6(2)♥ заход allowed)
- Modify: `engine/apply.go` (`AskAboutWest` case + `isLegal`; Middle 6(2)♥ заход opens `Unsettled{Sh12}`)
- Test: `engine/endgame_test.go`

**Interfaces:**
- Consumes: `assessShukh`, `IsLowestHeart`, `Endgame`, `Unsettled`.
- Produces: `AskAboutWest{Target SeatID}`.

- [ ] **Step 1: Write the failing tests**

Add to `engine/endgame_test.go`:

```go
func TestAskAboutWestAssessesSh12(t *testing.T) {
	// Endgame, unasked. Seat 1 asks seat 0, who still holds 6(2)♥ → Ш-12: skip +
	// obligation to discard (R-9.4.2/R-9.4.3). Asked becomes true.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	require.Contains(t, LegalActions(s, 1), AskAboutWest{Target: 0})

	ns, events, err := Apply(s, AskAboutWest{Target: 0})
	require.NoError(t, err)
	require.True(t, ns.Endgame.Asked)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh12})
	require.NotNil(t, ns.Pending)
	require.True(t, ns.Pending.Skip)
	require.True(t, ns.Pending.ThenDiscardWest)
}

func TestAskAboutWestNoShukhWhenAlreadyDiscarded(t *testing.T) {
	// Asked but seat 0 no longer holds 6(2)♥ (discarded earlier) → no ШУХ, just
	// closes the безнаказанно window.
	s := middle(map[SeatID][]Card{
		0: {{Spades, 7}},
		1: {{Clubs, 8}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	ns, events, err := Apply(s, AskAboutWest{Target: 0})
	require.NoError(t, err)
	require.True(t, ns.Endgame.Asked)
	require.Nil(t, ns.Pending)
	for _, e := range events {
		_, isAssessed := e.(ShukhAssessed)
		require.False(t, isAssessed)
	}
}

func TestMiddleSixHeartsZahodCaughtAsSh12(t *testing.T) {
	// Middle endgame: seat 0 заходит with 6(2)♥ («использование», R-9.4.3) → Ш-12
	// window. A claim reverses it (6(2)♥ back in hand), skips, and obligates the
	// discard.
	s := middle(map[SeatID][]Card{
		0: {{Hearts, 6}, {Spades, 7}},
		1: {{Clubs, 8}, {Clubs, 9}},
	}, nil, 0)
	s.Endgame = EndgameState{Active: true}
	ns, _, err := Apply(s, PlayCard{Card{Hearts, 6}})
	require.NoError(t, err)
	require.NotNil(t, ns.Unsettled)
	require.Equal(t, Sh12, ns.Unsettled.Code)

	ns2, events, err := Apply(ns, ClaimShukh{Target: 0, Code: Sh12})
	require.NoError(t, err)
	require.Contains(t, events, ShukhAssessed{Offender: 0, Code: Sh12})
	require.ElementsMatch(t, []Card{{Hearts, 6}, {Spades, 7}}, ns2.Hands[0]) // reversed
	require.True(t, ns2.Pending.ThenDiscardWest)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./engine/ -run 'TestAskAboutWest|TestMiddleSixHearts' -v`
Expected: BUILD FAIL — `undefined: AskAboutWest`; Middle 6(2)♥ заход does not open a window.

- [ ] **Step 3: Add the action**

In `engine/action.go`:

```go
// AskAboutWest asks whether Target holds 6(2)♥ in the endgame (R-9.4). It closes
// the безнаказанно window (Endgame.Asked) and, if Target still holds 6(2)♥,
// assesses Ш-12 (skip + discard obligation). Actor-agnostic (P-1).
type AskAboutWest struct{ Target SeatID }

func (AskAboutWest) isAction() {}
```

- [ ] **Step 4: Offer `AskAboutWest`; add the case and routing; Middle 6(2)♥ заход window**

In `engine/legal.go`, social section, add:

```go
	if s.Endgame.Active && !s.Endgame.Asked {
		for _, t := range s.Seats {
			if t != seat && s.Live[t] {
				social = append(social, AskAboutWest{Target: t})
			}
		}
	}
```

In `engine/legal.go`, `turnActions` заход branch: in Middle the 6(2)♥ заход must be **offered** (allowed-and-caught), so only skip it in Guard (the Task 10 edit already does `&& s.Mode == Guard`). Confirm that clause reads `if s.Endgame.Active && s.Rules.IsLowestHeart(c) && s.Mode == Guard { continue }` — Middle therefore lists `PlayCard{6(2)♥}`. No further change here.

In `engine/apply.go`, extend `isLegal`:

```go
	case AskAboutWest:
		return s.gatesClosed() && s.Endgame.Active && !s.Endgame.Asked && s.Live[act.Target] && act.Target != s.Turn
```

> `act.Target != s.Turn` is a light guard; in a 2-player endgame the asker is the non-target live seat. (The asker seat is not modeled; legality is by the window, P-1.)

Add the `AskAboutWest` case to `Apply` (before `default`):

```go
	case AskAboutWest:
		ns.Endgame.Asked = true
		west := Card{Suit: Hearts, Rank: ns.Rules.LowestRank()}
		if slices.Contains(ns.Hands[act.Target], west) {
			ns.assessShukh(act.Target, Sh12, true, true, &events) // R-9.4.2: skip + discard
		}
```

In the `PlayCard` case, the Middle 6(2)♥ заход opens a `Sh12` window (alongside the existing Дама♥ `Sh2` window). Change the `wasEmpty` arm added in Task 4 so it recognizes both illegal заходы:

```go
		if wasEmpty {
			switch {
			case IsQueenHearts(act.Card):
				// Middle Дама♥ заход (R-3.7.2) → Ш-2 window.
				ns.Unsettled = &Unsettled{Prev: s, Seat: turn, Code: Sh2}
			case ns.Endgame.Active && ns.Rules.IsLowestHeart(act.Card):
				// Middle endgame 6(2)♥ заход = «использование» (R-9.4.3) → Ш-12 window.
				ns.Unsettled = &Unsettled{Prev: s, Seat: turn, Code: Sh12}
			}
			ns.settleTurn(ns.nextLive(turn), &events)
		} else if IsQueenHearts(act.Card) || len(ns.Table) == ns.liveCount() {
```

> Only Guard blocks these заходы (Task 2/Task 10). In Middle they reach `Apply` as legal-to-attempt and open the appropriate window. The `Sh12` window's `ClaimShukh` routes through the same reverse path; `assessShukh(offender, Sh12, skip=true, thenDiscardWest=true)` runs because `Unsettled.Code == Sh12` — update the `ClaimShukh` case to pass the Ш-12 effect flags.

Update the `ClaimShukh` case in `engine/apply.go` to derive the effect flags from the code:

```go
	case ClaimShukh:
		u := s.Unsettled
		ns = u.Prev.clone()
		events = append(events, ActionReverted{Seat: u.Seat})
		skip := u.Code == Sh2 || u.Code == Sh12
		thenDiscard := u.Code == Sh12
		ns.assessShukh(u.Seat, u.Code, skip, thenDiscard, &events)
```

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./engine/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/action.go engine/legal.go engine/apply.go engine/endgame_test.go
git commit -m "$(cat <<'EOF'
feat(engine): Ш-12 — AskAboutWest + Middle 6(2)♥ заход caught (R-9.4)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

# SUB-ITERATION 5 — fuzz + full-suite verification

### Task 12: Middle fuzz driving social actions + full suite/vet

**Files:**
- Modify: `engine/fuzz_test.go` (add a Middle fuzz variant)
- Test: (run) full package + `go vet`

**Interfaces:**
- Consumes: `NewGame`, `LegalActions`, `Apply`, `CheckInvariants`, all iteration-4 actions/state.
- Produces: `TestFuzzMiddleGamesTerminate`.

**Design of the driver:** the existing Guard fuzz picks a random legal action for `s.Turn`. In Middle, legal actions also belong to **other seats** (claims, asks, gives, takes, declares). The driver therefore, each step, collects legal actions across **all seats**, dedups, and picks one at random — exercising catch/settle, the payment gate, `TakeShukhCards`, `DeclareOneCard`/`AskCount`, and the endgame. `CheckInvariants` runs after every `Apply` (it self-gates: only I-1 while `Unsettled != nil`). Termination is bounded; the final `Finish` must be a full ranking.

- [ ] **Step 1: Add the Middle fuzz test**

Add to `engine/fuzz_test.go`:

```go
// TestFuzzMiddleGamesTerminate plays N seeded random *legal* Middle games to the
// end. Unlike Guard, legal actions belong to several seats at once (claims, asks,
// payment, takes — §15 model shift), so the driver gathers legal actions across
// all seats and picks one at random. CheckInvariants self-gates (I-1 only during
// an open catch-window). Games must terminate with a valid ranking (§15.10).
func TestFuzzMiddleGamesTerminate(t *testing.T) {
	const (
		games    = 200
		maxSteps = 8000
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
		cfg := engine.Config{Rules: rs, Mode: engine.Middle, Players: players}
		deck := shuffle.Deck(engine.NewDeck(rs), seed)

		s, _, err := engine.NewGame(cfg, deck)
		require.NoErrorf(t, err, "seed %d: NewGame", seed)
		require.NoErrorf(t, engine.CheckInvariants(s), "seed %d: invariants after deal", seed)

		steps := 0
		for s.Phase != engine.Finished {
			require.Lessf(t, steps, maxSteps, "seed %d: game did not terminate in %d steps", seed, maxSteps)

			// Gather legal actions across all seats (dedup identical actions).
			type keyed struct {
				a engine.Action
			}
			var pool []engine.Action
			seen := map[string]bool{}
			for i := 0; i < np; i++ {
				for _, a := range engine.LegalActions(s, engine.SeatID(i)) {
					k := fmt.Sprintf("%T%v", a, a)
					if !seen[k] {
						seen[k] = true
						pool = append(pool, a)
					}
				}
			}
			require.NotEmptyf(t, pool, "seed %d step %d: no legal action for any seat", seed, steps)

			a := pool[rng.IntN(len(pool))]
			ns, _, err := engine.Apply(s, a)
			require.NoErrorf(t, err, "seed %d step %d: Apply(%T)", seed, steps, a)
			require.NoErrorf(t, engine.CheckInvariants(ns), "seed %d step %d: invariants after %T", seed, steps, a)
			s = ns
			steps++
		}

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

Add `"fmt"` to the `fuzz_test.go` import block. Remove the unused `keyed` type if the reviewer prefers — it is illustrative; the dedup uses the `fmt.Sprintf` key. (Simplify to a bare `map[string]bool` dedup with no helper type.)

- [ ] **Step 2: Run the Middle fuzz**

Run: `go test ./engine/ -run TestFuzzMiddleGamesTerminate -v`
Expected: PASS — 200 Middle games across decks 36/52 and 2/3/4/6 players terminate with valid rankings; no invariant violation.

> If a game exceeds `maxSteps`, suspect a no-progress loop (e.g. a social action that neither advances the turn nor changes a monotone quantity). Replay the reported `seed` in a focused test. A legitimate non-terminating cycle is a bug in the plan's action set, not an expected outcome — every social action must either advance play, move a card, or close a window/gate.

- [ ] **Step 3: Run the full package + vet**

Run: `go test ./... -count=1 && go vet ./...`
Expected: PASS, no vet complaints. (Use `-count=1` to bypass the test cache, per the known stale-LSP artifact noted in the iteration handoff.)

- [ ] **Step 4: Commit**

```bash
git add engine/fuzz_test.go
git commit -m "$(cat <<'EOF'
test(engine): Middle fuzz — random legal ШУХ games run to a valid Finish

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage (§15):**
- §15.1 scope — Ш-2 (Tasks 2–4), Ш-3 (Task 7), payment §8 (Tasks 5–6), «одна карта» Ш-11 (Tasks 8–9), endgame Ш-12 (Tasks 10–11). Out-of-scope items (Adjudication/Vote, R-9.5, Culture, Ш-4/Ш-5) are never introduced. ✅
- §15.2 new `State` fields + types → Task 1. ✅
- §15.3 catch model (snapshot reverse, settle-by-move, `CheckInvariants` gating) → Task 1 (gating) + Task 4 (window/reverse). ✅
- §15.4 payment (choose-card, I-2/I-3, `ShukhTakeable`, early-take Ш-3) → Tasks 5–7. ✅
- §15.5 auto-judge + reject false triggers → `isLegal`/precondition checks (Tasks 4, 9, 11) and P-7. ✅
- §15.6 «одна карта» (set at 1, auto-clear, `DeclareOneCard`, `AskCount`→Ш-11) → Tasks 8–9. ✅
- §15.7 endgame (`DiscardWest`, `AskAboutWest`, Ш-12, R-9.4.3 west-заход + западло-forbid) → Tasks 10–11. ✅
- §15.8 actions/events/invariants (I-2/I-3 by construction; one active gate) → distributed; serialization enforced by `gatesClosed`/gate-first `LegalActions`. ✅
- §15.9 sub-iteration slicing → the five sub-iteration groups. **Deviation:** Ш-3 is placed in sub-iteration 2 (with payment/`TakeShukhCards`), not sub-iteration 1, because an early take is only meaningful once the take mechanic and `Shukh` piles exist; the `Unsettled` machinery it reuses is built in sub-iteration 1. Noted here intentionally. ✅
- §15.10 tests (per-ШУХ golden: caught + settled/not-asked; payment multi-giver + 1-card exclusion; Guard-vs-Middle; extended fuzz) → Tasks 4–11 unit tests + Task 12 fuzz. ✅
- Rules: R-3.7 Дама♥ unbeatable (Task 3, needed for the window); R-8.1.1/I-2 (Task 5); R-8.3 (Tasks 6–7); R-6.1/6.3/6.4 (Task 8); R-9.3/9.4.3 (Tasks 10–11). ✅

**2. Placeholder scan:** No TBD/TODO left in delivered code. The one deliberate seam — `assessShukh` applies effects immediately in Task 4, then Task 5 inserts the gate — is a complete, testable intermediate (not a placeholder): Task 4 ships real, passing behavior and Task 5 replaces the whole function body with the shown code. ✅

**3. Type consistency:** `ShukhCode` constants `Sh2/Sh3/Sh11/Sh12` used identically across `Unsettled.Code`, `ClaimShukh.Code`, `ShukhAssessed.Code`. `Payment{Offender, Owed, Skip, ThenDiscardWest}` fields consistent between `assessShukh`, the `GiveShukhCard` case, and tests. `EndgameState{Active, Asked, MustDiscard}` consistent between `resolveExits`, `LegalActions`, `applyShukhEffect`, `DiscardWest`. Action structs: `ClaimShukh{Target, Code}`, `GiveShukhCard{Card}`, `TakeShukhCards{Seat}`, `DeclareOneCard{Seat}`, `AskCount{Target}`, `AskAboutWest{Target}`, `DiscardWest{}` — the seat-carrying vs actor-agnostic split matches P-1 and the `isLegal` routing. Helpers `assessShukh(offender, code, skip, thenDiscardWest, events)`, `applyShukhEffect(offender, skip, thenDiscardWest, events)`, `markShukhTakeable()`, `owedGivers(offender)`, `reconcileOneCard(before)`, `gatesClosed()`, `isLegal(s, a)`, `turnActions(s, seat)` — signatures used identically where consumed. ✅

**4. Invariant safety:** `CheckInvariants` asserts I-1 always and I-6/beat-stack only when `Unsettled == nil` (Task 1), so the transient Дама♥/6(2)♥-on-table windows (Tasks 4, 11) never trip the oracle; both windows settle or reverse to a stable position where the oracle holds again (asserted in tests + fuzz). ✅

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-16-engine-iteration-4-shukhs.md`. Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
