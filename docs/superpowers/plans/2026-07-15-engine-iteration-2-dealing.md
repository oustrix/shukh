# Engine Iteration 2 — Automated Dealing (§4) + `NewGame` + I-1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn a shuffled deck into starting hands by implementing the automatic dealing algorithm of §4, wrap it in `NewGame`, and add the card-conservation invariant I-1 as the test oracle.

**Architecture:** The pure `engine` package (Layer 0) gains the game-state model (`State`, `Config`, seats, phase, enforcement mode), the dealing reducer, and `CheckInvariants`. `engine.NewGame(cfg, deck []Card)` takes an **already-ordered** deck — it owns no randomness. A separate `shuffle` package (which *may* import `math/rand/v2`) produces a deterministic seeded permutation and imports `engine`, never the reverse (decision **D-11**). This keeps Layer 0 pure (D-6/D-7) and lets dealing golden-tests feed an exact deck instead of hunting for a seed.

**Tech Stack:** Go 1.26. `engine` (non-test) is standard-library-only (`fmt`). `shuffle` imports `math/rand/v2`. Tests use `github.com/stretchr/testify/require`, table-driven where there are more than two cases.

**Source of truth:** Rules in [`docs/shukh-rules.md`](../../shukh-rules.md) (refs `R-§.n`, `I-n`, `Ш-n`). Spec: [`docs/superpowers/specs/2026-07-15-engine-core-design.md`](../specs/2026-07-15-engine-core-design.md). Architecture decisions `D-n`: [`docs/architecture.md`](../../architecture.md). Prior plan: [`2026-07-15-engine-iteration-1-cards-beating.md`](./2026-07-15-engine-iteration-1-cards-beating.md).

## Global Constraints

- Module path: `github.com/oustrix/shukh`. Go version floor: `1.26`.
- **Layer 0 purity (`engine` non-test code):** the `engine` package's non-test files MUST NOT import any I/O, networking, `time`, or `math/rand`/`math/rand/v2` package. Only non-I/O stdlib helpers (`fmt`) are used here. The **`shuffle`** package is exempt: it may import `math/rand/v2` (that is the whole point of moving randomness to the boundary — D-11). `_test.go` files may import `testify`.
- **`shuffle` imports `engine`, never the reverse.** No `engine` file may import `shuffle` (would break Layer 0 purity and create a cycle). Fuzz tests that need both live in the **external** `package engine_test`.
- **Test assertions use `github.com/stretchr/testify/require`** (`require.Equal`, `require.True/False`, `require.Error/NoError`, `require.Empty`, `require.Len`, `require.Contains`), keeping table-driven structure where there are >2 cases. testify (v1.11.1) and deps are already in the local module cache and wired into `go.mod`/`go.sum`. NB: corporate `GOPROXY` does not serve public modules — if a dependency ever needs fetching, use the public proxy inline for this repo only (`GOPROXY=https://proxy.golang.org,direct go get …`), never change the global default, or stay offline with `GOPROXY=off`.
- `Rank` is an absolute face value (Jack=11, Queen=12, King=13, Ace=14); `RuleSet.LowestRank()`/`Successor()` (iteration 1) already encode the deck-dependent order and the Ace→`6(2)` wrap (R-4.5). **Reuse them** — do not re-derive rank order.
- Player count is **2..8** (D-3). Deck sizes **36 | 52** (D-5). `EnforcementMode` **Culture** is enum-present but **not implemented** in this iteration (D-10) — `Config.Validate` rejects it.
- Every exported symbol carries a doc comment citing the rule/decision it implements where one applies; plain helpers get a descriptive comment.
- Commit after every task with a passing `go test ./...`.

### Reused from Iteration 1 (do not redefine)

- `type Card struct { Suit Suit; Rank Rank }`, suits `Spades, Hearts, Diamonds, Clubs`, ranks `Jack, Queen, King, Ace` (`engine/card.go`).
- `type RuleSet struct { DeckSize int; PodkladkaSnizu bool; Jokers bool }`, `Deck36`, `Deck52`, `RuleSet.Validate()`, `RuleSet.LowestRank() Rank`, `RuleSet.Successor(r Rank) Rank` (Ace→lowest wrap, R-4.5) (`engine/rules.go`).
- `func NewDeck(rs RuleSet) []Card` — full ordered deck (`engine/deck.go`).

