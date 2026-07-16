# Спец 2 (Слой 1) — `game.Session` + голосование в движке — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Достроить в движке отложенную из Спеца 1 машинерию голосования R-8.6 (субъективные ШУХи Ш-6/Ш-8/Ш-9/Ш-10), затем построить транспорт-агностичный `game.Session` — оркестратор одной живой партии поверх движка.

**Architecture:** Две фазы. **Фаза 1** расширяет чистый Слой 0 (`engine/`): новое поле `State.Adjudication`, действия `ClaimSubjective`/`Vote`, авто-подсчёт кворума и применение исхода через существующую §8-машинерию (`assessShukh`). **Фаза 2** добавляет пакет `game/` (Слой 1): синхронный `Session` под `sync.Mutex`, жизненный цикл `Lobby→Playing→Finished`, идентичность `PlayerID↔SeatID`, приём действий с анти-имперсонацией, per-seat проекция и фанаут событий подписчикам.

**Tech Stack:** Go (stdlib: `sync`, `errors`, `slices`, `maps`, `testing`). Зависимости — только внутренние пакеты `engine` и `shuffle`. Никаких сетевых/сторонних библиотек.

**Design doc:** [`docs/superpowers/specs/2026-07-16-engine-session-layer1-design.md`](../specs/2026-07-16-engine-session-layer1-design.md).

## Global Constraints

- **Слой 0 (`engine/`) без I/O, сети, времени, RNG** — чистые функции над `State` (D-6/D-7). Фаза 1 это сохраняет.
- **Правила — источник истины** [`docs/shukh-rules.md`](../../shukh-rules.md); ссылки `R-§.n`/`Ш-n`/`I-n`/`P-n`.
- **`State.Pending` уже занят** §8 payment-гейтом (`*engine.Payment`) — новое поле голосования называется **`Adjudication`**, тип `*Adjudication`.
- **Избиратели R-8.9** — весь стол, все сиденья `0..n-1`, включая вышедших (`Finish`); одно сиденье — один голос.
- **Исход R-8.6** — `support*2 > len(Seats)` (большинство за оспаривание) → Ш-8 на предъявителя; иначе ШУХ подтверждён на цели.
- **`Session` синхронна под `sync.Mutex`** (S-3); никаких внутренних горутин; фанаут — неблокирующая отправка в буферизованные каналы.
- **Тесты в стиле `engine/`**: детерминизм, точная колода/seed; гонки — `go test -race ./...`.
- **Коммиты частые**, по одному на задачу; сообщения в стиле репозитория (`feat(engine)`, `feat(game)`, `test(...)`).
- **Проверка перед коммитом:** `go build ./...` и `go test ./...` зелёные (pre-commit хук гоняет это для `engine/`).

---

## Файловая структура

**Фаза 1 (движок, правит существующие файлы):**
- `engine/state.go` — + константы `Sh6/Sh8/Sh9/Sh10`, тип `Adjudication`, поле `State.Adjudication`, хелперы `isSubjective`, `voterEligible`, `hasVoted`; правки `clone`, `gatesClosed`.
- `engine/event.go` — + события `VoteOpened`, `VoteResolved`.
- `engine/action.go` — + действия `ClaimSubjective`, `Vote`.
- `engine/apply.go` — + ветки `ClaimSubjective`/`Vote` в `Apply`, кейсы в `isLegal`, хелпер `resolveAdjudication`.
- `engine/legal.go` — + ранняя ветка «идёт разбор» в `LegalActions`.
- Тесты: `engine/adjudication_test.go` (новый).

**Фаза 2 (новый пакет `game/`):**
- `game/session.go` — `PlayerID`, `Lifecycle`, `Config`, `Session`, `NewSession`, лобби-методы (`Join`/`Leave`/`SetConfig`/`Start`), ошибки.
- `game/submit.go` — `Submit`, `authorize` (анти-имперсонация).
- `game/projection.go` — `Update`, `SeatMeta`, `Snapshot`, `project`, `roster`.
- `game/subscribe.go` — `subscriber`, `Subscribe`, `unsubscribe`, `fanout`.
- Тесты: `game/session_test.go`, `game/submit_test.go`, `game/subscribe_test.go`, `game/integration_test.go`.

---

# ФАЗА 1 — Движок: голосование R-8.6

### Task 1: Состояние разбора — коды, тип `Adjudication`, поле, хелперы

**Files:**
- Modify: `engine/state.go`
- Modify: `engine/apply.go` (в `clone` и `gatesClosed`)
- Test: `engine/adjudication_test.go` (Create)

**Interfaces:**
- Produces: `engine.Sh6/Sh8/Sh9/Sh10 ShukhCode`; `engine.Adjudication{Claimant, Target SeatID; Code ShukhCode; Votes map[SeatID]bool}`; поле `State.Adjudication *Adjudication`; методы `(ShukhCode).isSubjective() bool`, `(State).voterEligible(SeatID) bool`, `(State).hasVoted(SeatID) bool` (неэкспортируемые).

- [ ] **Step 1: Тест — `gatesClosed` ложно при активном разборе, коды субъективны**

Создать `engine/adjudication_test.go`:

```go
package engine

import "testing"

func TestSubjectiveCodesClassified(t *testing.T) {
	for _, c := range []ShukhCode{Sh6, Sh9, Sh10} {
		if !c.isSubjective() {
			t.Errorf("Ш-%d must be subjective", c)
		}
	}
	for _, c := range []ShukhCode{Sh2, Sh3, Sh8, Sh11, Sh12} {
		if c.isSubjective() {
			t.Errorf("Ш-%d must not be subjective (Sh8 is an outcome, not a claim)", c)
		}
	}
}

func TestAdjudicationClosesGates(t *testing.T) {
	s := State{Seats: []SeatID{0, 1, 2}}
	if !s.gatesClosed() {
		t.Fatal("empty state must have gates closed")
	}
	s.Adjudication = &Adjudication{Claimant: 0, Target: 1, Code: Sh6, Votes: map[SeatID]bool{}}
	if s.gatesClosed() {
		t.Fatal("an open Adjudication must close the gates")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run 'TestSubjectiveCodesClassified|TestAdjudicationClosesGates'`
Expected: FAIL — `undefined: Sh6 / isSubjective / Adjudication / Adjudication field`.

- [ ] **Step 3: Добавить коды и `isSubjective` в `engine/state.go`**

После существующего блока `const ( Sh2 ... Sh12 )` добавить:

```go
const (
	Sh6  ShukhCode = 6  // «завис» / затянул ход (R-8.4) — субъективный
	Sh8  ShukhCode = 8  // «ложный ШУХ»: исход оспаривания, перекладывается на предъявителя (R-8.6)
	Sh9  ShukhCode = 9  // произнёс «ШУХ» без надобности (R-8.7) — субъективный
	Sh10 ShukhCode = 10 // небрежность: уронил карту, походил рубашкой (R-8.8) — субъективный
)

// isSubjective reports whether a ШУХ is a subjective claim resolved by table vote
// (R-8.6, R-8.9): Ш-6/Ш-9/Ш-10. Ш-8 is the outcome of a failed claim, not itself
// claimable; the auto-detected codes (Ш-2/Ш-3/Ш-11/Ш-12) are not voted on.
func (c ShukhCode) isSubjective() bool { return c == Sh6 || c == Sh9 || c == Sh10 }
```

