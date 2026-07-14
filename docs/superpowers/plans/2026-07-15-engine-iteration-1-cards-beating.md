# Engine Iteration 1 — Cards, Deck & Beating (§3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation of the pure Go game engine (Layer 0): card model, parametric 36/52 deck, rank ordering with Ace-wrap, special-card predicates, and the legal-beat matrix of §3.

**Architecture:** A single dependency-free Go package `engine` under module `github.com/oustrix/shukh`. All values are absolute face-values so beating logic is deck-independent; deck-dependent facts (lowest rank, special cards) come from a `RuleSet`. No I/O, no networking, no randomness inside the package (determinism lives at higher layers — decision D-7).

**Tech Stack:** Go 1.26, standard library only, table-driven `testing` tests.

**Source of truth:** Rules in [`docs/shukh-rules.md`](../../shukh-rules.md) (refs `R-§.n`, `I-n`). Spec: [`docs/superpowers/specs/2026-07-15-engine-core-design.md`](../specs/2026-07-15-engine-core-design.md). Architecture decisions: [`docs/architecture.md`](../../architecture.md).

## Global Constraints

- Module path: `github.com/oustrix/shukh`. Go version floor: `1.26`.
- All engine code lives in package `engine` (directory `engine/`).
- **Layer 0 purity:** the `engine` package MUST NOT import any I/O, networking, `time`, or `math/rand` package. Standard library non-I/O helpers (`fmt`, `errors`, `sort`) are allowed.
- `Rank` is an absolute face value 2..14 (Jack=11, Queen=12, King=13, Ace=14). Deck size never changes rank values — only which ranks exist (R-2.2, R-2.3).
- Every exported symbol carries a doc comment citing the rule number(s) it implements.
- Tests are table-driven where more than two cases exist.
- Commit after every task with a passing `go test ./...`.

---

### Task 1: Project scaffolding + Card/Suit/Rank types

**Files:**
- Create: `go.mod`
- Create: `engine/card.go`
- Test: `engine/card_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces:
  - `type Suit uint8` with consts `Spades, Hearts, Diamonds, Clubs Suit` (in that order, `Spades == 0`).
  - `func (s Suit) IsTrump() bool` — true only for `Diamonds` (R-2.5).
  - `func (s Suit) String() string` — `"♠"/"♥"/"♦"/"♣"`.
  - `type Rank uint8` with consts `Jack=11, Queen=12, King=13, Ace=14`.
  - `func (r Rank) String() string` — `"J"/"Q"/"K"/"A"` or the decimal for 2..10.
  - `type Card struct { Suit Suit; Rank Rank }` with `func (c Card) String() string` → e.g. `"9♦"`, `"Q♥"`, `"10♠"`.

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /Users/fomindan/go/projects/github.com/oustrix/shukh
go mod init github.com/oustrix/shukh
```
Expected: creates `go.mod` containing `module github.com/oustrix/shukh` and `go 1.26`.

- [ ] **Step 2: Write the failing test**

Create `engine/card_test.go`:
```go
package engine

import "testing"

func TestSuitIsTrump(t *testing.T) {
	if !Diamonds.IsTrump() {
		t.Errorf("Diamonds must be trump (R-2.5)")
	}
	for _, s := range []Suit{Spades, Hearts, Clubs} {
		if s.IsTrump() {
			t.Errorf("%v must not be trump", s)
		}
	}
}

func TestCardString(t *testing.T) {
	cases := []struct {
		card Card
		want string
	}{
		{Card{Diamonds, 9}, "9♦"},
		{Card{Hearts, Queen}, "Q♥"},
		{Card{Spades, 10}, "10♠"},
		{Card{Clubs, Ace}, "A♣"},
		{Card{Hearts, Jack}, "J♥"},
		{Card{Spades, King}, "K♠"},
	}
	for _, c := range cases {
		if got := c.card.String(); got != c.want {
			t.Errorf("Card{%v,%d}.String() = %q, want %q", c.card.Suit, c.card.Rank, got, c.want)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./engine/ -run 'TestSuitIsTrump|TestCardString' -v`
Expected: FAIL — build error, `undefined: Diamonds`, `undefined: Card`, etc.