---

### Task 1: Game-state model + `Config.Validate`

**Files:**
- Create: `engine/state.go`
- Test: `engine/state_test.go`

**Interfaces:**
- Consumes: `RuleSet`, `RuleSet.Validate`, `Card`, `Deck36` (iteration 1).
- Produces:
  - `type SeatID int` — a seat index in clockwise order (R-2.13).
  - `type Phase int` with `Playing Phase = iota`, `Finished`.
  - `type EnforcementMode int` with `Guard EnforcementMode = iota`, `Middle`, `Culture` (D-10).
  - `type Player struct { Name string }` — carried for higher layers; engine logic uses only seat count/order.
  - `type Config struct { Rules RuleSet; Mode EnforcementMode; Players []Player }`.
  - `func (c Config) Validate() error` — nil iff `Rules.Validate()` passes, `Mode ∈ {Guard, Middle}`, and `2 ≤ len(Players) ≤ 8`.
  - `type State struct { … }` — the fields below (Endgame/Pending/Unsettled are deferred to later iterations; only card-bearing + control fields exist now).
  - `type Event interface { isEvent() }` and `type GameStarted struct { Turn SeatID }` with `func (GameStarted) isEvent() {}`.

- [ ] **Step 1: Write the failing test**

Create `engine/state_test.go`:
```go
package engine

import "testing"

import "github.com/stretchr/testify/require"

func players(n int) []Player {
	ps := make([]Player, n)
	for i := range ps {
		ps[i] = Player{Name: "p"}
	}
	return ps
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"2 players guard ok", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Guard, Players: players(2)}, true},
		{"8 players middle ok", Config{Rules: RuleSet{DeckSize: Deck52}, Mode: Middle, Players: players(8)}, true},
		{"1 player too few", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Middle, Players: players(1)}, false},
		{"9 players too many", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Middle, Players: players(9)}, false},
		{"culture not implemented", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Culture, Players: players(2)}, false},
		{"bad deck size", Config{Rules: RuleSet{DeckSize: 40}, Mode: Middle, Players: players(2)}, false},
		{"unknown mode", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: EnforcementMode(99), Players: players(2)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGameStartedIsEvent(t *testing.T) {
	var e Event = GameStarted{Turn: 3}
	require.Equal(t, SeatID(3), e.(GameStarted).Turn)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run 'TestConfigValidate|TestGameStartedIsEvent' -v`
Expected: FAIL — `undefined: Config`, `undefined: Player`, `undefined: GameStarted`, etc.

- [ ] **Step 3: Write minimal implementation**