- [ ] **Step 4: Добавить тип `Adjudication` и поле в `engine/state.go`**

Перед объявлением `type State struct` добавить тип:

```go
// Adjudication is an open R-8.6 table vote over a subjective ШУХ. Claimant raised
// Code against Target; every seat 0..n-1 (including finished players, R-8.9/R-9.5)
// casts one vote. Votes[seat] == true means «support the challenge» (the ШУХ is
// bogus). On full turnout the vote auto-resolves: a table majority for the
// challenge moves the penalty onto Claimant as Ш-8, otherwise the ШУХ is confirmed
// on Target. While it is non-nil the table is frozen — only Vote is legal.
type Adjudication struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
	Votes    map[SeatID]bool
}
```

Внутри `type State struct`, сразу после поля `Unsettled *Unsettled`, добавить поле:

```go
	Adjudication *Adjudication // active R-8.6 table vote; nil = none (§8/§15.8: exclusive with Unsettled/Pending)
```

- [ ] **Step 5: Добавить `voterEligible` и `hasVoted` в `engine/state.go`**

В конец файла добавить:

```go
// voterEligible reports whether seat may cast an R-8.6 vote: any seat at the table
// (0..n-1), finished players included (R-8.9/R-9.5).
func (s State) voterEligible(seat SeatID) bool {
	return int(seat) >= 0 && int(seat) < len(s.Seats)
}

// hasVoted reports whether seat has already cast its vote in the active
// Adjudication. Precondition: s.Adjudication != nil.
func (s State) hasVoted(seat SeatID) bool {
	_, ok := s.Adjudication.Votes[seat]
	return ok
}
```

- [ ] **Step 6: Расширить `gatesClosed` и `clone` в `engine/apply.go`**

Заменить тело `gatesClosed`:

```go
func (s State) gatesClosed() bool {
	return s.Unsettled == nil && s.Pending == nil && s.Adjudication == nil
}
```

В `clone`, перед `return ns`, добавить глубокое копирование разбора:

```go
	if s.Adjudication != nil {
		cp := *s.Adjudication
		cp.Votes = make(map[SeatID]bool, len(s.Adjudication.Votes))
		for k, v := range s.Adjudication.Votes {
			cp.Votes[k] = v
		}
		ns.Adjudication = &cp
	}
```

- [ ] **Step 7: Запустить тесты — проходят**

Run: `go test ./engine/ -run 'TestSubjectiveCodesClassified|TestAdjudicationClosesGates'`
Expected: PASS.

- [ ] **Step 8: Полный прогон движка (регрессий нет)**

Run: `go test ./engine/`
Expected: `ok`.

- [ ] **Step 9: Commit**

```bash
git add engine/state.go engine/apply.go engine/adjudication_test.go
git commit -m "feat(engine): ШУХ-разбор — коды Ш-6/8/9/10, State.Adjudication, гейт (R-8.6)"
```

---

### Task 2: События `VoteOpened` / `VoteResolved`

**Files:**
- Modify: `engine/event.go`
- Test: `engine/adjudication_test.go` (Modify)

**Interfaces:**
- Produces: `engine.VoteOpened{Claimant, Target SeatID; Code ShukhCode}`; `engine.VoteResolved{Code ShukhCode; Overturned bool}` (оба реализуют `Event`).

- [ ] **Step 1: Тест — события реализуют `Event`**

Добавить в `engine/adjudication_test.go`:

```go
func TestVoteEventsAreEvents(t *testing.T) {
	var _ Event = VoteOpened{Claimant: 0, Target: 1, Code: Sh6}
	var _ Event = VoteResolved{Code: Sh6, Overturned: true}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestVoteEventsAreEvents`
Expected: FAIL — `undefined: VoteOpened / VoteResolved`.

- [ ] **Step 3: Добавить события в `engine/event.go`**

Перед блоком методов `isEvent()` добавить:

```go
// VoteOpened is emitted when a subjective ШУХ is claimed and the table vote opens
// (R-8.6): Claimant raised Code against Target.
type VoteOpened struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
}

// VoteResolved is emitted when an R-8.6 vote resolves. Overturned == true means the
// table backed the challenge and the ШУХ moved onto Claimant as Ш-8; false means
// the ШУХ was confirmed on Target.
type VoteResolved struct {
	Code      ShukhCode
	Overturned bool
}
```

В блок методов добавить:

```go
func (VoteOpened) isEvent()   {}
func (VoteResolved) isEvent() {}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./engine/ -run TestVoteEventsAreEvents`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/event.go engine/adjudication_test.go
git commit -m "feat(engine): события VoteOpened/VoteResolved (R-8.6)"
```

---

### Task 3: Действие `ClaimSubjective` — открыть разбор

**Files:**
- Modify: `engine/action.go`
- Modify: `engine/apply.go`
- Test: `engine/adjudication_test.go` (Modify)

**Interfaces:**
- Consumes: `engine.Adjudication`, `VoteOpened`, `isSubjective` (Task 1–2).
- Produces: `engine.ClaimSubjective{Claimant, Target SeatID; Code ShukhCode}` (реализует `Action`); ветка `ClaimSubjective` в `Apply`; кейс в `isLegal`.

- [ ] **Step 1: Тест — предъявление открывает разбор и эмитит `VoteOpened`**

Добавить в `engine/adjudication_test.go`. Хелпер строит малую живую позицию без гейтов:

```go
// playingState builds a minimal gates-closed 3-seat Playing state with the given
// hand sizes, for adjudication unit tests. Cards are arbitrary distinct spades —
// the adjudication path never inspects card identity, only counts.
func playingState(t *testing.T, sizes map[SeatID]int) State {
	t.Helper()
	s := State{
		Rules: RuleSet{DeckSize: Deck36},
		Mode:  Middle,
		Seats: []SeatID{0, 1, 2},
		Phase: Playing,
		Hands: map[SeatID][]Card{},
		Shukh: map[SeatID][]Card{},
		Live:  map[SeatID]bool{0: true, 1: true, 2: true},
		OwesOneCard:   map[SeatID]bool{},
		ShukhTakeable: map[SeatID]bool{},
	}
	rank := Rank(7)
	for seat, n := range sizes {
		for i := 0; i < n; i++ {
			s.Hands[seat] = append(s.Hands[seat], Card{Suit: Spades, Rank: rank})
			rank++
		}
	}
	return s
}

func TestClaimSubjectiveOpensVote(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, events, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	if ns.Adjudication == nil || ns.Adjudication.Target != 0 || ns.Adjudication.Code != Sh6 {
		t.Fatalf("expected open Adjudication over seat 0/Ш-6, got %+v", ns.Adjudication)
	}
	if len(ns.Adjudication.Votes) != 0 {
		t.Fatalf("fresh vote must have no ballots, got %v", ns.Adjudication.Votes)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d: %+v", len(events), events)
	}
	if _, ok := events[0].(VoteOpened); !ok {
		t.Fatalf("want VoteOpened, got %T", events[0])
	}
}

