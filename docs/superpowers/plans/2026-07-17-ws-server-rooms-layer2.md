# Спец 2 (Слой 2) — WebSocket-сервер + комнаты + реконнект — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Построить сетевой адаптер над `game.Session` — WebSocket-сервер с комнатами по коду/ссылке и реконнектом по HttpOnly-куке — плюс достроить вынесенные из ревью Слоя 1 прерогативы Слоёв 0/1 (тайм-аут голосования, проекция разбора, close-and-replace, снимок/восстановление сессии).

**Architecture:** Три фазы. **Фаза A** аддитивно расширяет чистый Слой 0 (`engine/`: `CloseVote`, `VoteView`) и Слой 1 (`game/`: close-and-replace в `Subscribe`, системный `CloseVote()`, миграция хоста, `Snapshot`/`Restore`). **Фаза B** строит новый пакет `server/` (Слой 2): `Clock`-шов, генерация кодов/токенов, `RoomStore`+`MemStore`, кодек протокола ↔ веб-контракт, `Room` (обёртка сессии + токены + таймеры), `Hub` (реестр + GC), WS-соединение, HTTP-хендлеры, бинарник. **Фаза C** — сквозной интеграционный тест и обновление документации.

**Tech Stack:** Go (stdlib: `net/http`, `context`, `sync`, `crypto/rand`, `encoding/json`, `time`, `testing`). Единственная новая зависимость — `github.com/coder/websocket` (добавляется inline-командой GOPROXY, см. Global Constraints). Внутренние пакеты `engine`, `game`, `shuffle`.

**Design doc:** [`docs/superpowers/specs/2026-07-17-ws-server-rooms-layer2-design.md`](../specs/2026-07-17-ws-server-rooms-layer2-design.md).

## Global Constraints

- **Слой 0 (`engine/`) остаётся чистым** — без I/O, сети, времени, RNG (D-6/D-7). `CloseVote` детерминирован, `VoteView` — данные. Фаза A это сохраняет.
- **Слой 1 (`game/`) синхронен под `sync.Mutex`** (S-3); никаких внутренних горутин; фанаут — неблокирующая отправка в буферизованные каналы.
- **Слой 2 (`server/`) — тонкий адаптер:** никакой игровой логики; правила — в движке (D-1), состояние/процедуры — в `game.Session`. Слой 2 добавляет только транспорт, коды, токены, часы, GC.
- **Правила — источник истины** [`docs/shukh-rules.md`](../../shukh-rules.md); архитектурные решения — [`docs/architecture.md`](../../architecture.md) (`D-n`), решения этого спека — `L2-n` в дизайн-доке.
- **Тайм-аут голосования R-8.6** (L2-1): таймер живёт в `server/` (`Clock`), резолв — детерминированным `engine.CloseVote`; формула `противШУХа*2 > n` неизменна, молчание = «За ШУХ».
- **Реконнект по HttpOnly-куке** (L2-6, ревизует D-2): инвайт-ссылка несёт только код; личный токен — в куке пути комнаты, выдаётся HTTP-шагом `join`. Токен = случайный секрет ~128 бит без подписи (L2-7).
- **`seed` тасовки — серверный** (crypto/rand или `Clock.Now`), клиент его не выбирает (L2-8). Тесты инжектят детерминированное время.
- **Время — за швом `Clock`** (L2-9): реальный в проде, фейковый в тестах; таймеры проверяются «прокруткой», без реальных `sleep`.
- **In-memory за швом `RoomStore`** (L2-5): дефолт `MemStore`; долговечная часть сериализуема, эфемерная (сокеты/каналы/таймеры) пересобирается.
- **Веб (`web/`) НЕ трогаем** — там другой агент. Расхождения контракта фиксируются задачами для веб-стороны (Task 20), владелец передаст их веб-агенту.
- **Добавление зависимости — только inline GOPROXY**, глобальный дефолт не трогать:
  `GOPROXY="$(go env GOPROXY),https://proxy.golang.org,direct" go get github.com/coder/websocket@latest`
- **Коммиты частые**, по одному на задачу; сообщения в стиле репозитория (`feat(engine)`, `feat(game)`, `feat(server)`, `test(...)`, `docs(...)`).
- **Проверка перед коммитом:** `go build ./...` и `go test ./...` зелёные; на задачах с конкурентностью — `go test -race ./...`.

---

## Файловая структура

**Фаза A (аддитивно правит существующие Слои 0/1):**
- `engine/action.go` — + `CloseVote` (Task 1).
- `engine/apply.go` — + ветки `CloseVote` в `isLegal`/`Apply`; экспорт `State.Clone()` (Task 1, 6).
- `engine/view.go` — + `VoteView`, поле `SeatView.Vote`, заполнение в `View()` (Task 2).
- `engine/adjudication_test.go`, `engine/clone_test.go` — тесты Фазы A.
- `game/subscribe.go` — close-and-replace в `Subscribe` (Task 3).
- `game/submit.go` — doc-комментарий контракта L2-4 (Task 3).
- `game/vote.go` (Create) — системный `Session.CloseVote()` (Task 4).
- `game/session.go` — миграция хоста в `Leave` (Task 5).
- `game/projection.go` — rename `Snapshot(id)` → `SnapshotFor(id)` (Task 6).
- `game/persist.go` (Create) — `SessionState`, `Snapshot`, `Restore` (Task 6).

**Фаза B (новый пакет `server/`):**
- `server/clock.go` — `Clock`/`Timer`, `realClock`, `fakeClock` (Task 7).
- `server/code.go` — генерация кода комнаты (Task 8).
- `server/token.go` — `Token`, генерация токена (Task 9).
- `server/store.go` — `RoomSnapshot`, `RoomStore`, `MemStore` (Task 10).
- `server/protocol.go` — кодек `engine`↔JSON + конверты `ClientMsg`/`ServerMsg` (Task 11–12).
- `server/room.go` — `Room`: обёртка сессии, токены, write-through, таймеры голосования/grace (Task 13–14).
- `server/hub.go` — `Hub`: реестр комнат, генерация кода, GC (Task 15).
- `server/conn.go` — WS-соединение, два потока (Task 16).
- `server/http.go` — HTTP-хендлеры (create/join/WS) (Task 17).
- `cmd/shukh-server/main.go` (Create) — точка запуска (Task 18).
- Тесты: `server/*_test.go`, `server/integration_test.go` (Task 19).

**Фаза C:**
- `docs/architecture.md` — ревизия D-2, новый OQ, журнал (Task 20).

---

# ФАЗА A — Достройки Слоёв 0/1

### Task 1: engine `CloseVote` — форс-резолв открытого голосования R-8.6

**Files:**
- Modify: `engine/action.go`
- Modify: `engine/apply.go`
- Test: `engine/adjudication_test.go` (Modify)

**Interfaces:**
- Consumes: `engine.Adjudication`, `(State).resolveAdjudication(*[]Event)`, `VoteResolved` (всё есть в движке).
- Produces: `engine.CloseVote struct{}` (реализует `Action`); кейс `CloseVote` в `isLegal`; ветка `CloseVote` в `Apply`.

- [ ] **Step 1: Тест — частичная явка, ранний перевес, ноль голосов, no-op**

Дописать в `engine/adjudication_test.go` (переиспользует `playingState`/`assertResolved`; новых импортов не нужно):

```go
func TestCloseVotePartialTurnoutConfirmsTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// one lone «против ШУХа» ballot — far short of a table majority.
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: true})
	if err != nil {
		t.Fatalf("vote rejected: %v", err)
	}
	if ns.Adjudication == nil {
		t.Fatal("a single ballot must not auto-resolve a 3-seat vote")
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	if ns.Adjudication != nil {
		t.Fatal("CloseVote must clear the Adjudication")
	}
	assertResolved(t, events, false) // 1 of 3 support ⇒ not overturned
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate confirming the ШУХ on target 0, got %+v", ns.Pending)
	}
}

func TestCloseVoteEarlyMajorityFlipsToClaimant(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// two «против ШУХа» ballots — a table majority (2 of 3) reached before full turnout.
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote 0 rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: true})
	if err != nil {
		t.Fatalf("vote 2 rejected: %v", err)
	}
	if ns.Adjudication == nil {
		t.Fatal("with seat 1 not yet voted the 3-seat vote must still be open")
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	assertResolved(t, events, true) // 2 of 3 support ⇒ overturned
	if ns.Pending == nil || ns.Pending.Offender != 1 {
		t.Fatalf("expected Ш-8 payment gate on claimant 1, got %+v", ns.Pending)
	}
}

func TestCloseVoteZeroVotesConfirmsTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	if ns.Adjudication != nil {
		t.Fatal("CloseVote must clear the Adjudication")
	}
	assertResolved(t, events, false) // no support at all ⇒ confirmed on target
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate on target 0, got %+v", ns.Pending)
	}
}

func TestCloseVoteNoAdjudicationIsNoop(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, events, err := Apply(s, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote with no open vote must not error, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("no-op CloseVote must emit no events, got %+v", events)
	}
	if ns.Adjudication != nil || ns.Pending != nil {
		t.Fatalf("no-op CloseVote must not open any gate, got adj=%+v pending=%+v", ns.Adjudication, ns.Pending)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestCloseVote`
Expected: FAIL — `undefined: CloseVote`.

- [ ] **Step 3: Добавить действие в `engine/action.go`**

После объявления `type Vote struct {...}` (перед блоком методов `isAction`) добавить:

```go
// CloseVote force-resolves the open R-8.6 Adjudication NOW with whatever ballots
// have been cast (L2-1): a table majority backing the challenge (support*2 > n)
// moves the penalty onto the claimant as Ш-8, otherwise the ШУХ is confirmed on the
// target; a missing ballot is simply not counted as «против ШУХа». It is a system
// action — issued by the Layer-2 vote timer, never surfaced to a player — and a
// harmless no-op when no vote is open.
type CloseVote struct{}
```

В блок методов `isAction()` добавить:

```go
func (CloseVote) isAction() {}
```

- [ ] **Step 4: Добавить `isLegal`-кейс и ветку `Apply` в `engine/apply.go`**

В `isLegal`, внутри `switch act := a.(type)`, сразу после `case Vote:` и перед `default:` добавить:

```go
	case CloseVote:
		// A system resolution primitive (L2-1), never enumerated by LegalActions. It
		// is always permitted to reach Apply: with an open vote it resolves it with a
		// partial tally, with none it is a harmless no-op (the Apply branch below
		// guards on Adjudication). Design §8.1/§9 requires the closed case to be a
		// no-op, not an error — hence unconditional true rather than `Adjudication != nil`.
		return true
```

В `Apply`, внутри `switch act := a.(type)`, сразу после `case Vote:` и перед `default:` добавить:

```go
	case CloseVote:
		// Force-resolve with the ballots present (L2-1). Missing votes are simply not
		// tallied as «против ШУХа», so the R-8.6 formula (support*2 > n) is unchanged.
		// No open vote → harmless no-op: the clone is returned untouched, events nil.
		if ns.Adjudication != nil {
			ns.resolveAdjudication(&events)
		}
```

- [ ] **Step 5: Запустить — проходит**

Run: `go test ./engine/ -run TestCloseVote`
Expected: PASS.

- [ ] **Step 6: Полный прогон движка (регрессий нет)**

Run: `go test ./engine/`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add engine/action.go engine/apply.go engine/adjudication_test.go
git commit -m "feat(engine): CloseVote — форс-резолв R-8.6 частичной явкой (L2-1)"
```

---

### Task 2: engine `VoteView` — сводка разбора в проекции

**Files:**
- Modify: `engine/view.go`
- Test: `engine/adjudication_test.go` (Modify)

**Interfaces:**
- Consumes: `State.Adjudication`, `slices.Sort` (пакет `slices` уже импортирован в `view.go`).
- Produces: `engine.VoteView{Claimant, Target SeatID; Code ShukhCode; Voted []SeatID}`; поле `SeatView.Vote *VoteView`; заполнение в `View()`.

- [ ] **Step 1: Тест — заполнение, сортировка, nil без разбора, копия**

Дописать в `engine/adjudication_test.go`:

```go
func TestVoteViewPopulatedAndSorted(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// Cast two ballots out of ascending order (seat 2 then seat 0); a 3-seat vote
	// stays open at two ballots.
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: false})
	if err != nil {
		t.Fatalf("vote 2 rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote 0 rejected: %v", err)
	}
	v := View(ns, 1)
	if v.Vote == nil {
		t.Fatal("an open Adjudication must populate SeatView.Vote")
	}
	if v.Vote.Claimant != 1 || v.Vote.Target != 0 || v.Vote.Code != Sh6 {
		t.Fatalf("wrong VoteView dispute: %+v", v.Vote)
	}
	if len(v.Vote.Voted) != 2 || v.Vote.Voted[0] != 0 || v.Vote.Voted[1] != 2 {
		t.Fatalf("Voted must list who voted, ascending [0 2], got %v", v.Vote.Voted)
	}
}

func TestVoteViewNilWithoutAdjudication(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	if v := View(s, 0); v.Vote != nil {
		t.Fatalf("no Adjudication ⇒ SeatView.Vote must be nil, got %+v", v.Vote)
	}
}

func TestVoteViewVotedIsACopy(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote rejected: %v", err)
	}
	v := View(ns, 1)
	v.Vote.Voted[0] = 99 // mutate the returned slice
	if again := View(ns, 1); again.Vote.Voted[0] != 0 {
		t.Fatalf("View must return a fresh Voted slice; state leaked (%v)", again.Vote.Voted)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestVoteView`
Expected: FAIL — `undefined: VoteView / v.Vote`.

- [ ] **Step 3: Добавить тип `VoteView` и поле `SeatView.Vote` в `engine/view.go`**

После объявления `type OpponentView struct {...}` добавить:

```go
// VoteView is the public summary of an open R-8.6 table vote (§8.3). It exposes the
// dispute (Claimant/Target/Code) and Voted — the seats that have cast a ballot, in
// ascending order — but never how anyone voted: the ballot stays secret until the
// vote resolves (§8.4).
type VoteView struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
	Voted    []SeatID
}
```

Внутри `type SeatView struct`, сразу после поля `Finish []SeatID`, добавить:

```go
	// Vote summarizes the open R-8.6 adjudication for a reconnecting/observing seat
	// (§8.3): who raised what against whom and which seats have already cast a ballot
	// (the fact only — never how). nil when no vote is open.
	Vote *VoteView
```

- [ ] **Step 4: Заполнить `Vote` в `View()`**

В `View`, сразу перед `return v`, добавить:

```go
	if s.Adjudication != nil {
		voted := make([]SeatID, 0, len(s.Adjudication.Votes))
		for seat := range s.Adjudication.Votes {
			voted = append(voted, seat)
		}
		slices.Sort(voted) // ascending; expose only the fact of a ballot (§8.4)
		v.Vote = &VoteView{
			Claimant: s.Adjudication.Claimant,
			Target:   s.Adjudication.Target,
			Code:     s.Adjudication.Code,
			Voted:    voted,
		}
	}
```

- [ ] **Step 5: Запустить — проходит**

Run: `go test ./engine/ -run TestVoteView`
Expected: PASS.

- [ ] **Step 6: Полный прогон движка**

Run: `go test ./engine/`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add engine/view.go engine/adjudication_test.go
git commit -m "feat(engine): VoteView + SeatView.Vote — сводка разбора в проекции (§8.3)"
```

---