Create `engine/state.go`:
```go
package engine

import "fmt"

// SeatID identifies a seat by its position in clockwise order (R-2.13). Seats are
// numbered 0..n-1 and fixed for the whole game.
type SeatID int

// Phase is the coarse lifecycle stage of a game.
type Phase int

const (
	// Playing means the game is in progress (dealing is atomic inside NewGame).
	Playing Phase = iota
	// Finished means every player has left (R-9.2); set by later iterations.
	Finished
)

// EnforcementMode selects how strictly the engine blocks vs. lets-and-catches
// rule violations (decision D-10). Culture is enum-present but not implemented in
// the MVP (it needs position rollback) — Config.Validate rejects it.
type EnforcementMode int

const (
	// Guard blocks everything detectable.
	Guard EnforcementMode = iota
	// Middle (the table default) blocks only integrity-breaking moves.
	Middle
	// Culture blocks only the physically impossible (later iteration).
	Culture
)

// Player is a seat occupant. Name is carried for higher layers (display, events);
// engine logic uses only the number and order of players.
type Player struct {
	Name string
}

// Config is the immutable setup of a game: rules, enforcement mode, and the
// players in clockwise order (spec §4).
type Config struct {
	Rules   RuleSet
	Mode    EnforcementMode
	Players []Player
}

// Validate reports whether the Config can start a game: a supported RuleSet, an
// implemented enforcement mode (Guard | Middle, D-10), and 2..8 players (D-3).
func (c Config) Validate() error {
	if err := c.Rules.Validate(); err != nil {
		return err
	}
	switch c.Mode {
	case Guard, Middle:
		// implemented
	case Culture:
		return fmt.Errorf("engine: Culture enforcement mode is not implemented in the MVP (D-10)")
	default:
		return fmt.Errorf("engine: unknown enforcement mode %d", c.Mode)
	}
	if n := len(c.Players); n < 2 || n > 8 {
		return fmt.Errorf("engine: player count %d out of range 2..8 (D-3)", n)
	}
	return nil
}

// State is the full authoritative game state (spec §2, invariant I-1: every card
// is in exactly one zone at all times). Fields for later iterations (Endgame,
// Pending adjudication, Unsettled catch-window) are added when those mechanics
// land; this iteration populates only the dealing result and turn control.
type State struct {
	Rules RuleSet         // deck size + §12 variant flags
	Mode  EnforcementMode // Guard | Middle (Culture later)
	Seats []SeatID        // clockwise order (R-2.13), fixed for the game
	Phase Phase           // Playing | Finished

	Talon   []Card            // undealt deck; empty after NewGame (dealing done, R-4.10)
	Hands   map[SeatID][]Card // each player's hand (a set; order irrelevant to play)
	Table   []Card            // the con, bottom→top; empty at start of game
	Discard []Card            // closed discard pile (R-2.9)
	Shukh   map[SeatID][]Card // set-aside ШУХ cards, face down (R-2.10, I-3)

	Turn   SeatID       // whose turn it is
	Live   map[SeatID]bool // players still in the game (R-5.5.1, R-9.1)
	Finish []SeatID     // finishing order → places (R-9.2)
}

// Event is a state-transition fact emitted for higher layers (animations, logs,
// spec §9). It is a sealed interface — only engine types implement it.
type Event interface{ isEvent() }

// GameStarted is emitted by NewGame once dealing is complete; Turn is the seat
// that opens the first con (holder of 9♦, R-5.1).
type GameStarted struct {
	Turn SeatID
}

func (GameStarted) isEvent() {}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run 'TestConfigValidate|TestGameStartedIsEvent' -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/state.go engine/state_test.go
git commit -m "feat(engine): game-state model, Config with validation, events

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `shuffle` package — deterministic seeded Fisher–Yates

**Files:**
- Create: `shuffle/shuffle.go`
- Test: `shuffle/shuffle_test.go`

**Interfaces:**
- Consumes: `engine.Card`, `engine.NewDeck`, `engine.RuleSet`, `engine.Deck36`.
- Produces:
  - `func Deck(cards []engine.Card, seed int64) []engine.Card` — a shuffled **copy** of `cards`, deterministic for a given `seed` and Go version. Does not mutate the input.

- [ ] **Step 1: Write the failing test**

Create `shuffle/shuffle_test.go`:
```go
package shuffle_test

import (
	"sort"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/shuffle"

	"github.com/stretchr/testify/require"
)

func key(c engine.Card) int { return int(c.Suit)*100 + int(c.Rank) }

func sortedKeys(cs []engine.Card) []int {
	ks := make([]int, len(cs))
	for i, c := range cs {
		ks[i] = key(c)
	}
	sort.Ints(ks)
	return ks
}

func TestDeckDeterministic(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	a := shuffle.Deck(full, 42)
	b := shuffle.Deck(full, 42)
	require.Equal(t, a, b, "same seed must yield the same permutation")
}

func TestDeckIsPermutation(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	got := shuffle.Deck(full, 7)
	require.Len(t, got, len(full))
	require.Equal(t, sortedKeys(full), sortedKeys(got), "shuffle must preserve the multiset of cards")
}

func TestDeckDoesNotMutateInput(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	before := append([]engine.Card(nil), full...)
	_ = shuffle.Deck(full, 99)
	require.Equal(t, before, full, "input slice must be untouched")
}