func TestClaimSubjectiveRejectsSelfAndNonSubjective(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	if _, _, err := Apply(s, ClaimSubjective{Claimant: 0, Target: 0, Code: Sh6}); err == nil {
		t.Error("claiming against self must be illegal")
	}
	if _, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh2}); err == nil {
		t.Error("a non-subjective code must be illegal for ClaimSubjective")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestClaimSubjective`
Expected: FAIL — `undefined: ClaimSubjective`.

- [ ] **Step 3: Добавить действие в `engine/action.go`**

После существующих `type ...Action` объявлений добавить:

```go
// ClaimSubjective raises a subjective ШУХ (Ш-6 «завис» / Ш-9 «зря крикнул» / Ш-10
// «небрежность») against Target, opening an R-8.6 table vote. Claimant carries the
// raising seat (P-1). No penalty applies until the vote resolves (D-10: subjective
// ШУХи go to предъявление+голосование). Legal only with all gates closed.
type ClaimSubjective struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
}
```

В блок `func (...) isAction() {}` добавить:

```go
func (ClaimSubjective) isAction() {}
```

- [ ] **Step 4: Добавить кейс `isLegal` и ветку `Apply` в `engine/apply.go`**

В `isLegal`, внутри `switch act := a.(type)`, перед `default:` добавить:

```go
	case ClaimSubjective:
		return s.gatesClosed() && act.Code.isSubjective() &&
			s.Live[act.Claimant] && s.Live[act.Target] && act.Claimant != act.Target
```

В `Apply`, внутри `switch act := a.(type)`, перед `default:` добавить:

```go
	case ClaimSubjective:
		ns.Adjudication = &Adjudication{
			Claimant: act.Claimant,
			Target:   act.Target,
			Code:     act.Code,
			Votes:    map[SeatID]bool{},
		}
		events = append(events, VoteOpened{Claimant: act.Claimant, Target: act.Target, Code: act.Code})
```

- [ ] **Step 5: Запустить — проходит**

Run: `go test ./engine/ -run TestClaimSubjective`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add engine/action.go engine/apply.go engine/adjudication_test.go
git commit -m "feat(engine): ClaimSubjective открывает разбор R-8.6 (Ш-6/9/10)"
```

---

### Task 4: Действие `Vote` — авто-разрешение при полной явке

**Files:**
- Modify: `engine/action.go`
- Modify: `engine/apply.go`
- Test: `engine/adjudication_test.go` (Modify)

**Interfaces:**
- Consumes: `ClaimSubjective`, `Adjudication`, `assessShukh` (уже в движке), `VoteResolved`.
- Produces: `engine.Vote{Voter SeatID; Support bool}` (реализует `Action`); ветка `Vote` в `Apply`; кейс `isLegal`; хелпер `(State).resolveAdjudication(*[]Event)`.

- [ ] **Step 1: Тест — большинство за оспаривание перекидывает Ш-8 на предъявителя; иначе ШУХ на цели**

Добавить в `engine/adjudication_test.go`:

```go
// voteOut casts all three seats' ballots and returns the resolved state + events.
func voteOut(t *testing.T, s State, ballots map[SeatID]bool) (State, []Event) {
	t.Helper()
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	var all []Event
	for _, seat := range []SeatID{0, 1, 2} {
		var evs []Event
		ns, evs, err = Apply(ns, Vote{Voter: seat, Support: ballots[seat]})
		if err != nil {
			t.Fatalf("vote by %d rejected: %v", seat, err)
		}
		all = append(all, evs...)
	}
	return ns, all
}

func TestVoteMajorityForChallengeFlipsToClaimant(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	// seats 0 and 2 support the challenge (2 of 3 > half) → Ш-8 onto claimant (seat 1).
	ns, events := voteOut(t, s, map[SeatID]bool{0: true, 1: false, 2: true})
	if ns.Adjudication != nil {
		t.Fatal("vote must clear the Adjudication")
	}
	assertResolved(t, events, true)
	// Ш-8 penalty on claimant (seat 1): others (0,2) with ≥2 cards owe → payment gate on seat 1.
	if ns.Pending == nil || ns.Pending.Offender != 1 {
		t.Fatalf("expected §8 payment gate for offender 1, got %+v", ns.Pending)
	}
}

func TestVoteNoMajorityConfirmsOnTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	// only seat 2 supports the challenge (1 of 3, not a majority) → ШУХ confirmed on target (seat 0).
	ns, events := voteOut(t, s, map[SeatID]bool{0: false, 1: false, 2: true})
	assertResolved(t, events, false)
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate for offender 0, got %+v", ns.Pending)
	}
}

func assertResolved(t *testing.T, events []Event, wantOverturned bool) {
	t.Helper()
	for _, e := range events {
		if r, ok := e.(VoteResolved); ok {
			if r.Overturned != wantOverturned {
				t.Fatalf("VoteResolved.Overturned = %v, want %v", r.Overturned, wantOverturned)
			}
			return
		}
	}
	t.Fatal("no VoteResolved event emitted")
}

func TestVoteGatesNormalActions(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	s.Turn = 0
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// While the vote is open, the seat to move cannot play a card.
	if got := LegalActions(ns, 0); len(got) != 2 {
		t.Fatalf("during a vote seat 0 may only cast 2 Vote options, got %d: %+v", len(got), got)
	}
	if _, _, err := Apply(ns, PlayCard{Card: ns.Hands[0][0]}); err == nil {
		t.Fatal("a normal move during an open vote must be illegal")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestVote`
Expected: FAIL — `undefined: Vote`.

- [ ] **Step 3: Добавить действие в `engine/action.go`**

```go
// Vote casts Voter's ballot in the open R-8.6 Adjudication. Support == true backs
// the challenge (the ШУХ is bogus → move it to the claimant as Ш-8). Any seat votes
// once (R-8.9, finished players included); on full turnout the vote auto-resolves.
type Vote struct {
	Voter   SeatID
	Support bool
}
```

В блок `isAction()`:

```go
func (Vote) isAction() {}
```

- [ ] **Step 4: Добавить `isLegal`, ветку `Apply`, `resolveAdjudication` в `engine/apply.go`**

В `isLegal`, перед `default:`:

```go
	case Vote:
		return s.Adjudication != nil && s.voterEligible(act.Voter) && !s.hasVoted(act.Voter)
```

В `Apply`, перед `default:`:

```go
	case Vote:
		ns.Adjudication.Votes[act.Voter] = act.Support
		if len(ns.Adjudication.Votes) == len(ns.Seats) {
			ns.resolveAdjudication(&events)
		}
```

Добавить хелпер (рядом с `assessShukh`):