### Task 3: game — `Subscribe` close-and-replace + контракт `Submit` (L2-4)

**Files:**
- Modify: `game/subscribe.go`
- Modify: `game/submit.go` (только doc-комментарий)
- Test: `game/subscribe_test.go` (Modify)

**Interfaces:**
- Consumes: `Session.subs map[PlayerID]chan Update`, `project`, `roster`, `fanout` (все есть).
- Produces: `Subscribe` закрывает+удаляет существующего подписчика `id` перед установкой нового; doc-комментарий на `Submit`, фиксирующий L2-4. Сигнатуры без изменений.

- [ ] **Step 1: Тест — повторный `Subscribe` закрывает первый канал, второй жив**

Дописать в `game/subscribe_test.go` (пакет `game`, `engine` уже импортирован в файле):

```go
func TestSubscribeCloseAndReplace(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	ch1, _, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	<-ch1 // drain the initial snapshot

	ch2, cancel2, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("second subscribe: %v", err)
	}
	defer cancel2()

	// close-and-replace: the first channel must now be closed so its reader exits.
	// A non-blocking probe distinguishes "closed" (case, open==false) from the
	// pre-fix "still open but empty" (default) without hanging the test.
	select {
	case _, open := <-ch1:
		if open {
			t.Fatal("re-Subscribe must close (not feed) the previous channel")
		}
	default:
		t.Fatal("re-Subscribe must close the previous subscriber channel (still open)")
	}

	// the second channel is the live one: a fanout reaches it.
	<-ch2 // drain its initial snapshot
	s.mu.Lock()
	s.fanout([]engine.Event{engine.OneCardDeclared{Seat: 0}})
	s.mu.Unlock()
	select {
	case up, open := <-ch2:
		if !open {
			t.Fatal("the replacement channel must stay open")
		}
		if len(up.Events) != 1 {
			t.Fatalf("want 1 event on the live channel, got %d", len(up.Events))
		}
	default:
		t.Fatal("the replacement subscriber must receive the fanout")
	}
}
```

- [ ] **Step 2: Запустить — падает (старый канал не закрыт)**

Run: `go test ./game/ -run TestSubscribeCloseAndReplace`
Expected: FAIL — `re-Subscribe must close the previous subscriber channel (still open)`.

- [ ] **Step 3: Close-and-replace в `game/subscribe.go`**

Заменить тело `Subscribe` (вставка закрытия старого подписчика перед созданием нового канала):

```go
// Subscribe registers id for push Updates and immediately delivers a snapshot.
// The returned func() unsubscribes (closing the channel). Errors if id is not seated.
func (s *Session) Subscribe(id PlayerID) (<-chan Update, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return nil, nil, ErrUnknownPlayer
	}
	// Close-and-replace (L2 reconnect): if a subscriber for id already exists (a
	// still-live socket at reconnect), close and drop it first so its reader goroutine
	// receives a closed channel and exits — otherwise it leaks. The old cancel becomes
	// a no-op: its identity check (cur == oldch) no longer matches s.subs[id].
	if prev, ok := s.subs[id]; ok {
		delete(s.subs, id)
		close(prev)
	}
	ch := make(chan Update, subCapacity)
	s.subs[id] = ch
	ch <- s.project(id, s.roster(), nil) // initial snapshot fits (fresh buffer)
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if cur, ok := s.subs[id]; ok && cur == ch {
			delete(s.subs, id)
			close(ch)
		}
	}
	return ch, cancel, nil
}
```

- [ ] **Step 4: Doc-комментарий контракта L2-4 на `game/submit.go`**

Заменить doc-комментарий над `func (s *Session) Submit` (сигнатуру и тело не трогаем):

```go
// Submit applies a on behalf of id. It maps id to a seat, rejects impersonation
// (acting as another seat), then defers rule legality to engine.Apply. On success
// it advances authoritative state, updates the lifecycle, fans out to subscribers,
// and returns the events. On any rejection state is untouched and nothing is
// fanned out.
//
// L2-4 delivery contract: the returned events are an ACK echo for the caller only —
// the authoritative render path is the subscription (fanout has already delivered the
// same change to every seat, including this one). Layer 2 MUST render from the
// subscription and MUST NOT re-emit or re-render from this return value; treat it as
// «accepted», nothing more.
```

- [ ] **Step 5: Запустить — проходит; гонки**

Run: `go test ./game/ -run TestSubscribeCloseAndReplace`
Expected: PASS.
Run: `go test -race ./game/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add game/subscribe.go game/submit.go game/subscribe_test.go
git commit -m "feat(game): Subscribe close-and-replace (реконнект) + контракт Submit L2-4"
```

---

### Task 4: game `Session.CloseVote()` — системный вход для таймера

**Files:**
- Create: `game/vote.go`
- Test: `game/vote_test.go` (Create)

**Interfaces:**
- Consumes: `Session` (`mu`, `state`, `stage`), `engine.Apply`, `engine.CloseVote`, `engine.CheckInvariants`, `fanout`; хелпер `startedDuel` (объявлен в `game/submit_test.go`, тот же пакет).
- Produces: `func (s *Session) CloseVote() ([]engine.Event, error)`.

- [ ] **Step 1: Тест — резолв частичной явкой через фанаут; no-op без разбора**

Создать `game/vote_test.go`:

```go
package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func TestSessionCloseVoteResolvesPartialTally(t *testing.T) {
	s := startedDuel(t) // host = seat 0, "p2" = seat 1 (submit_test.go)
	ch, cancel, err := s.Subscribe("p2")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer cancel()
	<-ch // drain the initial snapshot

	// Host raises a subjective ШУХ against p2 → the vote opens.
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	opened := <-ch // fanout of VoteOpened
	if opened.View == nil || opened.View.Vote == nil {
		t.Fatal("with the vote open the projection must carry a VoteView")
	}

	// A single (partial) ballot — a 2-seat duel needs both to auto-resolve.
	if _, err := s.Submit("h", engine.Vote{Voter: 0, Support: false}); err != nil {
		t.Fatalf("host ballot rejected: %v", err)
	}
	<-ch // fanout of the still-open vote

	// The system timer forces resolution with only that partial tally.
	evs, err := s.CloseVote()
	if err != nil {
		t.Fatalf("CloseVote: %v", err)
	}
	sawResolved := false
	for _, e := range evs {
		if _, ok := e.(engine.VoteResolved); ok {
			sawResolved = true
		}
	}
	if !sawResolved {
		t.Fatal("CloseVote must emit VoteResolved")
	}

	// The resolution reaches the subscriber via fanout, and the vote is gone.
	resolved := <-ch
	if resolved.View == nil || resolved.View.Vote != nil {
		t.Fatalf("after CloseVote the projection must show no open vote, got %+v", resolved.View)
	}
	gotResolved := false
	for _, e := range resolved.Events {
		if _, ok := e.(engine.VoteResolved); ok {
			gotResolved = true
		}
	}
	if !gotResolved {
		t.Fatal("subscriber must receive the VoteResolved fanout")
	}
}

func TestSessionCloseVoteNoopWhenClosed(t *testing.T) {
	s := startedDuel(t)
	evs, err := s.CloseVote()
	if err != nil {
		t.Fatalf("CloseVote with no open vote must not error, got %v", err)
	}
	if evs != nil {
		t.Fatalf("CloseVote with no open vote must return nil events, got %+v", evs)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./game/ -run TestSessionCloseVote`
Expected: FAIL — `undefined: (*Session).CloseVote`.

- [ ] **Step 3: Написать `game/vote.go`**

```go
package game

import "github.com/oustrix/shukh/engine"

// CloseVote force-resolves the open R-8.6 vote with the ballots cast so far (L2-1).
// It is the system entrypoint for the Layer-2 vote timer — it bypasses authorize (no
// player owns it). With no vote open it is a (nil, nil) no-op (the timer may fire
// after a full-turnout auto-resolve already cleared the vote). Otherwise it applies
// engine.CloseVote (resolving on a partial tally — a missing ballot is not counted as
// «против ШУХа»), advances the lifecycle, and fans the resolution out to every
// subscriber. Mirrors Submit's apply→invariant→stage→fanout discipline.
func (s *Session) CloseVote() ([]engine.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Adjudication == nil {
		return nil, nil // no open vote → nothing to resolve
	}
	ns, events, err := engine.Apply(s.state, engine.CloseVote{})
	if err != nil {
		return nil, err
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
	s.fanout(events)
	return events, nil
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/ -run TestSessionCloseVote`
Expected: PASS.

- [ ] **Step 5: Полный прогон пакета + гонки**

Run: `go test -race ./game/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add game/vote.go game/vote_test.go
git commit -m "feat(game): Session.CloseVote() — системный резолв R-8.6 по таймеру (L2-1)"
```

---

### Task 5: game — миграция хоста в `Leave`

**Files:**
- Modify: `game/session.go`
- Test: `game/session_test.go` (Modify)

**Interfaces:**
- Consumes: `Session` (`mu`, `stage`, `host`, `order`, `names`).
- Produces: `Leave` мигрирует хост на `s.order[0]` после удаления, если уходит хост и ≥1 игрок остаётся; при опустевшей комнате — no-op (L2-3). Сигнатура без изменений.

- [ ] **Step 1: Тест — хост уходит из лобби, роль переезжает; пустая комната**

Дописать в `game/session_test.go` (пакет `game`; `cfg36` объявлен в этом файле):

```go
func TestLeaveMigratesHost(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.Join("p3", "Cara"); err != nil {
		t.Fatal(err)
	}
	s.Leave("h") // the host leaves the lobby

	// The host role migrates to the new order[0] (p2); p3 is still a non-host.
	if err := s.Start("p3", 1); err != ErrNotHost {
		t.Fatalf("non-host p3 start: want ErrNotHost, got %v", err)
	}
	if err := s.Start("p2", 42); err != nil {
		t.Fatalf("migrated host p2 must be able to Start, got %v", err)
	}
	if s.Stage() != Playing {
		t.Fatalf("after Start stage must be Playing, got %v", s.Stage())
	}
}

func TestLeaveHostEmptiesRoom(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	s.Leave("h") // sole player leaves → host migration is a no-op
	if len(s.order) != 0 {
		t.Fatalf("room must be empty after the sole player leaves, got order %v", s.order)
	}
}
```

- [ ] **Step 2: Запустить — падает (хост не мигрирует)**

Run: `go test ./game/ -run TestLeave`
Expected: FAIL — `migrated host p2 must be able to Start, got game: only the host may do this`.

- [ ] **Step 3: Миграция хоста в `Leave` (`game/session.go`)**

Заменить `Leave` целиком:

```go
// Leave removes a player from the lobby (mid-game leave is a Layer-2 disconnect
// concern, out of scope here). No-op if the game has started or the player is absent.
// If the leaving player is the host and at least one player remains, the host role
// migrates to the new order[0] (L2-3); if the room becomes empty the host is left
// dangling and Layer 2 GCs the room — nothing to migrate to.
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
	if id == s.host && len(s.order) > 0 {
		s.host = s.order[0] // migrate the host role to the next seat (L2-3)
	}
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./game/ -run TestLeave`
Expected: PASS.

- [ ] **Step 5: Полный прогон пакета**

Run: `go test ./game/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "feat(game): миграция хоста при уходе из лобби (L2-3)"
```

---

### Task 6: `engine.State.Clone()` + game `Snapshot`/`Restore`

> **Необходимая правка имени (коллизия методов).** В `game/projection.go` уже есть
> `func (s *Session) Snapshot(id PlayerID) (Update, error)` (per-seat проекция). Заблокированный
> контракт требует `func (s *Session) Snapshot() SessionState` — Go запрещает два метода с одним
> именем на `*Session`. Поэтому per-seat аксессор **переименовывается** в `SnapshotFor(id)` (Step 5),
> освобождая имя `Snapshot` для персистентного снимка. Фаза B ссылается на durable `Snapshot()` и
> per-seat `SnapshotFor(id)` именно в этих именах.

**Files:**
- Modify: `engine/apply.go` (экспорт `Clone`)
- Test: `engine/clone_test.go` (Create)
- Modify: `game/projection.go` (rename `Snapshot(id)` → `SnapshotFor(id)`)
- Modify: `game/projection_test.go`, `game/submit_test.go`, `game/integration_test.go` (обновить вызовы)
- Create: `game/persist.go`
- Test: `game/persist_test.go` (Create)

**Interfaces:**
- Consumes: unexported `(State).clone() State`; `Session` поля (`cfg`, `host`, `stage`, `order`, `names`, `state`, `subs`); `maps.Clone`; `engine.State.Clone`; `startedDuel` (submit_test.go).
- Produces: `func (s State) Clone() State`; `game.SessionState{Config Config; Host PlayerID; Stage Lifecycle; Order []PlayerID; Names map[PlayerID]string; Game engine.State}`; `func (s *Session) Snapshot() SessionState`; `func Restore(st SessionState) *Session`; renamed `func (s *Session) SnapshotFor(id PlayerID) (Update, error)`.

- [ ] **Step 1: Тест — `engine.State.Clone` глубокий**

Создать `engine/clone_test.go`:

```go
package engine

import "testing"

func TestStateCloneIsDeep(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 2, 1: 2, 2: 2})
	s.Adjudication = &Adjudication{Claimant: 0, Target: 1, Code: Sh6, Votes: map[SeatID]bool{0: true}}
	c := s.Clone()
	// Mutate the clone's maps and pointer target in place; the original must not move.
	c.Live[0] = false
	c.Hands[0] = nil
	c.Adjudication.Votes[1] = false
	c.Adjudication.Claimant = 9
	if s.Live[0] != true {
		t.Fatal("Clone aliased the Live map")
	}
	if s.Hands[0] == nil {
		t.Fatal("Clone aliased the Hands map")
	}
	if _, ok := s.Adjudication.Votes[1]; ok {
		t.Fatal("Clone aliased Adjudication.Votes")
	}
	if s.Adjudication.Claimant != 0 {
		t.Fatal("Clone aliased the Adjudication pointer")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./engine/ -run TestStateCloneIsDeep`
Expected: FAIL — `s.Clone undefined (type State has no field or method Clone)`.

- [ ] **Step 3: Экспортировать `Clone` в `engine/apply.go`**

Сразу после unexported `func (s State) clone() State {...}` добавить:

```go
// Clone returns a deep copy of s suitable for handing across a layer boundary
// (Layer 1 Snapshot/Restore): every mutated map and slice is fresh and no pointer
// target (Pending/Unsettled/Adjudication) is aliased, so the copy shares no storage
// with the original. It is the exported form of the internal copy-on-write clone.
func (s State) Clone() State { return s.clone() }
```

- [ ] **Step 4: Запустить — проходит; commit движка**

Run: `go test ./engine/ -run TestStateCloneIsDeep`
Expected: PASS.
Run: `go test ./engine/`
Expected: `ok`.

```bash
git add engine/apply.go engine/clone_test.go
git commit -m "feat(engine): экспорт State.Clone() — шов для Snapshot/Restore Слоя 1"
```

- [ ] **Step 5: Переименовать per-seat `Snapshot(id)` → `SnapshotFor(id)`**

В `game/projection.go` заменить метод `Snapshot`:

```go
// SnapshotFor returns the current per-seat projection for id (no events). Errors if
// id is not seated. Renamed from Snapshot so that name is free for the durable
// SessionState snapshot (persist.go); Layer 2 uses this for slow-consumer recovery.
func (s *Session) SnapshotFor(id PlayerID) (Update, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seatOf(id); !ok {
		return Update{}, ErrUnknownPlayer
	}
	return s.project(id, s.roster(), nil), nil
}
```