func TestDeckDifferentSeedsDiffer(t *testing.T) {
	full := engine.NewDeck(engine.RuleSet{DeckSize: engine.Deck36})
	require.NotEqual(t, shuffle.Deck(full, 1), shuffle.Deck(full, 2),
		"different seeds should (overwhelmingly likely) differ for a 36-card deck")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./shuffle/ -v`
Expected: FAIL — `undefined: shuffle.Deck` (package/function does not exist yet).

- [ ] **Step 3: Write minimal implementation**

Create `shuffle/shuffle.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./shuffle/ -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add shuffle/shuffle.go shuffle/shuffle_test.go
git commit -m "feat(shuffle): deterministic seeded deck shuffle (D-11)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Dealing algorithm (§4) — internal `dealAll`

**Files:**
- Create: `engine/deal.go`
- Test: `engine/deal_test.go`

**Interfaces:**
- Consumes: `Card`, suits/ranks (iteration 1); `RuleSet.Successor` (iteration 1); `SeatID` (Task 1).
- Produces (unexported — the dealing reducer, tested in-package because it accepts arbitrary tiny decks that `NewGame` would reject):
  - `func dealAll(rs RuleSet, seats []SeatID, deck []Card) map[SeatID][]Card` — plays out the §4 deal. `deck[0]` is the top of the talon (drawn first). `seats[0]` is the shuffler (R-4.7). Returns each seat's final pile (bottom→top; the top is `pile[len-1]`), which becomes the starting hand (R-4.10).

**Algorithm (implements R-4.3…R-4.10, spec §7):**
- R-4.7: the shuffler (`seats[0]`) takes the first (top) card onto its own pile, unconditionally.
- Then, clockwise, each player takes a turn until the talon is empty:
  - **R-4.4.1 unload (before touching the deck):** while the player's own **top** card fits some opponent by "+1" (`Successor(oppTop.Rank) == myTop.Rank`), move that top card to the **first such opponent clockwise** (R-4.6) and repeat with the newly exposed top. Only opponents receive — never self.
  - **R-4.4.2/3 draw:** draw the top of the talon. If it fits some opponent by "+1", place it on the first such opponent clockwise (R-4.6) and **draw again** (R-4.4.3 loops back to the draw, *not* to unload).
  - **R-4.4.4 terminal:** if the drawn card fits **no** opponent, place it on the player's **own** pile (R-4.8, no "+1" restriction) — the turn ends.
  - **R-4.9 last card:** the **last** card of the talon always goes to the drawer's own pile, *even if it would fit an opponent* ("последняя карта обязательно себе"). This overrides R-4.4.3.
- "Fits by +1" is `Successor(oppTop.Rank) == card.Rank`, where `Successor` already wraps Ace→lowest (R-4.5, so `6(2)` lands on Ace). Suit is ignored during dealing (R-4.3).

- [ ] **Step 1: Write the failing test**

Create `engine/deal_test.go`. The three golden decks are hand-traced against the algorithm above (36-deck ranks, `Successor` wraps Ace→6). Suits are irrelevant to dealing and only keep cards unique.

```go
package engine

import "testing"

import "github.com/stretchr/testify/require"

// Golden A (3 players, shuffler P0): opponent-forwarding + self-terminal, and the
// last card (8♥) goes to the drawer (P0) even though it would fit P2 — R-4.9.
//
// Trace: P0 seeds 8♠ (R-4.7). P1 draws 9♠→P0, Q♠→self. P2 draws 10♠→P0, K♠→P1,
// 7♥→self. P0 draws last 8♥→self (R-4.9).
func TestDealAllGoldenForwarding(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1, 2}
	deck := []Card{
		{Spades, 8}, {Spades, 9}, {Spades, Queen},
		{Spades, 10}, {Spades, King}, {Hearts, 7}, {Hearts, 8},
	}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, 8}, {Spades, 9}, {Spades, 10}, {Hearts, 8}}, got[0])
	require.Equal(t, []Card{{Spades, Queen}, {Spades, King}}, got[1])
	require.Equal(t, []Card{{Hearts, 7}}, got[2])
}

// Golden B (2 players, shuffler P0): the R-4.4.1 unload. On P1's final turn its own
// top 8♠ fits P0 (top 7♠), so P1 MUST move 8♠ to P0 before drawing; then P1 draws
// the last card 9♥ to self (R-4.9).
//
// Trace: P0 seeds 6♠. P1 draws 8♠→self. P0 draws 7♠→self. P1 unloads 8♠→P0, draws
// last 9♥→self.
func TestDealAllGoldenUnload(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1}
	deck := []Card{{Spades, 6}, {Spades, 8}, {Spades, 7}, {Hearts, 9}}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, 6}, {Spades, 7}, {Spades, 8}}, got[0])
	require.Equal(t, []Card{{Hearts, 9}}, got[1])
}

// Golden C (2 players, shuffler P0): the Ace→6 wrap (R-4.5). P1 draws 6♥, which
// fits P0's Ace top (Successor(Ace)=6), so 6♥ goes onto the Ace; then 9♠ is the
// last card → self.
func TestDealAllGoldenAceWrap(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1}
	deck := []Card{{Spades, Ace}, {Hearts, 6}, {Spades, 9}}
	got := dealAll(rs, seats, deck)
	require.Equal(t, []Card{{Spades, Ace}, {Hearts, 6}}, got[0])
	require.Equal(t, []Card{{Spades, 9}}, got[1])
}

// Conservation: dealAll never loses or duplicates a card (I-1 at the dealing level).
func TestDealAllConservesCards(t *testing.T) {
	rs := RuleSet{DeckSize: Deck36}
	seats := []SeatID{0, 1, 2}
	deck := []Card{
		{Spades, 8}, {Spades, 9}, {Spades, Queen},
		{Spades, 10}, {Spades, King}, {Hearts, 7}, {Hearts, 8},
	}
	got := dealAll(rs, seats, deck)
	seen := map[Card]int{}
	total := 0
	for _, s := range seats {
		for _, c := range got[s] {
			seen[c]++
			total++
		}
	}
	require.Equal(t, len(deck), total)
	for _, c := range deck {
		require.Equal(t, 1, seen[c], "card %v must appear exactly once", c)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestDealAll -v`
Expected: FAIL — `undefined: dealAll`.

- [ ] **Step 3: Write minimal implementation**

Create `engine/deal.go`:
```go
package engine

// dealAll plays out the automatic §4 deal: it turns an ordered deck into each
// seat's starting pile (R-4.3…R-4.10). deck[0] is the top of the talon (drawn
// first); seats[0] is the shuffler (R-4.7). A pile is stored bottom→top, so its
// top card — the only one that matters during dealing (R-4.2) — is pile[len-1].
//
// The move set per turn (spec §7):
//   - R-4.4.1: unload — while my top fits an opponent by "+1", give it to the
//     first such opponent clockwise (R-4.6); repeat. Only opponents receive.
//   - R-4.4.2/3: draw — a drawn card that fits an opponent goes to the first such
//     opponent clockwise, then draw again.
//   - R-4.4.4: a drawn card that fits no opponent goes onto my own pile; turn ends.
//   - R-4.9: the LAST card of the talon always goes to the drawer's own pile,
//     even if it would fit an opponent.
//
// "Fits by +1" is Successor(oppTop.Rank) == card.Rank, with Successor wrapping
// Ace→lowest (R-4.5); suit is ignored (R-4.3).
func dealAll(rs RuleSet, seats []SeatID, deck []Card) map[SeatID][]Card {
	n := len(seats)
	piles := make(map[SeatID][]Card, n)
	for _, s := range seats {
		piles[s] = nil
	}

	idx := 0 // index of the next card to draw (top of talon)
	remaining := func() int { return len(deck) - idx }
	draw := func() Card { c := deck[idx]; idx++; return c }

	topOf := func(s SeatID) (Card, bool) {
		p := piles[s]
		if len(p) == 0 {
			return Card{}, false
		}
		return p[len(p)-1], true
	}
	pop := func(s SeatID) { piles[s] = piles[s][:len(piles[s])-1] }
	push := func(s SeatID, c Card) { piles[s] = append(piles[s], c) }

	// firstOpponent returns the first seat clockwise from cur (excluding cur)
	// whose top card is the predecessor of c ("+1" fit, R-4.3/R-4.6).
	firstOpponent := func(c Card, cur int) (SeatID, bool) {
		for k := 1; k < n; k++ {
			s := seats[(cur+k)%n]
			if t, ok := topOf(s); ok && rs.Successor(t.Rank) == c.Rank {
				return s, true
			}
		}
		return 0, false
	}

	// R-4.7: the shuffler takes the first card onto its own pile.
	push(seats[0], draw())

	for cur := 1 % n; remaining() > 0; cur = (cur + 1) % n {
		curSeat := seats[cur]

		// R-4.4.1: unload own pile onto opponents while the top fits.
		for {
			t, ok := topOf(curSeat)
			if !ok {
				break
			}
			opp, found := firstOpponent(t, cur)
			if !found {
				break
			}
			pop(curSeat)
			push(opp, t)
		}

		// R-4.4.2/3/4 + R-4.9: draw until a card lands on the current player.
		for {
			c := draw()
			if remaining() == 0 {
				push(curSeat, c) // R-4.9: last card always to the drawer
				break
			}
			if opp, found := firstOpponent(c, cur); found {
				push(opp, c) // R-4.4.3: to opponent, then draw again
				continue
			}
			push(curSeat, c) // R-4.4.4: terminal, turn ends
			break
		}
	}

	return piles
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestDealAll -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/deal.go engine/deal_test.go
git commit -m "feat(engine): §4 automatic dealing algorithm (dealAll)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: `CheckInvariants` — I-1 card conservation

**Files:**
- Create: `engine/invariants.go`
- Test: `engine/invariants_test.go`

**Interfaces:**
- Consumes: `State`, `Card` (Task 1); `NewDeck` (iteration 1).
- Produces:
  - `func CheckInvariants(s State) error` — verifies I-1: the multiset of cards across `Talon`, all `Hands`, `Table`, `Discard`, and all `Shukh` piles equals exactly `NewDeck(s.Rules)` — every deck card present exactly once, no foreign cards, no duplicates. Returns a typed error describing the first violation, else nil. (Later iterations extend this with I-2/I-4/I-5/I-6/I-7.)

- [ ] **Step 1: Write the failing test**

Create `engine/invariants_test.go`:
```go
package engine

import "testing"

import "github.com/stretchr/testify/require"

// fullState builds a minimal valid State whose hands hold the entire deck (as if
// just dealt), so I-1 holds.
func fullState(rs RuleSet) State {
	deck := NewDeck(rs)
	return State{
		Rules: rs,
		Seats: []SeatID{0, 1},
		Hands: map[SeatID][]Card{0: deck, 1: {}},
		Shukh: map[SeatID][]Card{},
	}
}

func TestCheckInvariantsI1Holds(t *testing.T) {
	require.NoError(t, CheckInvariants(fullState(RuleSet{DeckSize: Deck36})))
	require.NoError(t, CheckInvariants(fullState(RuleSet{DeckSize: Deck52})))
}

func TestCheckInvariantsI1MissingCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Hands[0] = s.Hands[0][:len(s.Hands[0])-1] // drop one card
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI1DuplicateCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	dup := s.Hands[0][0]
	s.Hands[1] = []Card{dup} // same card now in two zones
	require.Error(t, CheckInvariants(s))
}