```go
// resolveAdjudication tallies a fully-voted R-8.6 Adjudication (§8, R-8.6): a table
// majority backing the challenge (support*2 > n) moves the penalty onto the
// claimant as Ш-8, otherwise the ШУХ is confirmed on the target. Either way it
// clears the vote and enacts the outcome through the shared §8 machinery
// (assessShukh → payment gate or immediate effect). Precondition: Adjudication != nil
// and every seat has voted.
func (s *State) resolveAdjudication(events *[]Event) {
	adj := s.Adjudication
	support := 0
	for _, v := range adj.Votes {
		if v {
			support++
		}
	}
	overturned := support*2 > len(s.Seats)
	s.Adjudication = nil
	*events = append(*events, VoteResolved{Code: adj.Code, Overturned: overturned})
	if overturned {
		s.assessShukh(adj.Claimant, Sh8, events)
	} else {
		s.assessShukh(adj.Target, adj.Code, events)
	}
}
```

- [ ] **Step 5: Добавить раннюю ветку разбора в `engine/legal.go`**

В `LegalActions`, сразу после блока `if s.Phase == Finished { return nil }`, добавить:

```go
	// An open R-8.6 vote freezes the table (§15.8): the only legal action for any
	// not-yet-voted eligible seat is to cast its ballot. ClaimSubjective is not
	// enumerated — it is an always-available social button, validated on submit.
	if s.Adjudication != nil {
		if !s.voterEligible(seat) || s.hasVoted(seat) {
			return nil
		}
		return []Action{Vote{Voter: seat, Support: true}, Vote{Voter: seat, Support: false}}
	}
```

- [ ] **Step 6: Запустить — проходит**

Run: `go test ./engine/ -run TestVote`
Expected: PASS.

- [ ] **Step 7: Полный прогон движка + гонки**

Run: `go test -race ./engine/`
Expected: `ok`.

- [ ] **Step 8: Commit**

```bash
git add engine/action.go engine/apply.go engine/legal.go engine/adjudication_test.go
git commit -m "feat(engine): Vote + авто-разрешение разбора R-8.6 (кворум, Ш-8-переклад)"
```

---

### Task 5: Инвариант эксклюзивности гейтов + фаззер учитывает разбор

**Files:**
- Modify: `engine/invariants.go`
- Modify: `engine/adjudication_test.go`

**Interfaces:**
- Consumes: `State.Adjudication`, `State.Unsettled`, `State.Pending`.
- Produces: расширенная `CheckInvariants` (гейты §15.8 взаимоисключающи).

- [ ] **Step 1: Тест — одновременно открытые разбор и payment-гейт ловятся**

Добавить в `engine/adjudication_test.go`:

```go
func TestGatesAreMutuallyExclusive(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	s.Adjudication = &Adjudication{Claimant: 1, Target: 0, Code: Sh6, Votes: map[SeatID]bool{}}
	s.Pending = &Payment{Offender: 0, Owed: []SeatID{1}}
	if err := CheckInvariants(s); err == nil {
		t.Fatal("CheckInvariants must reject two open gates at once (§15.8)")
	}
}
```

- [ ] **Step 2: Запустить — падает (инвариант ещё не проверяет)**

Run: `go test ./engine/ -run TestGatesAreMutuallyExclusive`
Expected: FAIL — no error returned.

- [ ] **Step 3: Добавить проверку в `engine/invariants.go`**

Перед финальным `return nil` в `CheckInvariants` добавить:

```go
	// §15.8: at most one adjudication device is open at a time (catch-window,
	// payment gate, or R-8.6 vote) — they are enacted and cleared serially.
	open := 0
	if s.Unsettled != nil {
		open++
	}
	if s.Pending != nil {
		open++
	}
	if s.Adjudication != nil {
		open++
	}
	if open > 1 {
		return fmt.Errorf("engine: §15.8 violated: %d adjudication gates open at once", open)
	}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./engine/ -run TestGatesAreMutuallyExclusive`
Expected: PASS.

- [ ] **Step 5: Полный прогон движка (включая фаззеры как unit)**

Run: `go test ./engine/`
Expected: `ok`. (Существующие фаззеры гоняются на сид-корпусе; действия голосования они не генерируют — регрессий быть не должно.)

- [ ] **Step 6: Commit**

```bash
git add engine/invariants.go engine/adjudication_test.go
git commit -m "feat(engine): инвариант §15.8 — гейты (окно/оплата/разбор) взаимоисключающи"
```

---

# ФАЗА 2 — Пакет `game/`: Session

### Task 6: Скелет пакета — идентичность, лобби (`NewSession`/`Join`/`Leave`)

**Files:**
- Create: `game/session.go`
- Test: `game/session_test.go` (Create)

**Interfaces:**
- Consumes: `engine.RuleSet`, `engine.EnforcementMode`, `engine.Config`, `engine.Player`, `engine.State`.
- Produces: `game.PlayerID string`; `game.Lifecycle` (`Lobby|Playing|Finished`); `game.Config{Rules engine.RuleSet; Mode engine.EnforcementMode}`; `game.Session`; `func NewSession(cfg Config, host PlayerID, hostName string) *Session`; `func (*Session) Join(PlayerID, string) error`; `func (*Session) Leave(PlayerID)`; `func (*Session) Stage() Lifecycle`; `func (*Session) seatOf(PlayerID) (engine.SeatID, bool)`; ошибки `ErrNotLobby, ErrNotHost, ErrFull, ErrDuplicate, ErrUnknownPlayer, ErrNotPlaying, ErrNotYours, ErrTooFewPlayers`.

- [ ] **Step 1: Тест — лобби принимает игроков, отклоняет дубликат и переполнение**

Создать `game/session_test.go`:

```go
package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func cfg36() Config {
	return Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
}

func TestLobbyJoin(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if s.Stage() != Lobby {
		t.Fatalf("new session must be in Lobby, got %v", s.Stage())
	}
	if _, ok := s.seatOf("h"); !ok {
		t.Fatal("host must occupy a seat")
	}
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatalf("join rejected: %v", err)
	}
	if err := s.Join("p2", "Bob again"); err != ErrDuplicate {
		t.Fatalf("duplicate join: want ErrDuplicate, got %v", err)
	}
}

func TestLobbyFull(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	// host + 7 more = 8 (D-3 max)
	for i := 0; i < 7; i++ {
		if err := s.Join(PlayerID(rune('a'+i)), "x"); err != nil {
			t.Fatalf("join %d rejected: %v", i, err)
		}
	}
	if err := s.Join("overflow", "x"); err != ErrFull {
		t.Fatalf("9th join: want ErrFull, got %v", err)
	}
}

func TestLeaveInLobby(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	s.Leave("p2")
	if _, ok := s.seatOf("p2"); ok {
		t.Fatal("left player must lose its seat in Lobby")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/`
Expected: FAIL — `no required module provides package .../game` / undefined symbols.

- [ ] **Step 3: Написать `game/session.go`**