- [ ] **Step 4: Write minimal implementation**

Create `engine/card.go`:
```go
// Package engine is the pure, deterministic rules core of the card game «Шух»
// (Layer 0). It contains no I/O, networking, time, or randomness — determinism
// and transport live at higher layers (see docs/architecture.md, D-6/D-7).
package engine

import "fmt"

// Suit is a card suit. Diamonds (♦) is always trump (R-2.5).
type Suit uint8

const (
	Spades   Suit = iota // ♠ пики
	Hearts               // ♥ черви
	Diamonds             // ♦ бубны — козырь (R-2.5)
	Clubs                // ♣ трефы
)

// IsTrump reports whether the suit is the trump suit ♦ (R-2.5).
func (s Suit) IsTrump() bool { return s == Diamonds }

func (s Suit) String() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		return "?"
	}
}

// Rank is a card nominal as an absolute face value 2..14 (R-2.2, R-2.3).
// Jack=11, Queen=12, King=13, Ace=14. Which ranks actually exist depends on the
// deck size (RuleSet.LowestRank), but the face values themselves never change.
type Rank uint8

const (
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

func (r Rank) String() string {
	switch r {
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return fmt.Sprintf("%d", uint8(r))
	}
}

// Card is a single playing card.
type Card struct {
	Suit Suit
	Rank Rank
}

func (c Card) String() string { return c.Rank.String() + c.Suit.String() }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./engine/ -run 'TestSuitIsTrump|TestCardString' -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
go test ./...
git add go.mod engine/card.go engine/card_test.go
git commit -m "feat(engine): card, suit and rank types

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: RuleSet — validation, LowestRank, Successor

**Files:**
- Create: `engine/rules.go`
- Test: `engine/rules_test.go`

**Interfaces:**
- Consumes: `Rank`, `Ace` (Task 1).
- Produces:
  - Consts `Deck36 = 36`, `Deck52 = 52`.
  - `type RuleSet struct { DeckSize int; PodkladkaSnizu bool; Jokers bool }`.
  - `func (rs RuleSet) Validate() error` — nil for a supported deck size with both variant flags false; non-nil otherwise.
  - `func (rs RuleSet) LowestRank() Rank` — `6` for Deck36, `2` for Deck52 (R-2.2).
  - `func (rs RuleSet) Successor(r Rank) Rank` — `r+1`, wrapping `Ace → LowestRank()` (R-4.5).

- [ ] **Step 1: Write the failing test**

Create `engine/rules_test.go`:
```go
package engine

import "testing"