Обновить все вызовы per-seat метода (единственные call-site'ы во всём пакете):

- `game/projection_test.go`: заменить вхождения `s.Snapshot(` на `s.SnapshotFor(` —
  `s.SnapshotFor("h")` (в `TestSnapshotLobbyHasRosterNoView` и `TestSnapshotPlayingHidesOpponents`)
  и `s.SnapshotFor("ghost")` (в `TestSnapshotUnknownPlayer`).
- `game/submit_test.go`: `s.SnapshotFor("h")` и `s.SnapshotFor(idler)` (в `TestSubmitRejectsOffTurnImpersonation`).
- `game/integration_test.go`: `s.SnapshotFor(...)` во всех вхождениях (драйвер сквозняка + финальный снапшот).

> **Как найти все call-site'ы:** `grep -rn 'Snapshot(' game/` — заменить только вызовы с
> аргументом-`PlayerID`; вызовов без аргумента (durable `Snapshot()`) до Step 8 ещё нет.

Run (чистое переименование, поведение не меняется):
`go test ./game/`
Expected: `ok`.

- [ ] **Step 6: Тест — round-trip `Snapshot`/`Restore` с доказательством глубокой копии**

Создать `game/persist_test.go`:

```go
package game

import (
	"reflect"
	"testing"
)

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	s := startedDuel(t) // host = seat 0, "p2" = seat 1 (submit_test.go)

	// Deep-copy proof: a snapshot must not observe later in-place mutation of the
	// live session's maps.
	snap := s.Snapshot()
	origLive0 := snap.Game.Live[0]
	origName := snap.Names["h"]
	s.mu.Lock()
	s.state.Live[0] = !s.state.Live[0]
	s.names["h"] = "Mutated"
	s.mu.Unlock()
	if snap.Game.Live[0] != origLive0 {
		t.Fatal("Snapshot must deep-copy engine.State (Live map aliased)")
	}
	if snap.Names["h"] != origName {
		t.Fatal("Snapshot must deep-copy the names map")
	}

	// Round-trip: Restore rebuilds a session whose own Snapshot deep-equals the input.
	want := s.Snapshot()
	r := Restore(want)
	got := r.Snapshot()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Restore round-trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}

	// The restored session is fully live: fresh empty subs accept a new subscriber,
	// and it can still project per-seat.
	ch, cancel, err := r.Subscribe("h")
	if err != nil {
		t.Fatalf("restored session Subscribe: %v", err)
	}
	defer cancel()
	<-ch // initial snapshot delivered
	if _, err := r.SnapshotFor("h"); err != nil {
		t.Fatalf("restored session SnapshotFor: %v", err)
	}
}
```

- [ ] **Step 7: Запустить — падает компиляцией**

Run: `go test ./game/ -run TestSnapshotRestoreRoundTrip`
Expected: FAIL — `s.Snapshot undefined` / `undefined: Restore` (нет персистентного снимка и `Restore`).

- [ ] **Step 8: Написать `game/persist.go`**

```go
package game

import (
	"maps"

	"github.com/oustrix/shukh/engine"
)

// SessionState is the durable snapshot of a Session (Layer-2 RoomStore seam, L2-5):
// pure data with no live machinery (no subs, no mutex). It round-trips through
// Snapshot/Restore so a room can be persisted and rebuilt.
type SessionState struct {
	Config Config
	Host   PlayerID
	Stage  Lifecycle
	Order  []PlayerID
	Names  map[PlayerID]string
	Game   engine.State
}

// Snapshot returns a deep copy of the session's durable state (L2-5). Every map and
// slice is cloned — including engine.State via State.Clone — so the returned value
// shares no aliased storage with the live session and can be persisted or held while
// play continues.
func (s *Session) Snapshot() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionState{
		Config: s.cfg,
		Host:   s.host,
		Stage:  s.stage,
		Order:  append([]PlayerID(nil), s.order...),
		Names:  maps.Clone(s.names),
		Game:   s.state.Clone(),
	}
}

// Restore rebuilds a live Session from a durable snapshot (L2-5). Durable data is
// deep-copied in; the ephemeral machinery (subscriptions) is recreated empty, so the
// restored session starts with no subscribers and is otherwise identical.
func Restore(st SessionState) *Session {
	return &Session{
		cfg:   st.Config,
		host:  st.Host,
		stage: st.Stage,
		order: append([]PlayerID(nil), st.Order...),
		names: maps.Clone(st.Names),
		state: st.Game.Clone(),
		subs:  map[PlayerID]chan Update{},
	}
}
```

- [ ] **Step 9: Запустить — проходит; полный прогон + гонки**

Run: `go test ./game/ -run TestSnapshotRestoreRoundTrip`
Expected: PASS.
Run: `go test -race ./...`
Expected: `ok  engine`, `ok  game`, `ok  shuffle`.

- [ ] **Step 10: Commit**

```bash
git add game/projection.go game/projection_test.go game/submit_test.go game/integration_test.go game/persist.go game/persist_test.go
git commit -m "feat(game): SessionState + Snapshot/Restore (шов RoomStore L2-5); rename per-seat SnapshotFor"
```

---

# ФАЗА B — Пакет `server/` (Слой 2)

### Task 7: `server/clock.go` — шов времени (`Clock`/`Timer`) + фейковые часы

**Files:**
- Create: `server/clock.go`
- Test: `server/clock_test.go` (Create)

**Interfaces:**
- Produces: `type Clock interface { Now() time.Time; AfterFunc(d time.Duration, f func()) Timer }`; `type Timer interface { Stop() bool }`; `func NewRealClock() Clock`; неэкспортируемые `realClock`, `fakeClock` (`newFakeClock(start time.Time) *fakeClock`, `(*fakeClock).Advance(d time.Duration)`, `(*fakeClock).Now()`, `(*fakeClock).AfterFunc(...)`).

- [ ] **Step 1: Тест — `Advance` за дедлайн стреляет; `Stop` предотвращает**

Создать `server/clock_test.go`:

```go
package server

import (
	"testing"
	"time"
)

func TestFakeClockFiresAfterDeadline(t *testing.T) {
	c := newFakeClock(time.Unix(0, 0))
	fired := false
	c.AfterFunc(10*time.Second, func() { fired = true })
	c.Advance(9 * time.Second)
	if fired {
		t.Fatal("timer fired before its deadline")
	}
	c.Advance(2 * time.Second) // now past 10s
	if !fired {
		t.Fatal("timer did not fire after Advance past the deadline")
	}
}

func TestFakeClockStopPreventsFiring(t *testing.T) {
	c := newFakeClock(time.Unix(0, 0))
	fired := false
	tm := c.AfterFunc(10*time.Second, func() { fired = true })
	if !tm.Stop() {
		t.Fatal("Stop of a pending timer must report true")
	}
	c.Advance(1 * time.Hour)
	if fired {
		t.Fatal("a stopped timer must not fire")
	}
	if tm.Stop() {
		t.Fatal("second Stop must report false")
	}
}

func TestFakeClockNowIsDeterministic(t *testing.T) {
	c := newFakeClock(time.Unix(100, 0))
	c.Advance(5 * time.Second)
	if got := c.Now().Unix(); got != 105 {
		t.Fatalf("Now = %d, want 105", got)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/`
Expected: FAIL — `no required module provides package .../server` / `undefined: newFakeClock`.

- [ ] **Step 3: Написать `server/clock.go`**

```go
// Package server is Layer 2 (D-6): the network adapter over game.Session. It maps
// connection→PlayerID and room-code→*game.Session, drives the synchronous session
// API, and relays Updates to browsers over WebSocket. It holds no game rules — the
// rules live in engine (Layer 0) and the authoritative procedures in game (Layer 1).
package server

import (
	"sync"
	"time"
)

// Timer is a pending one-shot callback. Stop cancels it, reporting whether it was
// still pending (true) or had already fired / been stopped (false).
type Timer interface {
	Stop() bool
}

// Clock is the seam over time (L2-9): real in production, fake in tests. Vote,
// grace, and GC timers go through it so tests advance time deterministically
// instead of sleeping.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// NewRealClock returns a Clock backed by the standard library.
func NewRealClock() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f) // *time.Timer satisfies Timer (Stop() bool)
}

// fakeClock is a deterministic test double: time only moves on Advance, which fires
// every timer whose deadline it passes.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{now: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, deadline: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock forward and fires every due timer, in insertion order,
// with the lock released so a callback may arm further timers.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.fired && !t.stopped && !t.deadline.After(now) {
			t.fired = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

type fakeTimer struct {
	clock    *fakeClock
	deadline time.Time
	f        func()
	fired    bool
	stopped  bool
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.fired || t.stopped {
		return false
	}
	t.stopped = true
	return true
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/clock.go server/clock_test.go
git commit -m "feat(server): Clock/Timer шов + фейковые часы (L2-9)"
```

---

### Task 8: `server/code.go` — генерация кодов комнат

**Files:**
- Create: `server/code.go`
- Test: `server/code_test.go` (Create)

**Interfaces:**
- Produces: `func newCode(rng func() byte) string` (6 символов над `23456789ABCDEFGHJKLMNPQRSTUVWXYZ`); неэкспортируемый `cryptoBytes() func() byte` (крипто-источник для прода).

- [ ] **Step 1: Тест — длина 6, алфавит, детерминизм при фиксированном источнике**

Создать `server/code_test.go`:

```go
package server

import (
	"strings"
	"testing"
)

func TestNewCodeShapeAndAlphabet(t *testing.T) {
	code := newCode(cryptoBytes())
	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}
	for _, r := range code {
		if !strings.ContainsRune(codeAlphabet, r) {
			t.Fatalf("code %q contains out-of-alphabet rune %q", code, r)
		}
	}
	// no visually ambiguous symbols leak in
	for _, bad := range []rune{'O', '0', 'I', '1', 'L'} {
		if strings.ContainsRune(codeAlphabet, bad) {
			t.Fatalf("alphabet must not contain ambiguous %q", bad)
		}
	}
}

func TestNewCodeDeterministicForFixedSource(t *testing.T) {
	src := func() func() byte {
		buf := []byte{0, 1, 2, 3, 30, 31, 40, 41}
		i := 0
		return func() byte { b := buf[i%len(buf)]; i++; return b }
	}
	a := newCode(src())
	b := newCode(src())
	if a != b {
		t.Fatalf("same byte source must yield same code: %q vs %q", a, b)
	}
	// index masks to 5 bits: 0->'2', 1->'3', 2->'4', 3->'5', 30->'Y', 31->'Z'
	if a != "2345YZ" {
		t.Fatalf("deterministic code = %q, want 2345YZ", a)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run TestNewCode`
Expected: FAIL — `undefined: newCode / codeAlphabet / cryptoBytes`.

- [ ] **Step 3: Написать `server/code.go`**

```go
package server

import crand "crypto/rand"

// codeAlphabet is 32 unambiguous symbols — no O/0, I/1/L. 32 = 2^5, so masking a
// byte to its low 5 bits selects a symbol without modulo bias.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const codeLen = 6

// newCode builds a 6-char room code (§5.1), drawing each symbol from the
// unambiguous alphabet via 5 bits of rng. Injecting rng makes it deterministic in
// tests; production passes cryptoBytes.
func newCode(rng func() byte) string {
	var b [codeLen]byte
	for i := range b {
		b[i] = codeAlphabet[rng()&0x1f]
	}
	return string(b[:])
}

// cryptoBytes returns a byte source backed by crypto/rand. A crypto/rand failure is
// unrecoverable, so it panics rather than return a weak code.
func cryptoBytes() func() byte {
	return func() byte {
		var b [1]byte
		if _, err := crand.Read(b[:]); err != nil {
			panic("server: crypto/rand failed: " + err.Error())
		}
		return b[0]
	}
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/ -run TestNewCode`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/code.go server/code_test.go
git commit -m "feat(server): генерация 6-символьного кода комнаты (§5.1)"
```

---

### Task 9: `server/token.go` — реконнект-токены

**Files:**
- Create: `server/token.go`
- Test: `server/token_test.go` (Create)

**Interfaces:**
- Produces: `type Token string`; `func newToken() (Token, error)` — 16 крипто-байт в base64url (без паддинга).

- [ ] **Step 1: Тест — токены различны, декодируются в 16 байт**

Создать `server/token_test.go`:

```go
package server

import (
	"encoding/base64"
	"testing"
)

func TestNewTokenDistinct(t *testing.T) {
	seen := map[Token]bool{}
	for i := 0; i < 100; i++ {
		tok, err := newToken()
		if err != nil {
			t.Fatalf("newToken: %v", err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token minted: %q", tok)
		}
		seen[tok] = true
	}
}

func TestNewTokenDecodesTo16Bytes(t *testing.T) {
	tok, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(string(tok))
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != 16 {
		t.Fatalf("decoded token length = %d, want 16 (~128 bits, L2-7)", len(raw))
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run TestNewToken`
Expected: FAIL — `undefined: Token / newToken`.

- [ ] **Step 3: Написать `server/token.go`**

```go
package server

import (
	crand "crypto/rand"
	"encoding/base64"
)

// Token is an opaque per-seat reconnect secret (~128 bits, L2-7). It lives in an
// HttpOnly cookie; the server maps it to a PlayerID. Knowing the token grants the
// seat — like a share link to a document. No JWT, no signature: state is server-side.
type Token string

// newToken mints 16 crypto-random bytes as base64url (no padding, URL-safe).
func newToken() (Token, error) {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", err
	}
	return Token(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/ -run TestNewToken`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/token.go server/token_test.go
git commit -m "feat(server): реконнект-токены (16 крипто-байт base64url, L2-7)"
```

---

### Task 10: `server/store.go` — шов хранилища (`RoomStore` + `MemStore`)

**Files:**
- Create: `server/store.go`
- Test: `server/store_test.go` (Create)

**Interfaces:**
- Consumes: `game.PlayerID`, `game.SessionState`, `(*game.Session).Snapshot() game.SessionState` (Фаза A, durable-снимок).
- Produces: `type RoomSnapshot struct { Code string; Tokens map[Token]game.PlayerID; Session game.SessionState }`; `type RoomStore interface { Save(RoomSnapshot) error; Load(code string) (RoomSnapshot, bool, error); Delete(code string) error; List() ([]string, error) }`; `type MemStore struct{...}`; `func NewMemStore() *MemStore`.

- [ ] **Step 1: Тест — round-trip, промах ok=false, Delete, List, глубокая копия Tokens**

Создать `server/store_test.go`:

```go
package server

import (
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

func sampleSnapshot(code string) RoomSnapshot {
	cfg := game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
	st := game.NewSession(cfg, "host", "Host").Snapshot() // durable SessionState (Фаза A)
	return RoomSnapshot{
		Code:    code,
		Tokens:  map[Token]game.PlayerID{"tok-h": "host"},
		Session: st,
	}
}

func TestMemStoreRoundTrip(t *testing.T) {
	m := NewMemStore()
	snap := sampleSnapshot("ROOM01")
	if err := m.Save(snap); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := m.Load("ROOM01")
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if got.Code != "ROOM01" || got.Tokens["tok-h"] != "host" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestMemStoreSaveDeepCopiesTokens(t *testing.T) {
	m := NewMemStore()
	snap := sampleSnapshot("ROOM01")
	_ = m.Save(snap)
	// mutating the live snapshot's Tokens must not corrupt the store.
	snap.Tokens["tok-h"] = "hijacked"
	got, _, _ := m.Load("ROOM01")
	if got.Tokens["tok-h"] != "host" {
		t.Fatalf("stored token was aliased and mutated: %q", got.Tokens["tok-h"])
	}
}

func TestMemStoreLoadMiss(t *testing.T) {
	m := NewMemStore()
	if _, ok, err := m.Load("nope"); ok || err != nil {
		t.Fatalf("miss must be ok=false,nil; got ok=%v err=%v", ok, err)
	}
}

func TestMemStoreDeleteAndList(t *testing.T) {
	m := NewMemStore()
	_ = m.Save(sampleSnapshot("A"))
	_ = m.Save(sampleSnapshot("B"))
	if err := m.Delete("A"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := m.Load("A"); ok {
		t.Fatal("deleted room still present")
	}
	list, _ := m.List()
	if len(list) != 1 || list[0] != "B" {
		t.Fatalf("List after delete = %v, want [B]", list)
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run TestMemStore`
Expected: FAIL — `undefined: RoomSnapshot / NewMemStore`.

- [ ] **Step 3: Написать `server/store.go`**

```go
package server

import (
	"sync"

	"github.com/oustrix/shukh/game"
)

// RoomSnapshot is a room's durable state — plain, serializable data (§4). The
// ephemeral machinery (sockets, subscription channels, timers) is rebuilt on
// reconnect/restart and is not part of the snapshot.
type RoomSnapshot struct {
	Code    string
	Tokens  map[Token]game.PlayerID
	Session game.SessionState // {Config, Host, Stage, Order, Names, Game engine.State}
}

// RoomStore is the storage seam (L2-5): the Hub depends only on this. MemStore is
// the MVP default; a RedisStore / SQLStore would implement the same interface with
// nothing else changed.
type RoomStore interface {
	Save(RoomSnapshot) error
	Load(code string) (RoomSnapshot, bool, error)
	Delete(code string) error
	List() ([]string, error)
}

// MemStore is an in-memory RoomStore under a mutex.
type MemStore struct {
	mu    sync.Mutex
	rooms map[string]RoomSnapshot
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{rooms: map[string]RoomSnapshot{}} }

// Save write-throughs a deep copy. Session.Game is already deep (game.Snapshot
// deep-copies, §4); we additionally copy the Tokens map so later mutation of the
// live snapshot cannot corrupt the store.
func (m *MemStore) Save(snap RoomSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rooms[snap.Code] = copySnapshot(snap)
	return nil
}

func (m *MemStore) Load(code string) (RoomSnapshot, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap, ok := m.rooms[code]
	if !ok {
		return RoomSnapshot{}, false, nil
	}
	return copySnapshot(snap), true, nil
}

func (m *MemStore) Delete(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, code)
	return nil
}

func (m *MemStore) List() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.rooms))
	for c := range m.rooms {
		out = append(out, c)
	}
	return out, nil
}

func copySnapshot(snap RoomSnapshot) RoomSnapshot {
	cp := snap
	cp.Tokens = make(map[Token]game.PlayerID, len(snap.Tokens))
	for k, v := range snap.Tokens {
		cp.Tokens[k] = v
	}
	return cp
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/ -run TestMemStore`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/store.go server/store_test.go
git commit -m "feat(server): RoomStore + MemStore (write-through, глубокая копия, L2-5)"
```

---
### Task 11: `server/protocol.go` (кодек ч.1) — карты + действия

**Files:**
- Create: `server/protocol.go`
- Test: `server/protocol_action_test.go` (Create)

**Interfaces:**
- Consumes: `engine.Card`, `engine.Suit`, `engine.Rank`, `engine.SeatID`, `engine.ShukhCode`, все `engine.*Action` типы.
- Produces: `func encodeCard(engine.Card) any`; `func decodeAction(raw json.RawMessage) (engine.Action, error)`; `func encodeAction(engine.Action) any`; `func withActor(a engine.Action, seat engine.SeatID) engine.Action`.

> **Design note.** Для `vote`/`claimSubjective`/`declareOneCard`/`takeShukhCards` клиент не
> должен называть чужое сиденье. `decodeAction` парсит поля как есть; `withActor` (зовётся из
> conn.go) штампует аутентифицированное сиденье в self-referential поля, а `Session.authorize`
> (Слой 1) проверяет личность.

- [ ] **Step 1: Тест — round-trip карты; декод каждого варианта; `againstShukh`⇒`Support:true`; неизвестный тип → ошибка**

Создать `server/protocol_action_test.go`:

```go
package server

import (
	"encoding/json"
	"testing"

	"github.com/oustrix/shukh/engine"
)

func decodeActionJSON(t *testing.T, s string) engine.Action {
	t.Helper()
	a, err := decodeAction(json.RawMessage(s))
	if err != nil {
		t.Fatalf("decodeAction(%s): %v", s, err)
	}
	return a
}

func TestCardRoundTrip(t *testing.T) {
	c := engine.Card{Suit: engine.Hearts, Rank: engine.Queen}
	data, err := json.Marshal(encodeCard(c))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"suit":"♥","rank":12}` {
		t.Fatalf("card JSON = %s", data)
	}
	back, err := decodeAction(json.RawMessage(`{"type":"playCard","card":` + string(data) + `}`))
	if err != nil {
		t.Fatalf("decode playCard: %v", err)
	}
	if pc, ok := back.(engine.PlayCard); !ok || pc.Card != c {
		t.Fatalf("card did not round-trip: %+v", back)
	}
}

func TestDecodeEachActionVariant(t *testing.T) {
	cases := map[string]engine.Action{
		`{"type":"takeBottomAndPass"}`:                                engine.TakeBottomAndPass{},
		`{"type":"podkladkaWest"}`:                                    engine.PodkladkaWest{},
		`{"type":"discardWest"}`:                                      engine.DiscardWest{},
		`{"type":"claimShukh","target":2,"code":2}`:                   engine.ClaimShukh{Target: 2, Code: engine.Sh2},
		`{"type":"giveShukhCard","card":{"suit":"♠","rank":7}}`:       engine.GiveShukhCard{Card: engine.Card{Suit: engine.Spades, Rank: 7}},
		`{"type":"takeShukhCards","seat":1}`:                          engine.TakeShukhCards{Seat: 1},
		`{"type":"declareOneCard","seat":1}`:                          engine.DeclareOneCard{Seat: 1},
		`{"type":"askCount","target":1}`:                              engine.AskCount{Target: 1},
		`{"type":"askAboutWest","target":1}`:                          engine.AskAboutWest{Target: 1},
		`{"type":"claimSubjective","claimant":0,"target":1,"code":6}`: engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6},
	}
	for raw, want := range cases {
		got := decodeActionJSON(t, raw)
		if got != want {
			t.Fatalf("decode %s = %#v, want %#v", raw, got, want)
		}
	}
}

func TestDecodeVoteMapsSupport(t *testing.T) {
	against := decodeActionJSON(t, `{"type":"vote","vote":"againstShukh"}`)
	if v, ok := against.(engine.Vote); !ok || !v.Support {
		t.Fatalf("againstShukh must map to Support:true, got %#v", against)
	}
	forShukh := decodeActionJSON(t, `{"type":"vote","vote":"forShukh"}`)
	if v, ok := forShukh.(engine.Vote); !ok || v.Support {
		t.Fatalf("forShukh must map to Support:false, got %#v", forShukh)
	}
}

func TestDecodeUnknownType(t *testing.T) {
	if _, err := decodeAction(json.RawMessage(`{"type":"noSuchAction"}`)); err == nil {
		t.Fatal("unknown action type must error")
	}
}

func TestWithActorStampsSelfFields(t *testing.T) {
	if a := withActor(engine.Vote{Support: true}, 3); a.(engine.Vote).Voter != 3 {
		t.Fatal("withActor must stamp Vote.Voter")
	}
	if a := withActor(engine.ClaimSubjective{Target: 1, Code: engine.Sh6}, 2); a.(engine.ClaimSubjective).Claimant != 2 {
		t.Fatal("withActor must stamp ClaimSubjective.Claimant")
	}
	if a := withActor(engine.DeclareOneCard{}, 4); a.(engine.DeclareOneCard).Seat != 4 {
		t.Fatal("withActor must stamp DeclareOneCard.Seat")
	}
	if a := withActor(engine.PlayCard{}, 5); a != (engine.PlayCard{}) {
		t.Fatal("withActor must leave non-self actions unchanged")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run 'TestCard|TestDecode|TestWithActor'`
Expected: FAIL — `undefined: encodeCard / decodeAction / withActor`.

- [ ] **Step 3: Написать `server/protocol.go` (карты + действия)**

```go
package server

import (
	"encoding/json"
	"fmt"

	"github.com/oustrix/shukh/engine"
)

// cardDTO is the wire form of a card: suit glyph + numeric rank, mirroring the TS
// Card in web/src/contract/types.ts (W-3).
type cardDTO struct {
	Suit string `json:"suit"`
	Rank int    `json:"rank"`
}

func encodeCard(c engine.Card) any {
	return cardDTO{Suit: suitGlyph(c.Suit), Rank: int(c.Rank)}
}

// suitGlyph / glyphToSuit are the explicit two-way map between engine suits and the
// TS glyphs, independent of any engine String() formatting.
func suitGlyph(s engine.Suit) string {
	switch s {
	case engine.Spades:
		return "♠"
	case engine.Hearts:
		return "♥"
	case engine.Diamonds:
		return "♦"
	case engine.Clubs:
		return "♣"
	}
	return "?"
}

func (d cardDTO) toCard() (engine.Card, error) {
	su, err := glyphToSuit(d.Suit)
	if err != nil {
		return engine.Card{}, err
	}
	return engine.Card{Suit: su, Rank: engine.Rank(d.Rank)}, nil
}

func glyphToSuit(g string) (engine.Suit, error) {
	switch g {
	case "♠":
		return engine.Spades, nil
	case "♥":
		return engine.Hearts, nil
	case "♦":
		return engine.Diamonds, nil
	case "♣":
		return engine.Clubs, nil
	}
	return 0, fmt.Errorf("server: unknown suit glyph %q", g)
}

// decodeAction parses the discriminated-union client action JSON into an
// engine.Action. Self-referential seat fields (vote.voter, claimSubjective.claimant)
// are left zero here — conn.go stamps them via withActor.
func decodeAction(raw json.RawMessage) (engine.Action, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "playCard":
		var p struct {
			Card cardDTO `json:"card"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		c, err := p.Card.toCard()
		if err != nil {
			return nil, err
		}
		return engine.PlayCard{Card: c}, nil
	case "takeBottomAndPass":
		return engine.TakeBottomAndPass{}, nil
	case "podkladkaWest":
		return engine.PodkladkaWest{}, nil
	case "discardWest":
		return engine.DiscardWest{}, nil
	case "claimShukh":
		var p struct {
			Target engine.SeatID    `json:"target"`
			Code   engine.ShukhCode `json:"code"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.ClaimShukh{Target: p.Target, Code: p.Code}, nil
	case "giveShukhCard":
		var p struct {
			Card cardDTO `json:"card"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		c, err := p.Card.toCard()
		if err != nil {
			return nil, err
		}
		return engine.GiveShukhCard{Card: c}, nil
	case "takeShukhCards":
		var p struct {
			Seat engine.SeatID `json:"seat"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.TakeShukhCards{Seat: p.Seat}, nil
	case "declareOneCard":
		var p struct {
			Seat engine.SeatID `json:"seat"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.DeclareOneCard{Seat: p.Seat}, nil
	case "askCount":
		var p struct {
			Target engine.SeatID `json:"target"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.AskCount{Target: p.Target}, nil
	case "askAboutWest":
		var p struct {
			Target engine.SeatID `json:"target"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.AskAboutWest{Target: p.Target}, nil
	case "claimSubjective":
		var p struct {
			Claimant engine.SeatID    `json:"claimant"`
			Target   engine.SeatID    `json:"target"`
			Code     engine.ShukhCode `json:"code"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.ClaimSubjective{Claimant: p.Claimant, Target: p.Target, Code: p.Code}, nil
	case "vote":
		var p struct {
			Vote string `json:"vote"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		switch p.Vote {
		case "forShukh": // «За ШУХ» → confirm on target
			return engine.Vote{Support: false}, nil
		case "againstShukh": // «Против ШУХа» → challenge, Support:true (§7.2)
			return engine.Vote{Support: true}, nil
		default:
			return nil, fmt.Errorf("server: unknown vote %q", p.Vote)
		}
	default:
		return nil, fmt.Errorf("server: unknown action type %q", head.Type)
	}
}

// encodeAction is the reverse of decodeAction: it serializes an engine.Action back
// to the tagged-union JSON (used for the `legal` list in updates).
func encodeAction(a engine.Action) any {
	switch act := a.(type) {
	case engine.PlayCard:
		return map[string]any{"type": "playCard", "card": encodeCard(act.Card)}
	case engine.TakeBottomAndPass:
		return map[string]any{"type": "takeBottomAndPass"}
	case engine.PodkladkaWest:
		return map[string]any{"type": "podkladkaWest"}
	case engine.DiscardWest:
		return map[string]any{"type": "discardWest"}
	case engine.ClaimShukh:
		return map[string]any{"type": "claimShukh", "target": int(act.Target), "code": int(act.Code)}
	case engine.GiveShukhCard:
		return map[string]any{"type": "giveShukhCard", "card": encodeCard(act.Card)}
	case engine.TakeShukhCards:
		return map[string]any{"type": "takeShukhCards", "seat": int(act.Seat)}
	case engine.DeclareOneCard:
		return map[string]any{"type": "declareOneCard", "seat": int(act.Seat)}
	case engine.AskCount:
		return map[string]any{"type": "askCount", "target": int(act.Target)}
	case engine.AskAboutWest:
		return map[string]any{"type": "askAboutWest", "target": int(act.Target)}
	case engine.ClaimSubjective:
		return map[string]any{"type": "claimSubjective", "claimant": int(act.Claimant), "target": int(act.Target), "code": int(act.Code)}
	case engine.Vote:
		v := "forShukh"
		if act.Support {
			v = "againstShukh"
		}
		return map[string]any{"type": "vote", "vote": v}
	default:
		return map[string]any{"type": "unknown"}
	}
}

// withActor stamps the authenticated seat onto the self-referential fields the
// client must not choose (§7.2). Actions without such a field pass through.
func withActor(a engine.Action, seat engine.SeatID) engine.Action {
	switch act := a.(type) {
	case engine.Vote:
		act.Voter = seat
		return act
	case engine.ClaimSubjective:
		act.Claimant = seat
		return act
	case engine.DeclareOneCard:
		act.Seat = seat
		return act
	case engine.TakeShukhCards:
		act.Seat = seat
		return act
	}
	return a
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/ -run 'TestCard|TestDecode|TestWithActor'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/protocol.go server/protocol_action_test.go
git commit -m "feat(server): кодек ч.1 — карты (глифы) + действия ↔ JSON (W-3, §7.3)"
```

---

### Task 12: `server/protocol.go` (кодек ч.2) — события/вид/апдейт + конверты

**Files:**
- Modify: `server/protocol.go`
- Test: `server/protocol_msg_test.go` (Create)

**Interfaces:**
- Consumes: `engine.Event` (все варианты, incl. `VoteOpened`/`VoteResolved`), `engine.SeatView` (incl. `Vote *VoteView`), `game.Update`, `game.Config`.
- Produces: `func encodeEvent(engine.Event) any`; `func encodeView(*engine.SeatView) any`; `func encodeUpdate(you engine.SeatID, roomCode string, u game.Update, voteDeadline *int64) ServerMsg`; `type ClientMsg struct{...}`; `type ServerMsg struct{...}`; `type ConfigDTO struct{...}` + `(ConfigDTO).toGame() (game.Config, error)`; `func ackMsg(reqID string) ServerMsg`; `func errorMsg(reqID, code, message string) ServerMsg`.

- [ ] **Step 1: Тест — апдейт с `VoteOpened`+`VoteView` даёт нужные ключи; ack/error; декод `ClientMsg`; `ConfigDTO`**

Создать `server/protocol_msg_test.go`:

```go
package server

import (
	"encoding/json"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestEncodeUpdateVoteKeys(t *testing.T) {
	view := &engine.SeatView{
		Phase: engine.Playing,
		You:   0,
		Turn:  1,
		Live:  map[engine.SeatID]bool{0: true, 1: true},
		Vote:  &engine.VoteView{Claimant: 0, Target: 1, Code: engine.Sh6, Voted: []engine.SeatID{0}},
	}
	dl := int64(1700000000000)
	msg := encodeUpdate(0, "ROOM01", game.Update{
		Stage:  game.Playing,
		Roster: []game.SeatMeta{{Seat: 0, Name: "Host"}, {Seat: 1, Name: "Bob"}},
		View:   view,
		Events: []engine.Event{engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6}},
	}, &dl)

	m := asMap(t, msg)
	if m["type"] != "update" || m["roomCode"] != "ROOM01" || m["stage"] != "playing" {
		t.Fatalf("envelope keys wrong: %+v", m)
	}
	if m["you"].(float64) != 0 || m["voteDeadline"].(float64) != 1.7e12 {
		t.Fatalf("you/voteDeadline wrong: %+v", m)
	}
	ev := m["events"].([]any)[0].(map[string]any)
	if ev["type"] != "voteOpened" || ev["claimant"].(float64) != 0 || ev["code"].(float64) != 6 {
		t.Fatalf("voteOpened event wrong: %+v", ev)
	}
	vv := m["view"].(map[string]any)["vote"].(map[string]any)
	if vv["target"].(float64) != 1 || len(vv["voted"].([]any)) != 1 {
		t.Fatalf("view.vote wrong: %+v", vv)
	}
}

func TestAckAndErrorEnvelopes(t *testing.T) {
	a := asMap(t, ackMsg("42"))
	if a["type"] != "ack" || a["reqId"] != "42" {
		t.Fatalf("ack shape: %+v", a)
	}
	e := asMap(t, errorMsg("42", "notYours", "boom"))
	if e["type"] != "error" || e["code"] != "notYours" || e["message"] != "boom" {
		t.Fatalf("error shape: %+v", e)
	}
}

func TestDecodeClientMsgAction(t *testing.T) {
	var msg ClientMsg
	if err := json.Unmarshal([]byte(`{"type":"action","reqId":"7","action":{"type":"takeBottomAndPass"}}`), &msg); err != nil {
		t.Fatalf("unmarshal ClientMsg: %v", err)
	}
	if msg.Type != "action" || msg.ReqID != "7" {
		t.Fatalf("ClientMsg fields: %+v", msg)
	}
	a, err := decodeAction(msg.Action)
	if err != nil {
		t.Fatalf("decodeAction: %v", err)
	}
	if _, ok := a.(engine.TakeBottomAndPass); !ok {
		t.Fatalf("want TakeBottomAndPass, got %T", a)
	}
}

func TestConfigDTOToGame(t *testing.T) {
	cfg, err := ConfigDTO{DeckSize: 36, Mode: "middle"}.toGame()
	if err != nil || cfg.Rules.DeckSize != engine.Deck36 || cfg.Mode != engine.Middle {
		t.Fatalf("toGame: cfg=%+v err=%v", cfg, err)
	}
	if _, err := (ConfigDTO{DeckSize: 36, Mode: "bogus"}).toGame(); err == nil {
		t.Fatal("unknown mode must error")
	}
	if _, err := (ConfigDTO{DeckSize: 99, Mode: "middle"}).toGame(); err == nil {
		t.Fatal("unsupported deck size must error")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run 'TestEncodeUpdate|TestAck|TestDecodeClientMsg|TestConfigDTO'`
Expected: FAIL — `undefined: encodeUpdate / ClientMsg / ServerMsg / ackMsg`.

- [ ] **Step 3: Дописать `server/protocol.go` (события/вид/апдейт + конверты)**

Добавить импорты `strconv` и `game` в блок import:

```go
	"strconv"

	"github.com/oustrix/shukh/game"
```

Добавить в конец файла:

```go
// --- envelopes (§7.1) ---

// ClientMsg is a decoded browser→server message. Action carries the raw union for
// decodeAction; Config is present only for setConfig.
type ClientMsg struct {
	Type   string          `json:"type"` // action | setConfig | start | leave
	Action json.RawMessage `json:"action,omitempty"`
	Config *ConfigDTO      `json:"config,omitempty"`
	ReqID  string          `json:"reqId,omitempty"`
}

// ServerMsg is a server→browser message. One struct covers update|ack|error; unset
// fields are omitted. `you` and `voteDeadline` are pointers so a zero value (seat 0)
// is still emitted.
type ServerMsg struct {
	Type string `json:"type"` // update | ack | error

	// update
	You          *int   `json:"you,omitempty"`
	RoomCode     string `json:"roomCode,omitempty"`
	Stage        string `json:"stage,omitempty"`
	Roster       []any  `json:"roster,omitempty"`
	View         any    `json:"view,omitempty"` // nil in the lobby → omitted (client treats absent as null)
	Legal        []any  `json:"legal,omitempty"`
	Events       []any  `json:"events,omitempty"`
	VoteDeadline *int64 `json:"voteDeadline,omitempty"`

	// ack / error
	ReqID   string `json:"reqId,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ConfigDTO is the wire form of game.Config for setConfig / create.
type ConfigDTO struct {
	DeckSize int    `json:"deckSize"`
	Mode     string `json:"mode"`
}

func (c ConfigDTO) toGame() (game.Config, error) {
	rules := engine.RuleSet{}
	switch c.DeckSize {
	case 36:
		rules.DeckSize = engine.Deck36
	case 52:
		rules.DeckSize = engine.Deck52
	default:
		return game.Config{}, fmt.Errorf("server: unsupported deck size %d", c.DeckSize)
	}
	var mode engine.EnforcementMode
	switch c.Mode {
	case "guard":
		mode = engine.Guard
	case "middle":
		mode = engine.Middle
	case "culture":
		mode = engine.Culture
	default:
		return game.Config{}, fmt.Errorf("server: unknown enforcement mode %q", c.Mode)
	}
	return game.Config{Rules: rules, Mode: mode}, nil
}

func ackMsg(reqID string) ServerMsg { return ServerMsg{Type: "ack", ReqID: reqID} }

func errorMsg(reqID, code, message string) ServerMsg {
	return ServerMsg{Type: "error", ReqID: reqID, Code: code, Message: message}
}

// encodeUpdate serializes a game.Update plus Layer-2 meta into an `update` message.
func encodeUpdate(you engine.SeatID, roomCode string, u game.Update, voteDeadline *int64) ServerMsg {
	roster := make([]any, len(u.Roster))
	for i, m := range u.Roster {
		roster[i] = map[string]any{"seat": int(m.Seat), "name": m.Name}
	}
	legal := make([]any, len(u.Legal))
	for i, a := range u.Legal {
		legal[i] = encodeAction(a)
	}
	events := make([]any, len(u.Events))
	for i, e := range u.Events {
		events[i] = encodeEvent(e)
	}
	yi := int(you)
	return ServerMsg{
		Type:         "update",
		You:          &yi,
		RoomCode:     roomCode,
		Stage:        encodeStage(u.Stage),
		Roster:       roster,
		View:         encodeView(u.View),
		Legal:        legal,
		Events:       events,
		VoteDeadline: voteDeadline,
	}
}

func encodeStage(s game.Lifecycle) string {
	switch s {
	case game.Lobby:
		return "lobby"
	case game.Playing:
		return "playing"
	case game.Finished:
		return "finished"
	}
	return "unknown"
}

func encodePhase(p engine.Phase) string {
	if p == engine.Finished {
		return "finished"
	}
	return "playing"
}

func encodeMode(m engine.EnforcementMode) string {
	switch m {
	case engine.Guard:
		return "guard"
	case engine.Middle:
		return "middle"
	case engine.Culture:
		return "culture"
	}
	return "unknown"
}

func encodeRules(r engine.RuleSet) any {
	return map[string]any{"deckSize": r.DeckSize, "podkladkaSnizu": r.PodkladkaSnizu, "jokers": r.Jokers}
}

func encodeCards(cards []engine.Card) []any {
	out := make([]any, len(cards))
	for i, c := range cards {
		out[i] = encodeCard(c)
	}
	return out
}

func encodeSeats(seats []engine.SeatID) []any {
	out := make([]any, len(seats))
	for i, s := range seats {
		out[i] = int(s)
	}
	return out
}

// encodeView serializes a per-seat projection (D-9), including the new optional
// VoteView (§8.3). Returns nil for a nil view (lobby).
func encodeView(v *engine.SeatView) any {
	if v == nil {
		return nil
	}
	opps := make([]any, len(v.Opponents))
	for i, o := range v.Opponents {
		opps[i] = map[string]any{
			"seat": int(o.Seat), "handCount": o.HandCount,
			"shukhPending": o.ShukhPending, "live": o.Live,
		}
	}
	table := make([]any, len(v.Table))
	for i, tc := range v.Table {
		table[i] = map[string]any{"card": encodeCard(tc.Card), "by": int(tc.By)}
	}
	live := make(map[string]bool, len(v.Live))
	for k, b := range v.Live {
		live[strconv.Itoa(int(k))] = b
	}
	m := map[string]any{
		"rules":        encodeRules(v.Rules),
		"mode":         encodeMode(v.Mode),
		"phase":        encodePhase(v.Phase),
		"you":          int(v.You),
		"turn":         int(v.Turn),
		"hand":         encodeCards(v.Hand),
		"shukhPending": v.ShukhPending,
		"opponents":    opps,
		"table":        table,
		"discard":      v.Discard,
		"talon":        v.Talon,
		"live":         live,
		"finish":       encodeSeats(v.Finish),
	}
	if v.Vote != nil {
		m["vote"] = map[string]any{
			"claimant": int(v.Vote.Claimant),
			"target":   int(v.Vote.Target),
			"code":     int(v.Vote.Code),
			"voted":    encodeSeats(v.Vote.Voted),
		}
	}
	return m
}

// encodeEvent serializes an engine.Event to the GameEvent union (mirrors types.ts).
func encodeEvent(e engine.Event) any {
	switch ev := e.(type) {
	case engine.GameStarted:
		return map[string]any{"type": "gameStarted", "turn": int(ev.Turn)}
	case engine.CardPlayed:
		return map[string]any{"type": "cardPlayed", "seat": int(ev.Seat), "card": encodeCard(ev.Card)}
	case engine.ConClosed:
		return map[string]any{"type": "conClosed", "by": int(ev.By)}
	case engine.ConSwept:
		return map[string]any{"type": "conSwept", "cards": encodeCards(ev.Cards)}
	case engine.PlayerFinished:
		return map[string]any{"type": "playerFinished", "seat": int(ev.Seat), "place": ev.Place}
	case engine.GameFinished:
		return map[string]any{"type": "gameFinished", "finish": encodeSeats(ev.Finish)}
	case engine.CardsTaken:
		return map[string]any{"type": "cardsTaken", "seat": int(ev.Seat), "cards": encodeCards(ev.Cards)}
	case engine.PodkladkaPlayed:
		return map[string]any{"type": "podkladkaPlayed", "seat": int(ev.Seat), "eater": int(ev.Eater)}
	case engine.TurnSkipped:
		return map[string]any{"type": "turnSkipped", "seat": int(ev.Seat)}
	case engine.ShukhAssessed:
		return map[string]any{"type": "shukhAssessed", "offender": int(ev.Offender), "code": int(ev.Code)}
	case engine.ActionReverted:
		return map[string]any{"type": "actionReverted", "seat": int(ev.Seat)}
	case engine.ShukhPaid:
		return map[string]any{"type": "shukhPaid", "offender": int(ev.Offender), "from": int(ev.From), "card": encodeCard(ev.Card)}
	case engine.ShukhCardsTaken:
		return map[string]any{"type": "shukhCardsTaken", "seat": int(ev.Seat), "cards": encodeCards(ev.Cards)}
	case engine.OneCardDeclared:
		return map[string]any{"type": "oneCardDeclared", "seat": int(ev.Seat)}
	case engine.WestDiscarded:
		return map[string]any{"type": "westDiscarded", "seat": int(ev.Seat)}
	case engine.VoteOpened:
		return map[string]any{"type": "voteOpened", "claimant": int(ev.Claimant), "target": int(ev.Target), "code": int(ev.Code)}
	case engine.VoteResolved:
		return map[string]any{"type": "voteResolved", "code": int(ev.Code), "overturned": ev.Overturned}
	default:
		return map[string]any{"type": "unknown"}
	}
}
```

> **Note (W-3).** `encodeEvent`/`encodeView` перечисляют все текущие типы `engine`. Если в
> движке появится новое событие/поле проекции — синхронно дописать сюда и в `web/`-контракт.

- [ ] **Step 4: Запустить — проходит; полный прогон пакета**

Run: `go test ./server/ -run 'TestEncodeUpdate|TestAck|TestDecodeClientMsg|TestConfigDTO'`
Expected: PASS.
Run: `go test ./server/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add server/protocol.go server/protocol_msg_test.go
git commit -m "feat(server): кодек ч.2 — события/вид/апдейт + конверты (voteOpened/VoteView, §7-8)"
```

---
### Task 13: `server/room.go` (ядро) — обёртка сессии + токены + write-through

**Files:**
- Create: `server/room.go`
- Test: `server/room_test.go` (Create)

**Interfaces:**
- Consumes: `game.NewSession`, `(*game.Session).Join`, `(*game.Session).Snapshot() game.SessionState` (Фаза A, durable), `RoomStore`, `Clock`.
- Produces: `type Room struct{...}`; `func NewRoom(code string, cfg game.Config, hostName string, store RoomStore, clock Clock) (*Room, Token)`; `func (*Room) Join(name string) (Token, error)`; `func (*Room) playerFor(tok Token) (game.PlayerID, bool)`; `func (*Room) persist()`; `func (*Room) seatOf(pid game.PlayerID) engine.SeatID`.

> **Имя-заметка (коллизия `Snapshot`).** Фаза A назвала **durable**-снимок `Snapshot() game.SessionState`,
> а per-seat проекцию переименовала в `SnapshotFor(id) (Update, error)`. Этот план зовёт
> `r.session.Snapshot()` для `SessionState` (durable) и `r.session.SnapshotFor(pid)` для per-seat —
> ровно эти имена, менять call-site'ы не нужно.

- [ ] **Step 1: Тест — `NewRoom` сажает хоста и мапит токен; `Join` добавляет место; `persist` пишет загружаемый снимок**

Создать `server/room_test.go`:

```go
package server

import (
	"testing"
	"time"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// cfg36 is the shared 36-card Middle config for server tests.
func cfg36() game.Config {
	return game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
}

func newTestRoom(t *testing.T) (*Room, *MemStore, *fakeClock) {
	t.Helper()
	store := NewMemStore()
	clock := newFakeClock(time.Unix(0, 0))
	r, tok := NewRoom("ROOM01", cfg36(), "Host", store, clock)
	if _, ok := r.playerFor(tok); !ok {
		t.Fatal("host token must map to a player")
	}
	return r, store, clock
}

func TestNewRoomSeatsHost(t *testing.T) {
	store := NewMemStore()
	r, tok := NewRoom("ROOM01", cfg36(), "Host", store, newFakeClock(time.Unix(0, 0)))
	pid, ok := r.playerFor(tok)
	if !ok {
		t.Fatal("host token not registered")
	}
	if st := r.session.Snapshot(); st.Host != pid {
		t.Fatalf("host token maps to %q, but session host is %q", pid, st.Host)
	}
}

func TestRoomJoinAddsSeatAndToken(t *testing.T) {
	r, _, _ := newTestRoom(t)
	tok2, err := r.Join("Bob")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	pid2, ok := r.playerFor(tok2)
	if !ok {
		t.Fatal("joined token not registered")
	}
	st := r.session.Snapshot()
	if len(st.Order) != 2 || st.Order[1] != pid2 {
		t.Fatalf("join did not seat Bob at index 1: order=%v", st.Order)
	}
	if r.seatOf(pid2) != 1 {
		t.Fatalf("seatOf(Bob) = %d, want 1", r.seatOf(pid2))
	}
}

func TestRoomPersistWritesLoadableSnapshot(t *testing.T) {
	r, store, _ := newTestRoom(t)
	snap, ok, err := store.Load("ROOM01")
	if err != nil || !ok {
		t.Fatalf("Load after NewRoom: ok=%v err=%v", ok, err)
	}
	if snap.Code != "ROOM01" || len(snap.Tokens) != 1 {
		t.Fatalf("persisted snapshot wrong: %+v", snap)
	}
	if snap.Session.Host == "" {
		t.Fatal("persisted session must carry a host")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run 'TestNewRoom|TestRoomJoin|TestRoomPersist'`
Expected: FAIL — `undefined: NewRoom / (*Room).Join`.

- [ ] **Step 3: Написать `server/room.go`**

```go
package server

import (
	"sync"
	"time"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// Room wraps one *game.Session with the Layer-2 machinery: room code, token→PlayerID
// table, storage write-through, timers (vote, grace), and connection bookkeeping.
// The session holds all game state; Room adds only transport.
type Room struct {
	mu      sync.Mutex
	code    string
	session *game.Session
	tokens  map[Token]game.PlayerID
	store   RoomStore
	clock   Clock

	// GC bookkeeping (§5): number of live sockets and when it last hit zero.
	live    int
	emptyAt time.Time

	// vote timer (§8) + reconnect grace (§5.4); wired in Task 14.
	voteTimer    Timer
	voteDeadline *int64
	graceTimers  map[game.PlayerID]Timer

	// live sockets, for double-connect eviction (§6); populated in Task 16.
	socks map[game.PlayerID]*wsConn
}

// NewRoom creates a room seated by the host: it builds the session, mints the host
// token, and persists the initial snapshot. Returns the room and the host token.
func NewRoom(code string, cfg game.Config, hostName string, store RoomStore, clock Clock) (*Room, Token) {
	host := newPlayerID()
	r := &Room{
		code:        code,
		session:     game.NewSession(cfg, host, hostName),
		tokens:      map[Token]game.PlayerID{},
		store:       store,
		clock:       clock,
		emptyAt:     clock.Now(), // no sockets yet → eligible for GC after grace if abandoned
		graceTimers: map[game.PlayerID]Timer{},
	}
	tok := r.mintToken(host)
	r.persist()
	return r, tok
}

// newPlayerID mints an opaque, server-private seat identity. It reuses the token
// generator's entropy; a PlayerID never leaves the server.
func newPlayerID() game.PlayerID {
	t, err := newToken()
	if err != nil {
		panic("server: crypto/rand failed minting PlayerID: " + err.Error())
	}
	return game.PlayerID(t)
}

// mintToken registers a fresh token for pid. Caller holds r.mu (or the room is not
// yet shared, as in NewRoom).
func (r *Room) mintToken(pid game.PlayerID) Token {
	tok, err := newToken()
	if err != nil {
		panic("server: crypto/rand failed minting token: " + err.Error())
	}
	r.tokens[tok] = pid
	return tok
}

// Join seats a new player: Session.Join + mint token + persist.
func (r *Room) Join(name string) (Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid := newPlayerID()
	if err := r.session.Join(pid, name); err != nil {
		return "", err
	}
	tok := r.mintToken(pid)
	r.persist()
	return tok, nil
}

// playerFor resolves a token to its PlayerID.
func (r *Room) playerFor(tok Token) (game.PlayerID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, ok := r.tokens[tok]
	return pid, ok
}

// seatOf resolves pid to its current seat index via the durable session order, or -1
// if not seated. Cheap enough for per-update use in the MVP.
func (r *Room) seatOf(pid game.PlayerID) engine.SeatID {
	for i, p := range r.session.Snapshot().Order {
		if p == pid {
			return engine.SeatID(i)
		}
	}
	return -1
}

// persist write-throughs the durable snapshot (§4). Caller holds r.mu (except the
// pre-publication call in NewRoom).
func (r *Room) persist() {
	snap := RoomSnapshot{
		Code:    r.code,
		Tokens:  r.tokens,
		Session: r.session.Snapshot(),
	}
	_ = r.store.Save(snap) // MemStore never errors; a real store would log/retry
}
```

- [ ] **Step 4: Запустить — проходит**

Run: `go test ./server/ -run 'TestNewRoom|TestRoomJoin|TestRoomPersist'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/room.go server/room_test.go
git commit -m "feat(server): Room-ядро — сессия + токены + write-through (§3.1/§4)"
```

---

### Task 14: `server/room.go` — таймер голосования + grace/дисконнект + миграция хоста

**Files:**
- Modify: `server/room.go`
- Test: `server/room_vote_test.go` (Create)

**Interfaces:**
- Consumes: `(*game.Session).CloseVote() ([]engine.Event, error)` (Фаза A), `(*game.Session).Leave`, `(*game.Session).Stage`, `Clock.AfterFunc`, `engine.VoteOpened`, `engine.VoteResolved`.
- Produces: `func (*Room) commit(events []engine.Event)`; `func (*Room) currentVoteDeadline() *int64`; `func (*Room) onDisconnect(pid game.PlayerID)`; неэкспортируемые `armVote`/`disarmVote`/`fireVote`/`graceExpired`/`cancelGrace`; константы `voteTTL`, `graceTTL`.

- [ ] **Step 1: Тест — `VoteOpened` взводит таймер, `Advance(voteTTL)` резолвит; полная явка гасит таймер до дедлайна**

Создать `server/room_vote_test.go`:

```go
package server

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// openVoteRoom returns a started 2-player room with an R-8.6 vote open (host claimed
// Ш-6 against seat 1), the timer armed via commit.
func openVoteRoom(t *testing.T) (*Room, *fakeClock) {
	t.Helper()
	r, _, clock := newTestRoom(t)
	if _, err := r.Join("Bob"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	host := r.session.Snapshot().Host
	if err := r.session.Start(host, 42); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.mu.Lock()
	evs, err := r.session.Submit(host, engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6})
	if err != nil {
		r.mu.Unlock()
		t.Fatalf("claim: %v", err)
	}
	r.commit(evs)
	r.mu.Unlock()
	return r, clock
}

func TestVoteTimerFiresAndResolves(t *testing.T) {
	r, clock := openVoteRoom(t)
	r.mu.Lock()
	armed := r.currentVoteDeadline() != nil
	r.mu.Unlock()
	if !armed {
		t.Fatal("VoteOpened must arm the vote deadline")
	}
	if r.session.Snapshot().Game.Adjudication == nil {
		t.Fatal("precondition: an Adjudication must be open")
	}
	clock.Advance(voteTTL) // fire → CloseVote → resolve
	if r.session.Snapshot().Game.Adjudication != nil {
		t.Fatal("timer expiry must resolve and clear the vote")
	}
	r.mu.Lock()
	stillArmed := r.currentVoteDeadline() != nil
	r.mu.Unlock()
	if stillArmed {
		t.Fatal("deadline must be cleared after resolution")
	}
}

func TestFullTurnoutStopsTimer(t *testing.T) {
	r, clock := openVoteRoom(t)
	snap := r.session.Snapshot()
	host := snap.Host
	p2 := snap.Order[1]
	// both seats vote → auto-resolve before the deadline.
	r.mu.Lock()
	e1, _ := r.session.Submit(host, engine.Vote{Voter: 0, Support: false})
	r.commit(e1)
	e2, _ := r.session.Submit(p2, engine.Vote{Voter: 1, Support: false})
	r.commit(e2)
	cleared := r.currentVoteDeadline() == nil
	r.mu.Unlock()
	if !cleared {
		t.Fatal("full turnout must disarm the vote timer")
	}
	if r.session.Snapshot().Game.Adjudication != nil {
		t.Fatal("full turnout must resolve the vote")
	}
	clock.Advance(voteTTL) // no-op: CloseVote on a nil Adjudication; must not panic
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run 'TestVoteTimer|TestFullTurnout'`
Expected: FAIL — `undefined: (*Room).commit / voteTTL / currentVoteDeadline`.

- [ ] **Step 3: Дописать таймеры/grace в `server/room.go`**

Добавить константы (после блока import):

```go
const (
	voteTTL  = 30 * time.Second // R-8.6 vote deadline (L2-1)
	graceTTL = 5 * time.Minute  // reconnect grace before a lobby Leave (§5.4)
)
```

Добавить методы в конец файла:

```go
// commit records the durable snapshot and updates the vote timer after a
// state-changing session call. A VoteOpened in the events arms the deadline; a
// VoteResolved disarms it. Caller holds r.mu.
func (r *Room) commit(events []engine.Event) {
	for _, e := range events {
		switch e.(type) {
		case engine.VoteOpened:
			r.armVote()
		case engine.VoteResolved:
			r.disarmVote()
		}
	}
	r.persist()
}

// armVote sets a voteTTL deadline whose expiry closes the vote. Caller holds r.mu.
func (r *Room) armVote() {
	if r.voteTimer != nil {
		r.voteTimer.Stop()
	}
	deadline := r.clock.Now().Add(voteTTL).UnixMilli()
	r.voteDeadline = &deadline
	r.voteTimer = r.clock.AfterFunc(voteTTL, r.fireVote)
}

// disarmVote cancels the vote timer and clears the deadline. Caller holds r.mu.
func (r *Room) disarmVote() {
	if r.voteTimer != nil {
		r.voteTimer.Stop()
		r.voteTimer = nil
	}
	r.voteDeadline = nil
}

// fireVote runs on the clock goroutine when the deadline elapses: it closes the vote
// with the current ballots (CloseVote fans the resolution out itself) and updates
// the timer state. A resolve that already happened makes CloseVote a no-op (§8.2).
func (r *Room) fireVote() {
	r.mu.Lock()
	defer r.mu.Unlock()
	events, err := r.session.CloseVote()
	if err != nil {
		return
	}
	r.commit(events)
}

// currentVoteDeadline returns a copy of the active vote deadline (unix-ms) or nil.
// Caller holds r.mu.
func (r *Room) currentVoteDeadline() *int64 {
	if r.voteDeadline == nil {
		return nil
	}
	d := *r.voteDeadline
	return &d
}

// onDisconnect starts a grace timer for pid after its socket drops (§5.4). It does
// NOT Leave immediately: the seat is held so a reconnect within grace is seamless.
func (r *Room) onDisconnect(pid game.PlayerID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.graceTimers[pid]; ok {
		t.Stop()
	}
	r.graceTimers[pid] = r.clock.AfterFunc(graceTTL, func() { r.graceExpired(pid) })
}

// graceExpired runs when a disconnect's grace elapses. In the Lobby it Leaves (which
// migrates the host per L2-3); while Playing the seat is kept — the engine has no
// fold, so a general turn-timeout is future work (§12).
func (r *Room) graceExpired(pid game.PlayerID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.graceTimers, pid)
	if r.session.Stage() == game.Lobby {
		r.session.Leave(pid)
		r.persist()
	}
}

// cancelGrace stops any pending grace timer for pid (on reconnect). Caller holds r.mu.
func (r *Room) cancelGrace(pid game.PlayerID) {
	if t, ok := r.graceTimers[pid]; ok {
		t.Stop()
		delete(r.graceTimers, pid)
	}
}
```

> **Concurrency note.** `commit`/`armVote` register `fireVote` via `clock.AfterFunc`. With the
> real clock `fireVote` runs on its own goroutine and takes `r.mu`; with `fakeClock` it runs
> synchronously inside `Advance` (fired with the fake clock's lock released, so `fireVote` may
> re-arm timers without deadlock). Tests call `Advance` WITHOUT holding `r.mu`.

- [ ] **Step 4: Запустить — проходит; полный прогон пакета с гонками**

Run: `go test ./server/ -run 'TestVoteTimer|TestFullTurnout'`
Expected: PASS.
Run: `go test -race ./server/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add server/room.go server/room_vote_test.go
git commit -m "feat(server): таймер R-8.6 (CloseVote по дедлайну) + grace/дисконнект (L2-1/§5.4)"
```

---
### Task 15: `server/hub.go` — реестр комнат + GC

**Files:**
- Create: `server/hub.go`
- Modify: `server/room.go` (методы учёта соединений + `collectible`)
- Test: `server/hub_test.go` (Create)

**Interfaces:**
- Consumes: `RoomStore`, `Clock`, `NewRoom`, `newCode`, `(*game.Session).Stage`.
- Produces: `type Hub struct{...}`; `func NewHub(store RoomStore, clock Clock) *Hub`; `func (*Hub) CreateRoom(cfg game.Config, hostName string) (string, Token, *Room)`; `func (*Hub) Room(code string) (*Room, bool)`; `func (*Hub) sweep()`; `func (*Hub) StartSweeper()`; `func (*Room) noteConnOpened()`; `func (*Room) noteConnClosed()`; `func (*Room) collectible(now time.Time) bool`.

- [ ] **Step 1: Тест — `CreateRoom` отдаёт уникальный извлекаемый код; `sweep` убирает пустую истёкшую и держит живую**

Создать `server/hub_test.go`:

```go
package server

import (
	"testing"
	"time"
)

func TestCreateRoomRetrievableUniqueCode(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	code, tok, r := h.CreateRoom(cfg36(), "Host")
	if code == "" || len(code) != 6 {
		t.Fatalf("bad code %q", code)
	}
	got, ok := h.Room(code)
	if !ok || got != r {
		t.Fatal("created room must be retrievable by code")
	}
	if _, ok := r.playerFor(tok); !ok {
		t.Fatal("host token must belong to the room")
	}
	code2, _, _ := h.CreateRoom(cfg36(), "Host2")
	if code2 == code {
		t.Fatal("codes must be unique across rooms")
	}
}

func TestSweepRemovesEmptyExpiredKeepsLive(t *testing.T) {
	clock := newFakeClock(time.Unix(0, 0))
	h := NewHub(NewMemStore(), clock)
	liveCode, _, live := h.CreateRoom(cfg36(), "Live")
	deadCode, _, _ := h.CreateRoom(cfg36(), "Dead")

	// keep the live room busy with a socket
	live.mu.Lock()
	live.noteConnOpened()
	live.mu.Unlock()

	clock.Advance(graceTTL + time.Minute)
	h.sweep()

	if _, ok := h.Room(liveCode); !ok {
		t.Fatal("a room with a live socket must survive sweep")
	}
	if _, ok := h.Room(deadCode); ok {
		t.Fatal("an empty room past grace must be swept")
	}
	if _, ok, _ := h.store.Load(deadCode); ok {
		t.Fatal("sweep must also delete the room from the store")
	}
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run 'TestCreateRoom|TestSweep'`
Expected: FAIL — `undefined: NewHub / (*Room).noteConnOpened`.

- [ ] **Step 3: Написать `server/hub.go`**

```go
package server

import (
	"sync"
	"time"

	"github.com/oustrix/shukh/game"
)

const (
	idleTTL       = 2 * time.Minute // Finished room lingers this long after going empty
	sweepInterval = 1 * time.Minute // background GC cadence
)

// Hub is the registry of rooms by code and their garbage collector (§3.1). Its only
// storage dependency is RoomStore; all timing goes through Clock.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
	store RoomStore
	clock Clock
}

// NewHub builds an empty hub over the given store and clock.
func NewHub(store RoomStore, clock Clock) *Hub {
	return &Hub{rooms: map[string]*Room{}, store: store, clock: clock}
}

// CreateRoom mints a collision-free code, creates the room (seating the host), and
// registers it. Returns the code, host token, and room.
func (h *Hub) CreateRoom(cfg game.Config, hostName string) (string, Token, *Room) {
	h.mu.Lock()
	defer h.mu.Unlock()
	code := h.freeCodeLocked()
	r, tok := NewRoom(code, cfg, hostName, h.store, h.clock)
	h.rooms[code] = r
	return code, tok, r
}

// freeCodeLocked generates codes until one is not already in the registry. Caller
// holds h.mu.
func (h *Hub) freeCodeLocked() string {
	for {
		c := newCode(cryptoBytes())
		if _, ok := h.rooms[c]; !ok {
			return c
		}
	}
}

// Room returns the room for a code, if present.
func (h *Hub) Room(code string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	return r, ok
}

// sweep removes collectible rooms (no live sockets past grace, or Finished past
// idle-TTL) from the registry and the store. Lock order: h.mu then each r.mu.
func (h *Hub) sweep() {
	now := h.clock.Now()
	h.mu.Lock()
	var dead []string
	for code, r := range h.rooms {
		if r.collectible(now) {
			dead = append(dead, code)
		}
	}
	for _, code := range dead {
		delete(h.rooms, code)
	}
	h.mu.Unlock()
	for _, code := range dead {
		_ = h.store.Delete(code)
	}
}

// StartSweeper schedules recurring GC via the clock. Real time in production; tests
// call sweep() directly with a fake clock instead.
func (h *Hub) StartSweeper() {
	h.clock.AfterFunc(sweepInterval, func() {
		h.sweep()
		h.StartSweeper()
	})
}
```

- [ ] **Step 4: Дописать учёт соединений + `collectible` в `server/room.go`**

Добавить в конец `server/room.go`:

```go
// noteConnOpened / noteConnClosed maintain the live-socket count and the empty
// timestamp used by Hub.sweep. Caller holds r.mu.
func (r *Room) noteConnOpened() { r.live++ }

func (r *Room) noteConnClosed() {
	if r.live > 0 {
		r.live--
	}
	if r.live == 0 {
		r.emptyAt = r.clock.Now()
	}
}

// collectible reports whether the room may be garbage-collected at now: no live
// sockets for longer than grace, or Finished and idle past idleTTL (§5.3).
func (r *Room) collectible(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live > 0 {
		return false
	}
	if r.session.Stage() == game.Finished {
		return now.Sub(r.emptyAt) >= idleTTL
	}
	return now.Sub(r.emptyAt) >= graceTTL
}
```

- [ ] **Step 5: Запустить — проходит; полный прогон с гонками**

Run: `go test ./server/ -run 'TestCreateRoom|TestSweep'`
Expected: PASS.
Run: `go test -race ./server/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add server/hub.go server/room.go server/hub_test.go
git commit -m "feat(server): Hub — реестр комнат + GC-свипер (§3.1/§5)"
```

---

### Task 16: `server/conn.go` — WebSocket-соединение (два потока)

**Files:**
- Create: `server/conn.go`
- Modify: `go.mod`, `go.sum` (зависимость `github.com/coder/websocket`)
- Test: `server/conn_test.go` (Create)

**Interfaces:**
- Consumes: `github.com/coder/websocket`, `(*game.Session).Subscribe` (close-and-replace, Фаза A), `(*game.Session).Submit/SetConfig/Start/Leave`, `(*game.Session).SnapshotFor`, `decodeAction`, `withActor`, `encodeUpdate`, `ackMsg`, `errorMsg`, `commit`, `currentVoteDeadline`, `seatOf`.
- Produces: `func (*Room) serveConn(ctx context.Context, c *websocket.Conn, pid game.PlayerID)`; неэкспортируемые `wsConn`, `registerConn`, `unregisterConn`, `writePump`, `writeMsg`, `voteDeadlineSafe`, `readPump`, `dispatch`, `afterLobbyChange`, `reply`, `codeFor`.

> **Поле `socks`** уже объявлено в `Room` (Task 13); `registerConn` лениво его инициализирует.

- [ ] **Step 1: Добавить зависимость `coder/websocket` (inline GOPROXY)**

Run:
```bash
GOPROXY="$(go env GOPROXY),https://proxy.golang.org,direct" go get github.com/coder/websocket@latest
```
Expected: `go.mod`/`go.sum` получают `github.com/coder/websocket vX.Y.Z`; глобальный GOPROXY-дефолт не меняется (корп-прокси остаётся первым — inline только для этой команды, по памяти проекта).

- [ ] **Step 2: Тест — действие round-trip'ится в ack + результирующий update (пара через `httptest`)**

Создать `server/conn_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/game"
)

// wsEcho stands up an httptest server that accepts a WS and runs serveConn for pid.
func wsEcho(t *testing.T, r *Room, pid game.PlayerID) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		r.serveConn(req.Context(), c, pid)
	}))
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, url
}

func readMsg(t *testing.T, ctx context.Context, c *websocket.Conn) map[string]any {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestConnActionAcksAndUpdates(t *testing.T) {
	r, _, _ := newTestRoom(t)
	if _, err := r.Join("Bob"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	host := r.session.Snapshot().Host
	if err := r.session.Start(host, 42); err != nil {
		t.Fatalf("Start: %v", err)
	}

	srv, url := wsEcho(t, r, host)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	// initial snapshot on connect
	first := readMsg(t, ctx, c)
	if first["type"] != "update" {
		t.Fatalf("first message must be an update, got %v", first["type"])
	}

	// host raises a subjective ШУХ (legal with gates closed, any turn) → ack + update
	send := map[string]any{
		"type":   "action",
		"reqId":  "1",
		"action": map[string]any{"type": "claimSubjective", "target": 1, "code": 6},
	}
	data, _ := json.Marshal(send)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	sawAck, sawVoteOpen := false, false
	for i := 0; i < 6 && !(sawAck && sawVoteOpen); i++ {
		m := readMsg(t, ctx, c)
		switch m["type"] {
		case "ack":
			if m["reqId"] == "1" {
				sawAck = true
			}
		case "update":
			if evs, ok := m["events"].([]any); ok {
				for _, e := range evs {
					if em, ok := e.(map[string]any); ok && em["type"] == "voteOpened" {
						sawVoteOpen = true
					}
				}
			}
		case "error":
			t.Fatalf("unexpected error: %+v", m)
		}
	}
	if !sawAck || !sawVoteOpen {
		t.Fatalf("expected ack (%v) and a voteOpened update (%v)", sawAck, sawVoteOpen)
	}
}
```

- [ ] **Step 3: Написать `server/conn.go`**

```go
package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/game"
)

// wsConn is one live socket. All writes for a socket go through its write pump (the
// single writer), so acks/errors and nudged snapshots are queued on out/nudge rather
// than written from the read goroutine.
type wsConn struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	out    chan ServerMsg // serialized outbound acks/errors
	nudge  chan struct{}  // request to re-snapshot (after non-fanout transitions)
}

// serveConn runs the two pumps for one authenticated socket. Subscribe uses
// close-and-replace (Фаза A), so a reconnect is clean; a double-connect evicts the
// prior socket in registerConn.
func (r *Room) serveConn(ctx context.Context, c *websocket.Conn, pid game.PlayerID) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wc := r.registerConn(pid, c, cancel)
	defer r.unregisterConn(pid, c)

	ch, unsub, err := r.session.Subscribe(pid)
	if err != nil {
		return
	}
	defer unsub()

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.writePump(ctx, wc, ch, pid)
	}()
	r.readPump(ctx, wc, pid)
	cancel()
	<-done
}

// registerConn installs wc as pid's live socket, evicting any prior one (double
// connect closes the first cleanly, §11) and cancelling any pending grace (reconnect).
func (r *Room) registerConn(pid game.PlayerID, c *websocket.Conn, cancel context.CancelFunc) *wsConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.socks == nil {
		r.socks = map[game.PlayerID]*wsConn{}
	}
	had := false
	if old, ok := r.socks[pid]; ok {
		old.cancel() // evict prior socket; its pumps unwind, its serveConn returns
		had = true
	}
	r.cancelGrace(pid)
	wc := &wsConn{conn: c, cancel: cancel, out: make(chan ServerMsg, 8), nudge: make(chan struct{}, 1)}
	r.socks[pid] = wc
	if !had {
		r.noteConnOpened()
	}
	return wc
}

// unregisterConn removes c if it is still pid's current socket, then starts grace.
// On eviction (c already replaced) it is a no-op — the replacement holds the seat.
func (r *Room) unregisterConn(pid game.PlayerID, c *websocket.Conn) {
	r.mu.Lock()
	removed := false
	if wc, ok := r.socks[pid]; ok && wc.conn == c {
		delete(r.socks, pid)
		r.noteConnClosed()
		removed = true
	}
	r.mu.Unlock()
	if removed {
		r.onDisconnect(pid) // start the grace timer (§5.4)
	}
}

// writePump is the sole writer for wc. It serializes subscription Updates, queued
// acks/errors, and nudged re-snapshots onto the socket.
func (r *Room) writePump(ctx context.Context, wc *wsConn, ch <-chan game.Update, pid game.PlayerID) {
	for {
		select {
		case <-ctx.Done():
			return
		case up, ok := <-ch:
			if !ok {
				return // Subscribe closed (unsub / close-and-replace)
			}
			r.writeMsg(ctx, wc, encodeUpdate(r.seatOf(pid), r.code, up, r.voteDeadlineSafe()))
		case m := <-wc.out:
			r.writeMsg(ctx, wc, m)
		case <-wc.nudge:
			if up, err := r.session.SnapshotFor(pid); err == nil {
				r.writeMsg(ctx, wc, encodeUpdate(r.seatOf(pid), r.code, up, r.voteDeadlineSafe()))
			}
		}
	}
}

func (r *Room) writeMsg(ctx context.Context, wc *wsConn, m ServerMsg) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = wc.conn.Write(ctx, websocket.MessageText, data)
}

func (r *Room) voteDeadlineSafe() *int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentVoteDeadline()
}

// readPump decodes ClientMsgs and dispatches them until the socket closes.
func (r *Room) readPump(ctx context.Context, wc *wsConn, pid game.PlayerID) {
	for {
		typ, data, err := wc.conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var msg ClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			r.reply(wc, errorMsg("", "badRequest", err.Error()))
			continue
		}
		r.dispatch(ctx, wc, pid, msg)
	}
}

// dispatch routes one ClientMsg to the session and replies ack/error.
func (r *Room) dispatch(ctx context.Context, wc *wsConn, pid game.PlayerID, msg ClientMsg) {
	switch msg.Type {
	case "action":
		act, err := decodeAction(msg.Action)
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, "badAction", err.Error()))
			return
		}
		act = withActor(act, r.seatOf(pid))
		events, err := r.session.Submit(pid, act)
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.mu.Lock()
		r.commit(events) // arms/disarms vote timer + persists; Submit already fanned out
		r.mu.Unlock()
		r.reply(wc, ackMsg(msg.ReqID))
	case "setConfig":
		if msg.Config == nil {
			r.reply(wc, errorMsg(msg.ReqID, "badRequest", "missing config"))
			return
		}
		cfg, err := msg.Config.toGame()
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, "badRequest", err.Error()))
			return
		}
		if err := r.session.SetConfig(pid, cfg); err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.afterLobbyChange()
		r.reply(wc, ackMsg(msg.ReqID))
	case "start":
		seed := r.clock.Now().UnixNano() // SERVER-generated seed (L2-8)
		if err := r.session.Start(pid, seed); err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.afterLobbyChange() // Start does not fan out → nudge sockets to re-snapshot
		r.reply(wc, ackMsg(msg.ReqID))
	case "leave":
		r.session.Leave(pid)
		r.afterLobbyChange()
		r.reply(wc, ackMsg(msg.ReqID))
	default:
		r.reply(wc, errorMsg(msg.ReqID, "badRequest", "unknown message type"))
	}
}

// afterLobbyChange persists and nudges every socket to re-snapshot. Used after
// transitions that change state without a Session fanout (SetConfig/Start/Leave).
func (r *Room) afterLobbyChange() {
	r.mu.Lock()
	r.persist()
	for _, wc := range r.socks {
		select {
		case wc.nudge <- struct{}{}:
		default:
		}
	}
	r.mu.Unlock()
}

// reply queues an ack/error for the write pump. A full outbound buffer drops it (the
// client re-syncs from the next update).
func (r *Room) reply(wc *wsConn, m ServerMsg) {
	select {
	case wc.out <- m:
	default:
	}
}

// codeFor maps game sentinel errors to stable protocol error codes (§10). Any other
// error is an engine rule rejection (engine.IllegalAction) with state untouched; we
// return a generic code rather than couple to the engine error's concrete type.
func codeFor(err error) string {
	switch {
	case errors.Is(err, game.ErrNotYours):
		return "notYours"
	case errors.Is(err, game.ErrNotPlaying):
		return "notPlaying"
	case errors.Is(err, game.ErrNotHost):
		return "notHost"
	case errors.Is(err, game.ErrNotLobby):
		return "notLobby"
	case errors.Is(err, game.ErrTooFewPlayers):
		return "tooFewPlayers"
	case errors.Is(err, game.ErrUnknownPlayer):
		return "seatNotFound"
	case errors.Is(err, game.ErrFull):
		return "full"
	case errors.Is(err, game.ErrDuplicate):
		return "duplicate"
	}
	return "illegalAction"
}
```

- [ ] **Step 4: Запустить — проходит; полный прогон с гонками**

Run: `go test ./server/ -run TestConnAction`
Expected: PASS.
Run: `go test -race ./server/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add server/conn.go go.mod go.sum
git commit -m "feat(server): WS-соединение — read/write pumps, ack/error, close-and-replace (§7/§10)"
```

---
### Task 17: `server/http.go` — HTTP-хендлеры (`create`/`join`/WS-апгрейд)

**Files:**
- Create: `server/http.go`
- Test: `server/http_test.go` (Create)

**Interfaces:**
- Consumes: `*Hub`, `(*Hub).CreateRoom/Room`, `(*Room).Join/playerFor/seatOf/serveConn`, `github.com/coder/websocket`, `ConfigDTO`.
- Produces: `type Server struct{...}`; `func NewServer(hub *Hub) *Server`; `func (*Server) Handler() http.Handler`; неэкспортируемые `createRoom`/`joinRoom`/`connect`/`roomCookie`/`cookieName`/`writeJSON`.

- [ ] **Step 1: Тест — create → join ставит куку → WS с кукой ОК; WS без куки отклонён 401**

Создать `server/http_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestHTTPCreateJoinConnect(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h).Handler())
	defer srv.Close()

	// create room
	resp, err := http.Post(srv.URL+"/r", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.Code == "" {
		t.Fatal("create must return a code")
	}
	hostCookie := findCookie(resp.Cookies(), cookieName(created.Code))
	if hostCookie == nil {
		t.Fatal("create must Set-Cookie the host token")
	}
	if !hostCookie.HttpOnly {
		t.Fatal("token cookie must be HttpOnly (L2-6)")
	}

	// join room → seat + cookie
	jresp, err := http.Post(srv.URL+"/r/"+created.Code+"/join", "application/json", strings.NewReader(`{"name":"Bob"}`))
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	var joined struct {
		Seat int `json:"seat"`
	}
	_ = json.NewDecoder(jresp.Body).Decode(&joined)
	jresp.Body.Close()
	if joined.Seat != 1 {
		t.Fatalf("Bob must be seat 1, got %d", joined.Seat)
	}
	bobCookie := findCookie(jresp.Cookies(), cookieName(created.Code))
	if bobCookie == nil {
		t.Fatal("join must Set-Cookie the seat token")
	}

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/r/" + created.Code
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// WS with cookie succeeds
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {bobCookie.Name + "=" + bobCookie.Value}},
	})
	if err != nil {
		t.Fatalf("WS dial with cookie failed: %v", err)
	}
	c.Close(websocket.StatusNormalClosure, "")

	// WS without cookie is rejected (401 seatNotFound, §10)
	_, resp2, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("WS without cookie must be rejected")
	}
	if resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", resp2)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
```

- [ ] **Step 2: Запустить — падает компиляцией**

Run: `go test ./server/ -run TestHTTPCreateJoinConnect`
Expected: FAIL — `undefined: NewServer / cookieName`.

- [ ] **Step 3: Написать `server/http.go`**

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// Server wires HTTP handlers to a Hub: room creation, join (mint token + cookie),
// and the WS upgrade. Identity is a per-room HttpOnly cookie (L2-6).
type Server struct {
	hub *Hub
}

func NewServer(hub *Hub) *Server { return &Server{hub: hub} }

// Handler builds the router. Go 1.22 method+path patterns; {code} via PathValue.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /r", s.createRoom)
	mux.HandleFunc("POST /r/{code}/join", s.joinRoom)
	mux.HandleFunc("GET /r/{code}", s.connect)
	return mux
}

// cookieName scopes one cookie per room so several rooms coexist in one browser.
func cookieName(code string) string { return "shukh_" + code }

func roomCookie(code string, tok Token) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName(code),
		Value:    string(tok),
		Path:     "/r/" + code,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true behind TLS in production.
	}
}

func (s *Server) createRoom(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Config *ConfigDTO `json:"config"`
		Name   string     `json:"name"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	cfg := game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
	if body.Config != nil {
		c, err := body.Config.toGame()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg = c
	}
	name := body.Name
	if name == "" {
		name = "Host"
	}
	code, tok, _ := s.hub.CreateRoom(cfg, name)
	http.SetCookie(w, roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

func (s *Server) joinRoom(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.hub.Room(code)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "Player"
	}
	tok, err := room.Join(body.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	pid, _ := room.playerFor(tok)
	http.SetCookie(w, roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]int{"seat": int(room.seatOf(pid))})
}

func (s *Server) connect(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.hub.Room(code)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	ck, err := req.Cookie(cookieName(code))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	pid, ok := room.playerFor(Token(ck.Value))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	room.serveConn(req.Context(), c, pid)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Запустить — проходит; полный прогон с гонками**

Run: `go test ./server/ -run TestHTTPCreateJoinConnect`
Expected: PASS.
Run: `go test -race ./server/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add server/http.go server/http_test.go
git commit -m "feat(server): HTTP — /r create, /r/{code}/join (HttpOnly кука), WS-апгрейд (§6/§10)"
```

---

### Task 18: `cmd/shukh-server/main.go` — точка запуска

**Files:**
- Create: `cmd/shukh-server/main.go`

**Interfaces:**
- Consumes: `server.NewMemStore`, `server.NewRealClock`, `server.NewHub`, `(*server.Hub).StartSweeper`, `server.NewServer`, `(*server.Server).Handler`.

- [ ] **Step 1: Написать `cmd/shukh-server/main.go`**

```go
// Command shukh-server runs the Layer-2 WebSocket server: a Hub over an in-memory
// RoomStore and the real clock, wired to the HTTP handlers, with the GC sweeper
// running on the clock (L2-5/L2-9). MVP: single instance, state in memory (§12).
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/oustrix/shukh/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	hub := server.NewHub(server.NewMemStore(), server.NewRealClock())
	hub.StartSweeper()

	handler := server.NewServer(hub).Handler()
	log.Printf("shukh-server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
```

- [ ] **Step 2: Build-smoke**

Run: `go build ./cmd/shukh-server`
Expected: no output (builds clean).

- [ ] **Step 3: Commit**

```bash
git add cmd/shukh-server/main.go
git commit -m "feat(server): точка запуска cmd/shukh-server (MemStore + realClock + Hub, §3.2)"
```

---

# ФАЗА C — Сквозной тест и документация

### Task 19: `server/integration_test.go` — сквозной сетевой сценарий

Фокус — Layer-2-специфика: тайм-аут голосования по сети (резолв доходит до ОБОИХ клиентов),
реконнект и double-connect. Полную партию до финиша не гоняем — это уже покрыто
`game/integration_test.go` детерминированно; здесь важно, что сеть корректно ретранслирует.

**Files:**
- Create: `server/integration_test.go`

**Interfaces:**
- Consumes: весь публичный API `server`; `httptest.Server`; `github.com/coder/websocket`; фейковый `Clock` в `Hub`; `findCookie`/`cookieName` (Task 17).

- [ ] **Step 1: Тест — create → join×2 → start → голосование по дедлайну доходит до обоих → реконнект → double-connect**

Создать `server/integration_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type wsClient struct {
	t   *testing.T
	c   *websocket.Conn
	ctx context.Context
}

func dialClient(t *testing.T, ctx context.Context, wsURL string, cookie *http.Cookie) *wsClient {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &wsClient{t: t, c: c, ctx: ctx}
}

func (w *wsClient) read() map[string]any {
	_, data, err := w.c.Read(w.ctx)
	if err != nil {
		w.t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		w.t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func (w *wsClient) send(v map[string]any) {
	data, _ := json.Marshal(v)
	if err := w.c.Write(w.ctx, websocket.MessageText, data); err != nil {
		w.t.Fatalf("write: %v", err)
	}
}

// readUpdateWithEvent reads until an update carrying an event of type typ.
func (w *wsClient) readUpdateWithEvent(typ string) {
	for i := 0; i < 50; i++ {
		m := w.read()
		if m["type"] == "update" && hasEvent(m, typ) {
			return
		}
	}
	w.t.Fatalf("did not observe an update carrying event %q", typ)
}

// readUntilStage reads until an update reports the given stage.
func (w *wsClient) readUntilStage(stage string) {
	for i := 0; i < 50; i++ {
		if w.read()["stage"] == stage {
			return
		}
	}
	w.t.Fatalf("never reached stage %q", stage)
}

func hasEvent(m map[string]any, typ string) bool {
	evs, _ := m["events"].([]any)
	for _, e := range evs {
		if em, ok := e.(map[string]any); ok && em["type"] == typ {
			return true
		}
	}
	return false
}

func TestIntegrationVoteTimeoutAndReconnect(t *testing.T) {
	clock := newFakeClock(time.Unix(1_000, 0)) // deterministic Now → deterministic start seed
	h := NewHub(NewMemStore(), clock)
	srv := httptest.NewServer(NewServer(h).Handler())
	defer srv.Close()
	base := srv.URL
	wsBase := "ws" + strings.TrimPrefix(base, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// create + join over HTTP (cookies mint identity)
	cResp, _ := http.Post(base+"/r", "application/json", strings.NewReader(`{"name":"Host"}`))
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(cResp.Body).Decode(&created)
	cResp.Body.Close()
	code := created.Code
	hostCookie := findCookie(cResp.Cookies(), cookieName(code))

	jResp, _ := http.Post(base+"/r/"+code+"/join", "application/json", strings.NewReader(`{"name":"Bob"}`))
	jResp.Body.Close()
	bobCookie := findCookie(jResp.Cookies(), cookieName(code))

	wsURL := wsBase + "/r/" + code
	host := dialClient(t, ctx, wsURL, hostCookie)
	defer host.c.CloseNow()
	bob := dialClient(t, ctx, wsURL, bobCookie)
	defer bob.c.CloseNow()

	host.read() // initial lobby snapshot
	bob.read()

	// host starts (server seed from the fake clock → deterministic)
	host.send(map[string]any{"type": "start", "reqId": "start"})
	host.readUntilStage("playing")
	bob.readUntilStage("playing")

	// host opens a subjective vote (gates are closed right after the deal)
	host.send(map[string]any{
		"type": "action", "reqId": "claim",
		"action": map[string]any{"type": "claimSubjective", "target": 1, "code": 6},
	})
	host.readUpdateWithEvent("voteOpened")
	bob.readUpdateWithEvent("voteOpened")

	// the deadline fires → CloseVote resolves with a partial tally → voteResolved to BOTH
	clock.Advance(voteTTL)
	host.readUpdateWithEvent("voteResolved")
	bob.readUpdateWithEvent("voteResolved")

	// --- reconnect: drop host, redial with the same cookie, get a fresh snapshot ---
	host.c.CloseNow()
	host2 := dialClient(t, ctx, wsURL, hostCookie)
	defer host2.c.CloseNow()
	if host2.read()["type"] != "update" {
		t.Fatal("reconnect must deliver a fresh snapshot")
	}

	// --- double-connect: a second dial with the same cookie evicts the first ---
	host3 := dialClient(t, ctx, wsURL, hostCookie)
	defer host3.c.CloseNow()
	host3.read() // fresh snapshot on the new socket
	if _, _, err := host2.c.Read(ctx); err == nil {
		t.Fatal("double-connect must close the previously connected socket")
	}
}
```

> **Driver note.** Тест опирается только на наблюдаемые protocol-сообщения. Если сойдётся не
> сразу — диагностировать через `superpowers:systematic-debugging`, не поднимать бюджет циклов
> вслепую. Тайм-аут `ctx` (15s) страхует от зависания на `read()`.

- [ ] **Step 2: Запустить — проходит**

Run: `go test ./server/ -run TestIntegrationVoteTimeoutAndReconnect -v`
Expected: PASS.

- [ ] **Step 3: Финальный прогон всего репозитория с гонками**

Run: `go test -race ./...`
Expected: `ok engine`, `ok game`, `ok server`, `ok shuffle`.

- [ ] **Step 4: Commit**

```bash
git add server/integration_test.go
git commit -m "test(server): сквозняк — голосование по дедлайну доходит до обоих + реконнект/double-connect"
```

---

### Task 20: `docs/architecture.md` — ревизия D-2, новый OQ, журнал + заметка веб-агенту

**Files:**
- Modify: `docs/architecture.md`

> **Заметка для владельца → веб-агенту (НЕ трогаем `web/` в этом плане).** Задачи синхронизации
> веб-контракта (`web/src/contract/types.ts`), выявленные при сборке кодека Слоя 2 (§7.4 дизайна):
> 1. `Action`-union неполон — добавить `claimSubjective`, `vote`
>    (`{vote:'forShukh'|'againstShukh'}`), `declareOneCard`, `askCount`, `askAboutWest`, `discardWest`.
> 2. `GameSnapshot` без `legal` — Слой 1 `Update` его несёт; добавить `legal: Action[]`.
> 3. `View` без `vote` — добавить `vote?: { claimant, target, code, voted: SeatID[] }` (VoteView, §8.3).
> 4. `GameEvent`-union — добавить `voteOpened`/`voteResolved`.
> 5. Конверт `update` несёт `you`, `roomCode`, `stage`, `voteDeadline?` (мета Слоя 2) и опускает
>    `view` в лобби (клиент трактует отсутствие как `null`).

- [ ] **Step 1: Ревизовать D-2 (кука вместо токена в ссылке)**

В `docs/architecture.md`, §2, заменить строку D-2:

```markdown
| D-2 | **Точки входа — комната по коду/ссылке, без аккаунтов** (в MVP). Ввёл имя — играешь. Реконнект по токену в ссылке. | Быстрый путь к «поиграть с друзьями». Аккаунты/история — возможное будущее (см. дорожную карту). |
```

на:

```markdown
| D-2 | **Точки входа — комната по коду/ссылке, без аккаунтов** (в MVP). Ввёл имя — играешь. **Реконнект по HttpOnly-куке** (ревизия 2026-07-17, L2-6): инвайт-ссылка `/r/CODE` несёт только код (её шарят), личный токен места выдаётся HTTP-шагом `join` и живёт в HttpOnly-куке пути комнаты. | Быстрый путь к «поиграть с друзьями». Шаримая ссылка не должна содержать личное право на место; HttpOnly-токен не читаем из JS и не попадает в URL/логи. Аккаунты/история — возможное будущее. |
```

- [ ] **Step 2: Добавить OQ (расширение OQ-2) в §5**

В `docs/architecture.md`, §5, после строки OQ-3 добавить:

```markdown
- **OQ-4 (расширение OQ-2). Общий тайм-аут хода / брошенная партия.** Grace спасает
  только от обрывов; навсегда-ушедший игрок на своём ходу морозит стол (у движка нет
  фолда/авто-хода). Тайм-аут **голосования** R-8.6 решён в Слое 2; общий тайм-аут хода /
  авто-ход / фолд — будущая работа. → следующий спек.
```

- [ ] **Step 3: Добавить строки журнала в §7**

В `docs/architecture.md`, в конец §7 «Журнал изменений», добавить:

```markdown
- **2026-07-17.** Ревизовано D-2: реконнект по HttpOnly-куке (инвайт-ссылка несёт только
  код), взамен «токена в ссылке» (L2-6). Добавлен OQ-4 (расширение OQ-2): общий тайм-аут
  хода / брошенная партия.
- **2026-07-17.** Завершён Слой 2 Спеца 2: пакет `server/` (Hub + GC, Room + токены +
  таймеры, WS-соединение, кодек ↔ веб-контракт, `RoomStore`/`MemStore`, `Clock`-шов) +
  `cmd/shukh-server`. Тайм-аут голосования R-8.6 (`engine.CloseVote` по дедлайну), миграция
  хоста в лобби, реконнект/double-connect (close-and-replace). Задачи синхронизации
  веб-контракта переданы веб-агенту (§7.4 дизайна). Спец 2 закрыт.
```

- [ ] **Step 4: Прогон + Commit**

Run: `go build ./... && go test ./...`
Expected: всё зелёное.

```bash
git add docs/architecture.md
git commit -m "docs(architecture): Слой 2 — ревизия D-2 (кука), новый OQ, журнал"
```