```go
// Package game is Layer 1 (D-6): transport-agnostic orchestration of a single live
// «Шух» game over the pure engine. A Session holds authoritative state, maps
// players to seats, runs the Lobby→Playing→Finished lifecycle, accepts actions, and
// projects a per-seat view + event stream. It knows nothing about sockets, room
// codes, or reconnect tokens — those live in Layer 2.
package game

import (
	"errors"
	"sync"

	"github.com/oustrix/shukh/engine"
)

// PlayerID is a stable, opaque identity handed in by Layer 2. Session only stores
// and compares it; it never interprets it.
type PlayerID string

// Lifecycle is the coarse stage of a session.
type Lifecycle int

const (
	Lobby Lifecycle = iota
	Playing
	Finished
)

// Config is the per-match setup chosen in the lobby (D-8/D-10): deck rules and
// enforcement mode. The player roster is assembled by Session from join order.
type Config struct {
	Rules engine.RuleSet
	Mode  engine.EnforcementMode
}

// Lobby / lifecycle errors.
var (
	ErrNotLobby      = errors.New("game: action allowed only in the lobby")
	ErrNotHost       = errors.New("game: only the host may do this")
	ErrFull          = errors.New("game: table is full (max 8, D-3)")
	ErrDuplicate     = errors.New("game: player already joined")
	ErrUnknownPlayer = errors.New("game: unknown player")
	ErrNotPlaying    = errors.New("game: game is not in progress")
	ErrNotYours      = errors.New("game: action does not belong to this player")
	ErrTooFewPlayers = errors.New("game: need at least 2 players to start (D-3)")
)

const maxPlayers = 8

// Session is the synchronous, mutex-guarded orchestrator of one game (S-3).
type Session struct {
	mu    sync.Mutex
	cfg   Config
	host  PlayerID
	stage Lifecycle

	order []PlayerID          // join order → seat index (R-2.13 clockwise seating)
	names map[PlayerID]string // display names

	state engine.State // authoritative; valid once stage >= Playing

	subs map[PlayerID]*subscriber // populated in Task 9
}

// NewSession creates a lobby seated by the host (seat 0, the eventual shuffler R-4.7).
func NewSession(cfg Config, host PlayerID, hostName string) *Session {
	return &Session{
		cfg:   cfg,
		host:  host,
		stage: Lobby,
		order: []PlayerID{host},
		names: map[PlayerID]string{host: hostName},
		subs:  map[PlayerID]*subscriber{},
	}
}

// Stage returns the current lifecycle stage.
func (s *Session) Stage() Lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stage
}

// Join seats a new player at the end of the clockwise order. Lobby only.
func (s *Session) Join(id PlayerID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if _, ok := s.names[id]; ok {
		return ErrDuplicate
	}
	if len(s.order) >= maxPlayers {
		return ErrFull
	}
	s.order = append(s.order, id)
	s.names[id] = name
	return nil
}

// Leave removes a player from the lobby (mid-game leave is a Layer-2 disconnect
// concern, out of scope here). No-op if the game has started or the player is absent.
func (s *Session) Leave(id PlayerID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return
	}
	if _, ok := s.names[id]; !ok {
		return
	}
	delete(s.names, id)
	for i, p := range s.order {
		if p == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// seatOf maps a player to its seat index (its position in join order). The caller
// must hold s.mu, except tests that never race.
func (s *Session) seatOf(id PlayerID) (engine.SeatID, bool) {
	for i, p := range s.order {
		if p == id {
			return engine.SeatID(i), true
		}
	}
	return 0, false
}
```

> **Note:** `subscriber` is defined in Task 9. Until then the `subs` field compiles as `map[PlayerID]*subscriber` only if the type exists — so add a temporary stub at the bottom of `session.go` now and delete it in Task 9:
>
> ```go
> // temporary stub — replaced in Task 9 (subscribe.go)
> type subscriber struct{}
> ```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "feat(game): скелет Session — идентичность PlayerID, лобби (Join/Leave)"
```

---

### Task 7: `SetConfig` + `Start` — старт партии через shuffle

**Files:**
- Modify: `game/session.go`
- Test: `game/session_test.go` (Modify)

**Interfaces:**
- Consumes: `NewSession`, `Config`, `engine.NewDeck`, `engine.NewGame`, `shuffle.Deck`.
- Produces: `func (*Session) SetConfig(host PlayerID, cfg Config) error`; `func (*Session) Start(host PlayerID, seed int64) error`.

- [ ] **Step 1: Тест — только хост стартует, ≥2 игроков, переход в Playing**

Добавить в `game/session_test.go`:

```go
func TestStartRequiresHostAndTwoPlayers(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Start("h", 1); err != ErrTooFewPlayers {
		t.Fatalf("solo start: want ErrTooFewPlayers, got %v", err)
	}
	_ = s.Join("p2", "Bob")
	if err := s.Start("p2", 1); err != ErrNotHost {
		t.Fatalf("non-host start: want ErrNotHost, got %v", err)
	}
	if err := s.Start("h", 42); err != nil {
		t.Fatalf("host start rejected: %v", err)
	}
	if s.Stage() != Playing {
		t.Fatalf("after Start stage must be Playing, got %v", s.Stage())
	}
	if err := s.Start("h", 42); err != ErrNotLobby {
		t.Fatalf("double start: want ErrNotLobby, got %v", err)
	}
}