func TestCheckInvariantsI1ForeignCard(t *testing.T) {
	s := fullState(RuleSet{DeckSize: Deck36})
	s.Table = []Card{{Hearts, 2}} // 2♥ is not in a 36-card deck
	require.Error(t, CheckInvariants(s))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestCheckInvariants -v`
Expected: FAIL — `undefined: CheckInvariants`.

- [ ] **Step 3: Write minimal implementation**

Create `engine/invariants.go`:
```go
package engine

import "fmt"

// CheckInvariants verifies the always-true structural invariants of a stable
// state. This iteration checks I-1 (card conservation): every card of the deck is
// in exactly one zone — Talon, a Hand, Table, Discard, or a Shukh pile — with no
// missing, foreign, or duplicated cards. Later iterations add I-2/I-4/I-5/I-6/I-7.
//
// It returns a typed error describing the first violation, or nil. Callers run it
// after every Apply that yields a stable position (spec §10).
func CheckInvariants(s State) error {
	full := NewDeck(s.Rules)
	want := make(map[Card]bool, len(full))
	for _, c := range full {
		want[c] = true
	}

	seen := make(map[Card]int, len(full))
	total := 0
	count := func(cs []Card) {
		for _, c := range cs {
			seen[c]++
			total++
		}
	}
	count(s.Talon)
	for _, h := range s.Hands {
		count(h)
	}
	count(s.Table)
	count(s.Discard)
	for _, z := range s.Shukh {
		count(z)
	}

	for c, k := range seen {
		if !want[c] {
			return fmt.Errorf("engine: I-1 violated: card %v is not part of a %d-card deck", c, s.Rules.DeckSize)
		}
		if k != 1 {
			return fmt.Errorf("engine: I-1 violated: card %v appears %d times (want 1)", c, k)
		}
	}
	if total != len(full) {
		return fmt.Errorf("engine: I-1 violated: %d cards present across zones, want %d", total, len(full))
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestCheckInvariants -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/invariants.go engine/invariants_test.go
git commit -m "feat(engine): CheckInvariants with I-1 card conservation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: `NewGame` — assemble the dealt game + seeded fuzz

**Files:**
- Modify: `engine/deal.go` (append `NewGame`, `validateDeck`, `holderOf`)
- Test: `engine/newgame_test.go` (external `package engine_test`, so it may import `shuffle`)

**Interfaces:**
- Consumes: `Config`, `Config.Validate`, `State`, `SeatID`, `GameStarted`, `Event` (Task 1); `dealAll` (Task 3); `CheckInvariants` (Task 4); `NewDeck`, `RuleSet` (iteration 1); `shuffle.Deck` (Task 2, tests only).
- Produces:
  - `func NewGame(cfg Config, deck []Card) (State, []Event, error)` — validates `cfg` and that `deck` is exactly the `NewDeck(cfg.Rules)` multiset, deals via `dealAll` (shuffler = seat 0), and builds the starting `State`: `Phase=Playing`, empty `Talon`/`Table`/`Discard`/`Shukh`, all seats `Live`, `Turn` = holder of `9♦` (R-5.1). Returns `[]Event{GameStarted{Turn}}`. On any validation failure returns a zero `State`, nil events, and a typed error.

- [ ] **Step 1: Write the failing test**

Create `engine/newgame_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestNewGame -v`
Expected: FAIL — `undefined: engine.NewGame`.

- [ ] **Step 3: Write minimal implementation**

Append to `engine/deal.go` (add `import "fmt"` at the top of the file — change the bare `package engine` line to a package clause followed by the import block):
```go
// starter is 9♦ — the holder opens the first con (R-5.1).
var starter = Card{Suit: Diamonds, Rank: 9}

// NewGame validates the config and deck, runs the automatic §4 deal, and returns
// the starting state plus a GameStarted event. deck must be exactly the
// NewDeck(cfg.Rules) multiset (any order — shuffling is the caller's job via the
// shuffle package, D-11). The shuffler is seat 0 (R-4.7). Turn is set to the
// holder of 9♦, who opens the first con (R-5.1).
func NewGame(cfg Config, deck []Card) (State, []Event, error) {
	if err := cfg.Validate(); err != nil {
		return State{}, nil, err
	}
	rs := cfg.Rules
	if err := validateDeck(rs, deck); err != nil {
		return State{}, nil, err
	}

	n := len(cfg.Players)
	seats := make([]SeatID, n)
	for i := range seats {
		seats[i] = SeatID(i)
	}

	hands := dealAll(rs, seats, deck)

	turn, ok := holderOf(hands, starter)
	if !ok {
		// 9♦ is in every deck size and I-1 puts it in exactly one hand; a miss
		// means a dealing bug.
		return State{}, nil, fmt.Errorf("engine: 9♦ was not dealt to any seat (internal invariant violated)")
	}

	live := make(map[SeatID]bool, n)
	for _, s := range seats {
		live[s] = true
	}

	st := State{
		Rules: rs,
		Mode:  cfg.Mode,
		Seats: seats,
		Phase: Playing,
		Talon: nil,
		Hands: hands,
		Shukh: make(map[SeatID][]Card, n),
		Turn:  turn,
		Live:  live,
	}
	return st, []Event{GameStarted{Turn: turn}}, nil
}

// validateDeck reports whether deck is exactly the full deck for rs: the right
// size, no foreign cards, no duplicates (I-1 precondition for NewGame).
func validateDeck(rs RuleSet, deck []Card) error {
	full := NewDeck(rs)
	if len(deck) != len(full) {
		return fmt.Errorf("engine: deck has %d cards, want %d for a %d-card game", len(deck), len(full), rs.DeckSize)
	}
	want := make(map[Card]bool, len(full))
	for _, c := range full {
		want[c] = true
	}
	seen := make(map[Card]bool, len(deck))
	for _, c := range deck {
		if !want[c] {
			return fmt.Errorf("engine: deck contains %v, not part of a %d-card deck", c, rs.DeckSize)
		}
		if seen[c] {
			return fmt.Errorf("engine: deck contains duplicate %v", c)
		}
		seen[c] = true
	}
	return nil
}

// holderOf returns the seat whose hand contains c. With I-1 holding, c is in at
// most one hand.
func holderOf(hands map[SeatID][]Card, c Card) (SeatID, bool) {
	for s, h := range hands {
		for _, x := range h {
			if x == c {
				return s, true
			}
		}
	}
	return 0, false
}
```

Change the first line of `engine/deal.go` from:
```go
package engine
```
to:
```go
package engine

import "fmt"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestNewGame -v`
Expected: PASS (all five tests, including the fuzz across both deck sizes × 2..8 players × 200 seeds).

- [ ] **Step 5: Full build + vet + commit**

```bash
go vet ./...
go test ./...
git add engine/deal.go engine/newgame_test.go
git commit -m "feat(engine): NewGame — deal a game from a deck + seeded fuzz

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Definition of Done (Iteration 2)

- `go build ./...`, `go vet ./...` clean; `go test ./...` all green.
- **Layer 0 purity holds:** the `engine` package imports only `fmt` (no I/O, net, time, `math/rand`); `math/rand/v2` lives solely in `shuffle`; no `engine` file imports `shuffle` (D-11).
- Dealing (§4) is covered by three hand-traced golden decks — opponent-forwarding + self-terminal, the R-4.4.1 unload, and the R-4.5 Ace→6 wrap — plus the R-4.9 last-card-to-self override.
- `NewGame` produces a valid starting state for both deck sizes and 2..8 players; the opener holds `9♦` (R-5.1); bad configs and bad decks are rejected with typed errors; `NewGame` is deterministic in `(cfg, deck)`.
- `CheckInvariants` enforces I-1 and passes across the seeded fuzz (2 deck sizes × 7 player counts × 200 seeds = 2800 deals) with zero violations.
- Zero `panic` on any tested input.

## Self-Review (spec coverage)

- Spec §7 (dealing R-4.3…R-4.10, +1 with R-4.5 wrap, R-4.6 conflict, R-4.7 shuffler-first, R-4.9 last-card) → Task 3 `dealAll` + Task 5 `NewGame`. **§4.3 dealing ШУХи (R-4.11…R-4.14) are explicitly out of scope** (spec §1: auto-dealing makes Ш-1 a dead branch).
- Spec §2 State model + I-1 → Task 1 `State`, Task 4 `CheckInvariants`.
- Spec §4 `NewGame` signature → Task 5 (adapted to `deck []Card` per **D-11**; `seed`-based reproducibility is provided by `shuffle.Deck` at the boundary).
- Spec §10 tests: micro-example golden (Task 3), fuzz-dealing with invariant oracle (Task 5). Determinism (D-7) → Task 2 + Task 5.
- Deferred by design (later iterations): con lifecycle §5, `LegalActions`/`Apply`/`View`, remaining invariants I-2/I-4/I-5/I-6/I-7, `Endgame`/`Pending`/`Unsettled` state fields, events beyond `GameStarted`.

## Notes for the next plan (Iteration 3 — con lifecycle §5 + exit §9.1)

- `Apply(s, PlayCard{...})` will consume `CanBeat` (iteration 1) and the `Table` stack; con-close by count (R-5.5, threshold falls with `Live`) and by Дама♥ (R-3.7.1); `TakeBottomAndPass` (R-5.8); turn order R-5.7. Extend `CheckInvariants` with I-4/I-5/I-6/I-7. Add `Finished` transition + `Finish` order (R-9.1/R-9.2). `LegalActions` arrives here as the executable spec of legal moves.
- Series play (loser shuffles, R-4.15) will need the shuffler seat to become a parameter rather than the hardcoded `seats[0]` — revisit `dealAll`/`NewGame` then.