func TestRuleSetValidate(t *testing.T) {
	cases := []struct {
		name string
		rs   RuleSet
		ok   bool
	}{
		{"36 ok", RuleSet{DeckSize: Deck36}, true},
		{"52 ok", RuleSet{DeckSize: Deck52}, true},
		{"bad size", RuleSet{DeckSize: 40}, false},
		{"zero size", RuleSet{}, false},
		{"podkladka unsupported", RuleSet{DeckSize: Deck36, PodkladkaSnizu: true}, false},
		{"jokers unsupported", RuleSet{DeckSize: Deck36, Jokers: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.rs.Validate()
			if c.ok && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if !c.ok && err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
}

func TestLowestRank(t *testing.T) {
	if got := (RuleSet{DeckSize: Deck36}).LowestRank(); got != 6 {
		t.Errorf("Deck36 LowestRank = %d, want 6", got)
	}
	if got := (RuleSet{DeckSize: Deck52}).LowestRank(); got != 2 {
		t.Errorf("Deck52 LowestRank = %d, want 2", got)
	}
}

func TestSuccessor(t *testing.T) {
	rs36 := RuleSet{DeckSize: Deck36}
	rs52 := RuleSet{DeckSize: Deck52}
	cases := []struct {
		rs   RuleSet
		in   Rank
		want Rank
	}{
		{rs36, 8, 9},
		{rs36, King, Ace},
		{rs36, Ace, 6}, // wrap Ace → lowest (R-4.5)
		{rs52, Ace, 2}, // wrap Ace → lowest for 52 deck
		{rs52, 2, 3},
	}
	for _, c := range cases {
		if got := c.rs.Successor(c.in); got != c.want {
			t.Errorf("Successor(%d) [deck %d] = %d, want %d", c.in, c.rs.DeckSize, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run 'TestRuleSetValidate|TestLowestRank|TestSuccessor' -v`
Expected: FAIL — `undefined: RuleSet`, `undefined: Deck36`, etc.

- [ ] **Step 3: Write minimal implementation**

Create `engine/rules.go`:
```go
package engine

import "fmt"

// Supported deck sizes (R-2.1).
const (
	Deck36 = 36
	Deck52 = 52
)

// RuleSet is the fixed rule configuration of a game (chosen before play and
// never changed mid-game, R-2.1). Variant flags for the optional §12 rules
// exist from day one but are false and unimplemented in the MVP (decision D-8).
type RuleSet struct {
	DeckSize       int  // Deck36 | Deck52
	PodkladkaSnizu bool // §12 V-1…V-4 — MVP: must be false
	Jokers         bool // §12 jokers — MVP: must be false
}

// Validate reports whether the RuleSet is usable by this build.
func (rs RuleSet) Validate() error {
	if rs.DeckSize != Deck36 && rs.DeckSize != Deck52 {
		return fmt.Errorf("engine: unsupported deck size %d (want 36 or 52)", rs.DeckSize)
	}
	if rs.PodkladkaSnizu {
		return fmt.Errorf("engine: PodkladkaSnizu (§12 V-1) is not implemented in the MVP")
	}
	if rs.Jokers {
		return fmt.Errorf("engine: Jokers (§12) are not implemented in the MVP")
	}
	return nil
}

// LowestRank returns the lowest nominal present in the active deck: 6 for a
// 36-card deck, 2 for a 52-card deck (R-2.2, R-2.3).
func (rs RuleSet) LowestRank() Rank {
	if rs.DeckSize == Deck52 {
		return 2
	}
	return 6
}

// Successor returns the next rank up, wrapping Ace → LowestRank (R-4.5).
func (rs RuleSet) Successor(r Rank) Rank {
	if r == Ace {
		return rs.LowestRank()
	}
	return r + 1
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run 'TestRuleSetValidate|TestLowestRank|TestSuccessor' -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/rules.go engine/rules_test.go
git commit -m "feat(engine): RuleSet with validation, LowestRank and Successor

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Special-card predicates

**Files:**
- Modify: `engine/rules.go` (append predicate methods/functions)
- Test: `engine/rules_test.go` (append)

**Interfaces:**
- Consumes: `Card`, `Hearts`, `Diamonds`, `Queen` (Task 1); `RuleSet`, `LowestRank` (Task 2).
- Produces:
  - `func (rs RuleSet) IsLowestHeart(c Card) bool` — `6(2)♥`, the западло card (R-3.6).
  - `func (rs RuleSet) IsSecondLowestHeart(c Card) bool` — `7(3)♥`, tuck-under target (R-3.6.2).
  - `func IsQueenHearts(c Card) bool` — Дама ♥ (R-3.7). Deck-independent.
  - `func IsStarter(c Card) bool` — `9♦`, opener of the first con (R-5.1). Deck-independent.

- [ ] **Step 1: Write the failing test**

Append to `engine/rules_test.go`:
```go
func TestSpecialCards(t *testing.T) {
	rs36 := RuleSet{DeckSize: Deck36}
	rs52 := RuleSet{DeckSize: Deck52}

	// 6(2)♥ — lowest heart is 6 in a 36 deck, 2 in a 52 deck (R-3.6).
	if !rs36.IsLowestHeart(Card{Hearts, 6}) {
		t.Errorf("6♥ must be the lowest heart in a 36 deck")
	}
	if rs36.IsLowestHeart(Card{Hearts, 2}) {
		t.Errorf("2♥ is not the lowest heart in a 36 deck")
	}
	if !rs52.IsLowestHeart(Card{Hearts, 2}) {
		t.Errorf("2♥ must be the lowest heart in a 52 deck")
	}
	if rs36.IsLowestHeart(Card{Spades, 6}) {
		t.Errorf("6♠ is not a heart")
	}

	// 7(3)♥ — second lowest heart (R-3.6.2).
	if !rs36.IsSecondLowestHeart(Card{Hearts, 7}) {
		t.Errorf("7♥ must be the second-lowest heart in a 36 deck")
	}
	if !rs52.IsSecondLowestHeart(Card{Hearts, 3}) {
		t.Errorf("3♥ must be the second-lowest heart in a 52 deck")
	}

	// Дама ♥ and 9♦ are deck-independent (R-3.7, R-5.1).
	if !IsQueenHearts(Card{Hearts, Queen}) {
		t.Errorf("Q♥ must be recognized")
	}
	if IsQueenHearts(Card{Spades, Queen}) {
		t.Errorf("Q♠ is not Дама ♥")
	}
	if !IsStarter(Card{Diamonds, 9}) {
		t.Errorf("9♦ must be the starter")
	}
	if IsStarter(Card{Hearts, 9}) {
		t.Errorf("9♥ is not the starter")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestSpecialCards -v`
Expected: FAIL — `rs36.IsLowestHeart undefined`, `undefined: IsQueenHearts`, etc.

- [ ] **Step 3: Write minimal implementation**

Append to `engine/rules.go`:
```go
// Special cards. 6(2)♥ and 7(3)♥ depend on the deck (lowest rank differs),
// so they hang off RuleSet; Дама ♥ and 9♦ are absolute face values.

// IsLowestHeart reports whether c is 6(2)♥ — the «западло» card that can only be
// shed by opening or the tuck-under move (R-3.6).
func (rs RuleSet) IsLowestHeart(c Card) bool {
	return c.Suit == Hearts && c.Rank == rs.LowestRank()
}

// IsSecondLowestHeart reports whether c is 7(3)♥ — the card 6(2)♥ tucks under
// in the западло move (R-3.6.2).
func (rs RuleSet) IsSecondLowestHeart(c Card) bool {
	return c.Suit == Hearts && c.Rank == rs.LowestRank()+1
}

// IsQueenHearts reports whether c is Дама ♥ — the highest card of the game
// (R-3.7). Deck-independent.
func IsQueenHearts(c Card) bool { return c.Suit == Hearts && c.Rank == Queen }

// IsStarter reports whether c is 9♦, whose holder opens the very first con of a
// game (R-5.1). Deck-independent (9 exists in both deck sizes).
func IsStarter(c Card) bool { return c.Suit == Diamonds && c.Rank == 9 }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestSpecialCards -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/rules.go engine/rules_test.go
git commit -m "feat(engine): special-card predicates (6(2)♥, 7(3)♥, Дама♥, 9♦)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: NewDeck — parametric 36/52 deck construction

**Files:**
- Create: `engine/deck.go`
- Test: `engine/deck_test.go`

**Interfaces:**
- Consumes: `Card`, `Suit`, `Rank`, `Ace`, `Spades..Clubs` (Task 1); `RuleSet`, `LowestRank` (Task 2).
- Produces:
  - `func NewDeck(rs RuleSet) []Card` — a full, ordered (unshuffled) deck: 36 cards (ranks 6..A) or 52 cards (ranks 2..A), four suits each (R-2.1, R-2.2). Shuffling is the caller's job (D-7).

- [ ] **Step 1: Write the failing test**

Create `engine/deck_test.go`:
```go
package engine

import "testing"

func TestNewDeckSizeAndUniqueness(t *testing.T) {
	for _, rs := range []RuleSet{{DeckSize: Deck36}, {DeckSize: Deck52}} {
		deck := NewDeck(rs)
		if len(deck) != rs.DeckSize {
			t.Fatalf("deck %d: got %d cards, want %d", rs.DeckSize, len(deck), rs.DeckSize)
		}
		seen := make(map[Card]bool, len(deck))
		for _, c := range deck {
			if seen[c] {
				t.Errorf("deck %d: duplicate card %v", rs.DeckSize, c)
			}
			seen[c] = true
			if c.Rank < rs.LowestRank() || c.Rank > Ace {
				t.Errorf("deck %d: card %v out of rank range", rs.DeckSize, c)
			}
		}
	}
}

func TestNewDeckRankBoundaries(t *testing.T) {
	has := func(deck []Card, c Card) bool {
		for _, x := range deck {
			if x == c {
				return true
			}
		}
		return false
	}
	d36 := NewDeck(RuleSet{DeckSize: Deck36})
	if has(d36, Card{Hearts, 2}) {
		t.Errorf("36 deck must not contain 2♥")
	}
	if !has(d36, Card{Hearts, 6}) {
		t.Errorf("36 deck must contain 6♥")
	}
	d52 := NewDeck(RuleSet{DeckSize: Deck52})
	if !has(d52, Card{Hearts, 2}) {
		t.Errorf("52 deck must contain 2♥")
	}
	if !has(d52, Card{Clubs, 5}) {
		t.Errorf("52 deck must contain 5♣")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run 'TestNewDeck' -v`
Expected: FAIL — `undefined: NewDeck`.

- [ ] **Step 3: Write minimal implementation**

Create `engine/deck.go`:
```go
package engine

// NewDeck returns a full, ordered (unshuffled) deck for the given rules: 36
// cards (ranks 6..A) or 52 cards (ranks 2..A), four suits each (R-2.1, R-2.2).
// The caller is responsible for shuffling with an external seed — the engine
// keeps no randomness of its own (decision D-7).
func NewDeck(rs RuleSet) []Card {
	suits := [4]Suit{Spades, Hearts, Diamonds, Clubs}
	low := rs.LowestRank()
	deck := make([]Card, 0, 4*int(Ace-low+1))
	for _, s := range suits {
		for r := low; r <= Ace; r++ {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	return deck
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run 'TestNewDeck' -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
go test ./...
git add engine/deck.go engine/deck_test.go
git commit -m "feat(engine): parametric 36/52 deck construction

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: CanBeat — the §3 legal-beat matrix

**Files:**
- Create: `engine/beat.go`
- Test: `engine/beat_test.go`

**Interfaces:**
- Consumes: `Card`, `Suit`, `Rank` (Task 1); `IsQueenHearts` (Task 3).
- Produces:
  - `func CanBeat(top, c Card) bool` — whether `c` may legally beat the con's top card `top` per §3. Deck-independent (absolute face values). Assumes `top` is a beatable top; Дама ♥ never persists as a top because it closes the con (R-3.7.1), guaranteed by later con logic.

- [ ] **Step 1: Write the failing test**

Create `engine/beat_test.go`:
```go
package engine

import "testing"

func TestCanBeat(t *testing.T) {
	cases := []struct {
		name string
		top  Card
		c    Card
		want bool
	}{
		// ♠ pika — only a higher spade; trump does NOT beat it (R-3.3, I-7).
		{"spade beaten by higher spade", Card{Spades, 9}, Card{Spades, 10}, true},
		{"spade not beaten by lower spade", Card{Spades, 10}, Card{Spades, 9}, false},
		{"spade not beaten by equal spade", Card{Spades, 9}, Card{Spades, 9}, false},
		{"spade not beaten by trump", Card{Spades, 9}, Card{Diamonds, Ace}, false},
		{"spade not beaten by heart", Card{Spades, 9}, Card{Hearts, King}, false},

		// ♦ trump — only a higher diamond (R-3.1).
		{"diamond beaten by higher diamond", Card{Diamonds, 9}, Card{Diamonds, 10}, true},
		{"diamond not beaten by lower diamond", Card{Diamonds, 10}, Card{Diamonds, 9}, false},
		{"diamond not beaten by spade", Card{Diamonds, 9}, Card{Spades, Ace}, false},
		{"diamond not beaten by heart", Card{Diamonds, 9}, Card{Hearts, Ace}, false},

		// ♥ / ♣ — higher same suit OR any diamond (R-3.1, R-3.2).
		{"heart beaten by higher heart", Card{Hearts, 9}, Card{Hearts, 10}, true},
		{"heart not beaten by lower heart", Card{Hearts, 10}, Card{Hearts, 9}, false},
		{"heart beaten by low trump", Card{Hearts, Ace}, Card{Diamonds, 6}, true},
		{"heart not beaten by club", Card{Hearts, 9}, Card{Clubs, King}, false},
		{"heart not beaten by spade", Card{Hearts, 9}, Card{Spades, King}, false},
		{"club beaten by higher club", Card{Clubs, 9}, Card{Clubs, 10}, true},
		{"club beaten by low trump", Card{Clubs, Ace}, Card{Diamonds, 6}, true},
		{"club not beaten by heart", Card{Clubs, 9}, Card{Hearts, King}, false},

		// Дама ♥ beats anything (R-3.7.1).
		{"queen hearts beats spade", Card{Spades, Ace}, Card{Hearts, Queen}, true},
		{"queen hearts beats trump", Card{Diamonds, Ace}, Card{Hearts, Queen}, true},
		{"queen hearts beats club", Card{Clubs, Ace}, Card{Hearts, Queen}, true},

		// 6♥ (lowest heart) beats nothing it could be played against (R-3.6).
		{"lowest heart beats nothing (spade)", Card{Spades, 7}, Card{Hearts, 6}, false},
		{"lowest heart beats nothing (heart)", Card{Hearts, 7}, Card{Hearts, 6}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanBeat(c.top, c.c); got != c.want {
				t.Errorf("CanBeat(%v, %v) = %v, want %v", c.top, c.c, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run TestCanBeat -v`
Expected: FAIL — `undefined: CanBeat`.

- [ ] **Step 3: Write minimal implementation**

Create `engine/beat.go`:
```go
package engine

// CanBeat reports whether card c may legally beat the current top card of the
// con, per §3. It is rule-set independent: ranks are absolute face values and
// the suit matrix is identical for both deck sizes.
//
//	♠ (пика)   — only a higher ♠; trump does NOT beat it (R-3.3, I-7)
//	♦ (козырь) — only a higher ♦ (R-3.1)
//	♥ / ♣      — a higher card of the same suit OR any ♦ (R-3.1, R-3.2)
//	Дама ♥     — beats any card (R-3.7.1)
//
// CanBeat assumes top is a legitimate beatable top. Дама ♥ never persists as a
// top card because it immediately closes the con (R-3.7.1); the con lifecycle
// (a later iteration) guarantees this.
func CanBeat(top, c Card) bool {
	if IsQueenHearts(c) {
		return true // R-3.7.1 — highest card, beats anything
	}
	switch top.Suit {
	case Spades:
		return c.Suit == Spades && c.Rank > top.Rank // R-3.3, I-7
	case Diamonds:
		return c.Suit == Diamonds && c.Rank > top.Rank // R-3.1
	default: // Hearts or Clubs
		if c.Suit == Diamonds {
			return true // R-3.2 — trump of any rank beats a non-spade non-trump
		}
		return c.Suit == top.Suit && c.Rank > top.Rank // R-3.1
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./engine/ -run TestCanBeat -v`
Expected: PASS (all cases).

- [ ] **Step 5: Full build + vet + commit**

```bash
go vet ./...
go test ./...
git add engine/beat.go engine/beat_test.go
git commit -m "feat(engine): §3 legal-beat matrix (CanBeat)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Definition of Done (Iteration 1)

- `go build ./...` and `go vet ./...` clean; `go test ./...` all green.
- `engine` package imports only `fmt` (no I/O, net, time, or math/rand) — Layer 0 purity holds.
- Card/deck/beating covered for both deck sizes; §3 matrix incl. pika stubbornness (I-7), trump non-nominality (R-3.2), and Дама ♥ (R-3.7.1).
- Foundation ready for Iteration 2 (automated dealing §4 + `NewGame` + I-1): `NewDeck`, `RuleSet.Successor` (the «+1» closure), and special-card predicates are the pieces the dealing algorithm builds on.

## Notes for the next plan (Iteration 2)

- Dealing (§4) consumes `NewDeck` + `Successor` (the R-4.3 «+1» with R-4.5 closure) + the `IsLowestHeart` closure detail (Ace precedes 6(2) per R-4.5). Shuffle is seeded at the boundary (D-7) — pass a `[]Card` or a seed into `NewGame`; keep `math/rand` OUT of package `engine` (a shuffle helper can live in a separate package or accept a pre-shuffled deck).