func TestSetConfigHostLobbyOnly(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	c52 := Config{Rules: engine.RuleSet{DeckSize: engine.Deck52}, Mode: engine.Guard}
	if err := s.SetConfig("p2", c52); err != ErrNotHost {
		t.Fatalf("non-host SetConfig: want ErrNotHost, got %v", err)
	}
	if err := s.SetConfig("h", c52); err != nil {
		t.Fatalf("host SetConfig rejected: %v", err)
	}
	_ = s.Join("p2", "Bob")
	_ = s.Start("h", 7)
	if err := s.SetConfig("h", cfg36()); err != ErrNotLobby {
		t.Fatalf("SetConfig after start: want ErrNotLobby, got %v", err)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/ -run 'TestStart|TestSetConfig'`
Expected: FAIL — `undefined: (*Session).Start / SetConfig`.

- [ ] **Step 3: Добавить `SetConfig` и `Start` в `game/session.go`**

Добавить импорт `shuffle` (в блок import):

```go
	"github.com/oustrix/shukh/shuffle"
```

Методы:

```go
// SetConfig changes the match config before the game starts. Host + Lobby only.
func (s *Session) SetConfig(host PlayerID, cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if host != s.host {
		return ErrNotHost
	}
	s.cfg = cfg
	return nil
}

// Start deals a fresh game: it builds the engine.Config from the roster (join order
// = clockwise seating, R-2.13), shuffles a canonical deck by seed at the D-11
// boundary, and runs engine.NewGame. Host + Lobby + ≥2 players only. On success the
// stage becomes Playing (or Finished if the game somehow ends immediately).
func (s *Session) Start(host PlayerID, seed int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Lobby {
		return ErrNotLobby
	}
	if host != s.host {
		return ErrNotHost
	}
	if len(s.order) < 2 {
		return ErrTooFewPlayers
	}
	players := make([]engine.Player, len(s.order))
	for i, id := range s.order {
		players[i] = engine.Player{Name: s.names[id]}
	}
	ecfg := engine.Config{Rules: s.cfg.Rules, Mode: s.cfg.Mode, Players: players}
	deck := shuffle.Deck(engine.NewDeck(s.cfg.Rules), seed)
	st, _, err := engine.NewGame(ecfg, deck)
	if err != nil {
		return err
	}
	s.state = st
	s.stage = Playing
	if st.Phase == engine.Finished {
		s.stage = Finished
	}
	return nil
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/ -run 'TestStart|TestSetConfig'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "feat(game): SetConfig + Start (роспись роли, shuffle+NewGame, D-11)"
```

---

### Task 8: Проекция — `Update`, `SeatMeta`, `Snapshot`

**Files:**
- Create: `game/projection.go`
- Test: `game/projection_test.go` (Create)

**Interfaces:**
- Consumes: `engine.SeatView`, `engine.View`, `engine.LegalActions`, `engine.Action`, `engine.Event`, `Session.state`, `Session.order/names`.
- Produces: `game.SeatMeta{Seat engine.SeatID; Name string}`; `game.Update{Stage Lifecycle; Roster []SeatMeta; View *engine.SeatView; Legal []engine.Action; Events []engine.Event}`; `func (*Session) Snapshot(id PlayerID) (Update, error)`; неэкспортируемый `(*Session) project(id PlayerID, events []engine.Event) Update` (держатель мьютекса).

> **Design note (deviation from spec §5):** the spec's `Update{View, Legal, Events}`
> is extended here with `Stage` and `Roster`, and `View` is a pointer (nil in Lobby).
> This matches the web `GameSnapshot{seats, view|null, legal}` the spec says to mirror,
> and lets one type serve both Lobby (roster, no view) and Playing (full projection).

- [ ] **Step 1: Тест — снапшот скрывает руки соперников (D-9); лобби без view**

Создать `game/projection_test.go`:

```go
package game

import (
	"testing"
)

func TestSnapshotLobbyHasRosterNoView(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	up, err := s.Snapshot("h")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if up.Stage != Lobby || up.View != nil {
		t.Fatalf("lobby snapshot must have nil View, got stage=%v view=%v", up.Stage, up.View)
	}
	if len(up.Roster) != 2 || up.Roster[0].Name != "Host" || up.Roster[1].Name != "Bob" {
		t.Fatalf("roster wrong: %+v", up.Roster)
	}
}

func TestSnapshotPlayingHidesOpponents(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	_ = s.Start("h", 42)
	up, err := s.Snapshot("h")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if up.View == nil {
		t.Fatal("playing snapshot must carry a View")
	}
	if up.View.You != 0 {
		t.Fatalf("host is seat 0, got You=%d", up.View.You)
	}
	// D-9: opponents are counts only — there is no card field on OpponentView.
	if len(up.View.Opponents) != 1 {
		t.Fatalf("want 1 opponent, got %d", len(up.View.Opponents))
	}
	if up.View.Opponents[0].HandCount == 0 {
		t.Fatal("opponent hand count should be public and non-zero after dealing")
	}
}

func TestSnapshotUnknownPlayer(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if _, err := s.Snapshot("ghost"); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/ -run TestSnapshot`
Expected: FAIL — `undefined: (*Session).Snapshot / Update / SeatMeta`.

- [ ] **Step 3: Написать `game/projection.go`**

```go
package game

import "github.com/oustrix/shukh/engine"

// SeatMeta is the public identity of a seat: its index and display name.
type SeatMeta struct {
	Seat engine.SeatID
	Name string
}

// Update is what a subscriber receives after each change (mirrors the web
// GameSnapshot + event stream, S-6). View is nil in the Lobby; once Playing it is
// the per-seat projection (D-9). Legal is the seat's legal actions; Events are the
// engine events produced by the change that triggered this Update (empty for a
// plain Snapshot).
type Update struct {
	Stage  Lifecycle
	Roster []SeatMeta
	View   *engine.SeatView
	Legal  []engine.Action
	Events []engine.Event
}

// roster builds the public seat list in clockwise (join) order. Caller holds s.mu.
func (s *Session) roster() []SeatMeta {
	out := make([]SeatMeta, len(s.order))
	for i, id := range s.order {
		out[i] = SeatMeta{Seat: engine.SeatID(i), Name: s.names[id]}
	}
	return out
}

// project builds an Update for id carrying the given events. Caller holds s.mu and
// has verified id is seated.
func (s *Session) project(id PlayerID, events []engine.Event) Update {
	seat, _ := s.seatOf(id)
	up := Update{
		Stage:  s.stage,
		Roster: s.roster(),
		Legal:  nil,
		Events: events,
	}
	if s.stage != Lobby {
		v := engine.View(s.state, seat)
		up.View = &v
		up.Legal = engine.LegalActions(s.state, seat)
	}
	return up
}

// Snapshot returns the current projection for id (no events). Errors if id is not
// seated.
func (s *Session) Snapshot(id PlayerID) (Update, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return Update{}, ErrUnknownPlayer
	}
	return s.project(id, nil), nil
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/ -run TestSnapshot`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add game/projection.go game/projection_test.go
git commit -m "feat(game): проекция Update/Snapshot — per-seat вид (D-9) + ростер"
```

---

### Task 9: Подписки и фанаут

**Files:**
- Create: `game/subscribe.go`
- Modify: `game/session.go` (удалить временный стаб `subscriber`)
- Test: `game/subscribe_test.go` (Create)

**Interfaces:**
- Consumes: `Session`, `project`, `Update`.
- Produces: неэкспортируемый `subscriber{ch chan Update; stale bool}`; `func (*Session) Subscribe(id PlayerID) (<-chan Update, func(), error)`; неэкспортируемый `(*Session) fanout(map[PlayerID][]engine.Event)` — вызывается держателем мьютекса после мутации.

- [ ] **Step 1: Тест — подписчик получает обновление; отписка закрывает канал**

Создать `game/subscribe_test.go`:

```go
package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func TestSubscribeReceivesFanout(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	ch, _, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	<-ch // drain the initial snapshot Subscribe delivers
	// Directly exercise fanout with a synthetic event.
	s.mu.Lock()
	s.fanout(map[PlayerID][]engine.Event{"h": {engine.OneCardDeclared{Seat: 0}}})
	s.mu.Unlock()
	select {
	case up := <-ch:
		if len(up.Events) != 1 {
			t.Fatalf("want 1 event delivered, got %d", len(up.Events))
		}
	default:
		t.Fatal("expected an Update on the subscriber channel")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	ch, cancel, _ := s.Subscribe("h")
	<-ch // drain the initial snapshot so the next receive reflects closure
	cancel()
	if _, open := <-ch; open {
		t.Fatal("unsubscribe must close the channel")
	}
}

func TestSubscribeUnknownPlayer(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if _, _, err := s.Subscribe("ghost"); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/ -run 'TestSubscribe|TestUnsubscribe'`
Expected: FAIL — `undefined: (*Session).Subscribe / fanout`.

- [ ] **Step 3: Удалить временный стаб в `game/session.go`**

Удалить строки, добавленные в Task 6 Step 3:

```go
// temporary stub — replaced in Task 9 (subscribe.go)
type subscriber struct{}
```

- [ ] **Step 4: Написать `game/subscribe.go`**

```go
package game

import "github.com/oustrix/shukh/engine"

// subCapacity bounds a subscriber's buffer. A consumer slower than this many
// pending Updates is marked stale (its delta is dropped) and must re-Snapshot; the
// game never blocks on a slow client.
const subCapacity = 16

type subscriber struct {
	ch    chan Update
	stale bool
}

// Subscribe registers id for push Updates and immediately delivers a snapshot.
// The returned func() unsubscribes (closing the channel). Errors if id is not seated.
func (s *Session) Subscribe(id PlayerID) (<-chan Update, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return nil, nil, ErrUnknownPlayer
	}
	sub := &subscriber{ch: make(chan Update, subCapacity)}
	s.subs[id] = sub
	sub.ch <- s.project(id, nil) // initial snapshot fits (fresh buffer)
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if cur, ok := s.subs[id]; ok && cur == sub {
			delete(s.subs, id)
			close(sub.ch)
		}
	}
	return sub.ch, cancel, nil
}

// fanout pushes a per-seat Update to every subscriber. perSeat maps a player to the
// events relevant to it (typically the same event list for all). A non-blocking
// send drops the delta and marks the subscriber stale when its buffer is full; the
// client recovers via Snapshot. Caller holds s.mu.
func (s *Session) fanout(perSeat map[PlayerID][]engine.Event) {
	for id, sub := range s.subs {
		up := s.project(id, perSeat[id])
		select {
		case sub.ch <- up:
			sub.stale = false
		default:
			sub.stale = true // buffer full: client must re-Snapshot
		}
	}
}
```

- [ ] **Step 5: Запустить — проходит; полный прогон пакета**

Run: `go test ./game/ -run 'TestSubscribe|TestUnsubscribe'`
Expected: PASS.
Run: `go test ./game/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add game/subscribe.go game/session.go game/subscribe_test.go
git commit -m "feat(game): подписки + неблокирующий фанаут (slow-consumer → re-Snapshot)"
```

---

### Task 10: `Submit` — приём действий с анти-имперсонацией

**Files:**
- Create: `game/submit.go`
- Test: `game/submit_test.go` (Create)

**Interfaces:**
- Consumes: `Session`, `engine.Apply`, `engine.CheckInvariants`, `engine.Action`, all action types, `fanout`.
- Produces: `func (*Session) Submit(id PlayerID, a engine.Action) ([]engine.Event, error)`; неэкспортируемый `(*Session) authorize(sub engine.SeatID, a engine.Action) error`.

- [ ] **Step 1: Тест — нельзя ходить за другого; легальный ход двигает игру и фанаутится**

Создать `game/submit_test.go`:

```go
package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// startedDuel returns a 2-player Playing session (host=seat0, "p2"=seat1) on a
// fixed seed, plus the host's view to read legal actions from.
func startedDuel(t *testing.T) *Session {
	t.Helper()
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.Start("h", 42); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSubmitRejectsOffTurnImpersonation(t *testing.T) {
	s := startedDuel(t)
	up, _ := s.Snapshot("h")
	// Find whichever player is NOT to move and have them try a turn-action.
	mover := up.View.Turn
	var idler PlayerID = "h"
	if mover == 0 {
		idler = "p2"
	}
	// idler tries to play the first card in *its own* hand out of turn.
	idlerUp, _ := s.Snapshot(idler)
	if len(idlerUp.View.Hand) == 0 {
		t.Skip("idler has no cards to attempt with")
	}
	_, err := s.Submit(idler, engine.PlayCard{Card: idlerUp.View.Hand[0]})
	if err != ErrNotYours {
		t.Fatalf("out-of-turn play: want ErrNotYours, got %v", err)
	}
}

func TestSubmitVoterMustBeSelf(t *testing.T) {
	s := startedDuel(t)
	// host raises a subjective ШУХ against p2 (seat 1).
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// host tries to cast p2's ballot → impersonation.
	if _, err := s.Submit("h", engine.Vote{Voter: 1, Support: true}); err != ErrNotYours {
		t.Fatalf("voting as another seat: want ErrNotYours, got %v", err)
	}
	// host casts its own ballot → ok.
	if _, err := s.Submit("h", engine.Vote{Voter: 0, Support: false}); err != nil {
		t.Fatalf("own vote rejected: %v", err)
	}
}

func TestSubmitUnknownPlayer(t *testing.T) {
	s := startedDuel(t)
	if _, err := s.Submit("ghost", engine.TakeBottomAndPass{}); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}

func TestSubmitNotPlaying(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host") // still Lobby
	if _, err := s.Submit("h", engine.TakeBottomAndPass{}); err != ErrNotPlaying {
		t.Fatalf("want ErrNotPlaying, got %v", err)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/ -run TestSubmit`
Expected: FAIL — `undefined: (*Session).Submit`.

- [ ] **Step 3: Написать `game/submit.go`**

```go
package game

import "github.com/oustrix/shukh/engine"

// Submit applies a on behalf of id. It maps id to a seat, rejects impersonation
// (acting as another seat), then defers rule legality to engine.Apply. On success
// it advances authoritative state, updates the lifecycle, fans out to subscribers,
// and returns the events. On any rejection state is untouched and nothing is
// fanned out.
func (s *Session) Submit(id PlayerID, a engine.Action) ([]engine.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage != Playing {
		if _, ok := s.seatOf(id); !ok {
			return nil, ErrUnknownPlayer
		}
		return nil, ErrNotPlaying
	}
	seat, ok := s.seatOf(id)
	if !ok {
		return nil, ErrUnknownPlayer
	}
	if err := s.authorize(seat, a); err != nil {
		return nil, err
	}
	ns, events, err := engine.Apply(s.state, a)
	if err != nil {
		return nil, err // engine.IllegalAction, state untouched
	}
	if inv := engine.CheckInvariants(ns); inv != nil {
		// A broken invariant after Apply is an engine bug — surface it, do not commit
		// the corrupt state.
		return nil, inv
	}
	s.state = ns
	if ns.Phase == engine.Finished {
		s.stage = Finished
	}
	perSeat := map[PlayerID][]engine.Event{}
	for _, pid := range s.order {
		perSeat[pid] = events
	}
	s.fanout(perSeat)
	return events, nil
}

// authorize rejects acting as a seat other than sub (anti-impersonation). Rule
// legality (whose turn, whether a gate is open) is left to engine.Apply; this only
// guards identity. Actor-agnostic social actions (AskCount/AskAboutWest/ClaimShukh,
// P-1) may be raised by any seated player.
func (s *Session) authorize(sub engine.SeatID, a engine.Action) error {
	switch act := a.(type) {
	case engine.ClaimSubjective:
		if act.Claimant != sub {
			return ErrNotYours
		}
	case engine.Vote:
		if act.Voter != sub {
			return ErrNotYours
		}
	case engine.DeclareOneCard:
		if act.Seat != sub {
			return ErrNotYours
		}
	case engine.TakeShukhCards:
		if act.Seat != sub {
			return ErrNotYours
		}
	case engine.GiveShukhCard:
		if s.state.Pending == nil || len(s.state.Pending.Owed) == 0 || s.state.Pending.Owed[0] != sub {
			return ErrNotYours
		}
	case engine.AskCount, engine.AskAboutWest, engine.ClaimShukh:
		// actor-agnostic (P-1): any seated player may raise; engine validates the rest.
	default:
		// turn-actions (PlayCard, TakeBottomAndPass, PodkladkaWest, DiscardWest): the
		// actor is the seat to move.
		if s.state.Turn != sub {
			return ErrNotYours
		}
	}
	return nil
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/ -run TestSubmit`
Expected: PASS.

- [ ] **Step 5: Полный прогон пакета + гонки**

Run: `go test -race ./game/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add game/submit.go game/submit_test.go
git commit -m "feat(game): Submit — анти-имперсонация + engine.Apply + фанаут"
```

---

### Task 11: Сквозной интеграционный тест — партия до конца + разбор ШУХа

**Files:**
- Create: `game/integration_test.go`

**Interfaces:**
- Consumes: весь публичный API `game`.

- [ ] **Step 1: Тест — дуэль играется до `Finished` через legal-driven ходы**

Создать `game/integration_test.go`. Драйвер должен быть устойчив: активным игроком
может быть не только `Turn` (при §8 payment-гейте ходит head payer), а «первый legal»
у `Turn`-игрока в окне Ш-2 может оказаться `ClaimShukh` и открыть гейт. Поэтому драйвер
играет **первый forward-ход по всем сиденьям** — только продвигающие партию действия
(`PlayCard`/`TakeBottomAndPass`/`PodkladkaWest`/`DiscardWest`/`GiveShukhCard`), не трогая
claim/vote/ask/social. В чистой дуэли у активного сиденья всегда есть forward-ход, так что
партия конечна:

```go
package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// forward reports whether a is a game-advancing move (not a claim/vote/ask/social
// action). The integration driver only plays these, so it never opens a vote or
// dispute — the duel converges to Finished.
func forward(a engine.Action) bool {
	switch a.(type) {
	case engine.PlayCard, engine.TakeBottomAndPass, engine.PodkladkaWest,
		engine.DiscardWest, engine.GiveShukhCard:
		return true
	default:
		return false
	}
}

// nextForward scans every seat for the first available forward move, returning the
// player to submit it and the action. ok=false means no seat has a forward move.
func nextForward(s *Session) (PlayerID, engine.Action, bool) {
	for _, id := range []PlayerID{"h", "p2"} {
		up, err := s.Snapshot(id)
		if err != nil {
			continue
		}
		for _, a := range up.Legal {
			if forward(a) {
				return id, a, true
			}
		}
	}
	return "", nil, false
}

func TestIntegrationDuelPlaysToFinish(t *testing.T) {
	s := startedDuel(t)
	for step := 0; step < 400; step++ {
		if s.Stage() == Finished {
			break
		}
		mover, a, ok := nextForward(s)
		if !ok {
			t.Fatalf("step %d: no seat has a forward move; state stuck", step)
		}
		if _, err := s.Submit(mover, a); err != nil {
			t.Fatalf("step %d: submit %T by %s rejected: %v", step, a, mover, err)
		}
	}
	if s.Stage() != Finished {
		t.Fatal("game did not finish within the step budget")
	}
	// After finish, the engine state carries the full ranking (R-9.2/R-10.1).
	final, _ := s.Snapshot("h")
	if len(final.View.Finish) != 2 {
		t.Fatalf("finished game must rank both players, got %v", final.View.Finish)
	}
}

func TestIntegrationSubjectiveShukhVote(t *testing.T) {
	s := startedDuel(t)
	// Host claims Ш-6 against p2; both seats vote; vote resolves and clears.
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	up, _ := s.Snapshot("p2")
	if len(up.Legal) != 2 {
		t.Fatalf("during the vote p2 must see 2 Vote options, got %d", len(up.Legal))
	}
	if _, err := s.Submit("p2", engine.Vote{Voter: 1, Support: true}); err != nil {
		t.Fatalf("p2 vote rejected: %v", err)
	}
	evs, err := s.Submit("h", engine.Vote{Voter: 0, Support: false})
	if err != nil {
		t.Fatalf("host vote rejected: %v", err)
	}
	sawResolved := false
	for _, e := range evs {
		if _, ok := e.(engine.VoteResolved); ok {
			sawResolved = true
		}
	}
	if !sawResolved {
		t.Fatal("full turnout must emit VoteResolved")
	}
	after, _ := s.Snapshot("h")
	// vote cleared → normal play resumes (or a §8 payment gate is open, but the
	// Adjudication itself is gone): the mover has legal actions again.
	if len(after.Legal) == 0 && after.Stage == Playing {
		t.Fatal("after the vote the game must be playable again")
	}
}
```

- [ ] **Step 2: Запустить — проходит**

Run: `go test ./game/ -run TestIntegration -v`
Expected: PASS (оба теста).

> **Если `TestIntegrationDuelPlaysToFinish` зациклился/упал:** это не флейк — значит
> драйвер «первый legal» упёрся в позицию (напр. бесконечная передача низа). Не
> поднимать лимит шагов вслепую: залогировать `up.Legal[0]` типы на последних шагах
> и убедиться, что движок реально сходится (в дуэли партия конечна). Диагностику
> вести через `superpowers:systematic-debugging`.

- [ ] **Step 3: Финальный прогон всего репозитория с гонками**

Run: `go test -race ./...`
Expected: `ok  engine`, `ok  game`, `ok  shuffle`.

- [ ] **Step 4: Commit**

```bash
git add game/integration_test.go
git commit -m "test(game): сквозняк — дуэль до финиша + разбор субъективного ШУХа"
```

---

### Task 12: Обновить дорожную карту в архитектуре

**Files:**
- Modify: `docs/architecture.md`

- [ ] **Step 1: Отметить Слой 1 Спеца 2 в журнале изменений**

В `docs/architecture.md`, в конец списка «## 7. Журнал изменений», добавить строку (дата — 2026-07-16):

```markdown
- **2026-07-16.** Реализован Слой 1 Спеца 2: пакет `game.Session` (Lobby→Playing→Finished,
  per-seat проекция + фанаут, анти-имперсонация) + достройка движка — голосование R-8.6
  (`State.Adjudication`, `ClaimSubjective`/`Vote`, субъективные Ш-6/8/9/10). Слой 2
  (WS + комнаты + реконнект) — следующий спек.
```

- [ ] **Step 2: Прогон + Commit**

Run: `go build ./... && go test ./...`
Expected: всё зелёное.

```bash
git add docs/architecture.md
git commit -m "docs(architecture): журнал — Слой 1 Спеца 2 (game.Session + R-8.6)"
```

---

## Замечания по исполнению

- **Порядок фаз строгий:** Фаза 2 зависит от новых действий/событий Фазы 1. Не начинать `game/` до зелёной Фазы 1.
- **Слой 2 вне этого плана:** WS-сервер, комнаты по коду, реконнект-токены, тайм-ауты (OQ-2) — следующий спек. `Session` спроектирована швом под них (синхронный API + `Update` = веб-контракт).
- **Ранний доигрыш голосования** (кто-то не голосует) в MVP не решается: движок авто-разрешает лишь при полной явке. Продуктовый UX (все проголосуют) + тайм-ауты — Слой 2. Это осознанное ограничение, не баг.
