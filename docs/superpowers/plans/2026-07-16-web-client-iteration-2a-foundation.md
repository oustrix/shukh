# Веб-итерация 2a — Foundation (контракт + скриптованный транспорт + стор) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заложить данные-фундамент итерации 2: сверить контракт с реальным движком (`SeatView`), добавить легальность в снапшот, заменить emit-once мок на **скриптованный транспорт**, который проигрывает сценарий партии и реагирует на ход игрока.

**Architecture:** Слой 3 поверх каркаса итерации 1. Тот же шов `Transport`: скриптованный транспорт — двойник будущего `ws.ts` Спеца 2 (клиент — зритель пушей). Правила остаются в Go; клиент легальность не считает, а получает `legal` в снапшоте. Спека: `docs/superpowers/specs/2026-07-16-web-client-iteration-2-design.md`.

**Tech Stack:** существующий стек итерации 1 (Vite 6 + React 19 + TS strict + Vitest + zustand). Новых зависимостей в 2a нет (`motion` придёт в 2b).

## Global Constraints

- Все команды `npm` — из каталога `web/`. Гейт: `npm run typecheck && npm run lint && npm test`.
- TS-типы в `src/contract/types.ts` — **ручные зеркала** `engine/*.go` (W-3): сверять с реальными `engine/view.go` (`SeatView`), `engine/legal.go`, `engine/action.go`, `engine/event.go`. Комментарии-зеркала сохранять.
- Клиент **не вычисляет легальность** — только читает `snapshot.legal` (W2-2). Правила живут в Go.
- Шов `Transport` неизменен: `ui/`/`store/` от источника данных не зависят. Скриптованный транспорт реализует тот же интерфейс, что и мок.
- Детерминизм тестов: auto-шаги сценария проигрываются через **инъектируемый планировщик** (`Scheduler`); в тестах — синхронный.
- TDD: сначала падающий тест, потом минимальная реализация. Частые коммиты.
- Трейлер каждого коммита: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

```
web/src/
  contract/
    types.ts        # MODIFY: View→SeatView; +legal в GameSnapshot; +cardKey/isLegal/isCardPlayable/actionsEqual
    types.test.ts   # MODIFY: rename View→SeatView; +тесты хелперов
  transport/
    scripted.ts     # CREATE: Step/Scenario/Scheduler + createScriptedTransport
    scripted.test.ts# CREATE
  fixtures/
    game.ts         # MODIFY: +legal в gameSnapshot
    scenario.ts     # CREATE: demoScenario (раздача → заход → 2 боя ботов → Дама♥ закрывает кон)
  store/
    game.ts         # MODIFY: singleton → scripted transport; +selectLegal; events ring buffer (cap)
    game.test.ts    # MODIFY: +selectLegal, +events cap, +прогон сценария
```

Компоненты `ui/` в 2a **не меняются** — они уже читают `view`/`seats` из стора и отрендерят продвигающееся состояние. `App.test.tsx` не трогаем: сценарий отдаёт начальный снапшот **синхронно** при подписке (совместимо с текущими проверками).

---

### Task 1: Контракт — SeatView, legal, хелперы карт/действий

**Files:**
- Modify: `web/src/contract/types.ts`, `web/src/fixtures/game.ts` (добавить `legal` — иначе рефактор ломает тайпчек фикстуры)
- Test: `web/src/contract/types.test.ts`

**Interfaces:**
- Consumes: существующие `Card`, `Action`, `SeatID`.
- Produces:
  - `interface SeatView` (переименован из `View`; поля те же).
  - `GameSnapshot` с новым полем `legal: Action[]`.
  - `function isYourTurn(view: SeatView): boolean` (сигнатура обновлена под SeatView).
  - `function cardKey(card: Card): string` → `` `${card.rank}${card.suit}` ``.
  - `function actionsEqual(a: Action, b: Action): boolean`.
  - `function isLegal(legal: Action[], action: Action): boolean`.
  - `function isCardPlayable(legal: Action[], card: Card): boolean`.

- [ ] **Step 1: Обновить тест `web/src/contract/types.test.ts`**

Заменить содержимое на:
```ts
import {
  isYourTurn,
  cardKey,
  actionsEqual,
  isLegal,
  isCardPlayable,
  type SeatView,
  type Action,
} from './types'

const baseView: SeatView = {
  rules: { deckSize: 36, podkladkaSnizu: false, jokers: false },
  mode: 'middle',
  phase: 'playing',
  you: 0,
  turn: 0,
  hand: [],
  shukhPending: 0,
  opponents: [],
  table: [],
  discard: 0,
  talon: 0,
  live: { 0: true },
  finish: [],
}

test('isYourTurn: true когда turn === you', () => {
  expect(isYourTurn({ ...baseView, you: 0, turn: 0 })).toBe(true)
})

test('isYourTurn: false когда ходит другой', () => {
  expect(isYourTurn({ ...baseView, you: 0, turn: 1 })).toBe(false)
})

test('cardKey уникален по рангу+масти', () => {
  expect(cardKey({ suit: '♥', rank: 12 })).toBe('12♥')
  expect(cardKey({ suit: '♦', rank: 9 })).not.toBe(cardKey({ suit: '♠', rank: 9 }))
})

test('actionsEqual сравнивает по типу и полям (включая карту)', () => {
  expect(
    actionsEqual({ type: 'playCard', card: { suit: '♦', rank: 9 } }, { type: 'playCard', card: { suit: '♦', rank: 9 } }),
  ).toBe(true)
  expect(
    actionsEqual({ type: 'playCard', card: { suit: '♦', rank: 9 } }, { type: 'playCard', card: { suit: '♠', rank: 9 } }),
  ).toBe(false)
  expect(actionsEqual({ type: 'takeBottomAndPass' }, { type: 'takeBottomAndPass' })).toBe(true)
})

test('isLegal / isCardPlayable читают список легальных ходов', () => {
  const legal: Action[] = [
    { type: 'playCard', card: { suit: '♦', rank: 9 } },
    { type: 'takeBottomAndPass' },
  ]
  expect(isLegal(legal, { type: 'takeBottomAndPass' })).toBe(true)
  expect(isCardPlayable(legal, { suit: '♦', rank: 9 })).toBe(true)
  expect(isCardPlayable(legal, { suit: '♥', rank: 12 })).toBe(false)
})
```

- [ ] **Step 2: Прогнать тест — убедиться, что падает**

Run (из `web/`): `npx vitest run src/contract/types.test.ts`
Expected: FAIL — нет экспортов `SeatView`/`cardKey`/`actionsEqual`/`isLegal`/`isCardPlayable`.

- [ ] **Step 3: Изменить `web/src/contract/types.ts`**

1) Заменить блок комментария и `interface View` на `SeatView`:
```ts
// зеркало engine/view.go (SeatView, per-seat проекция, D-9) — синхронизировать вручную
export interface OpponentView {
  seat: SeatID
  handCount: number
  shukhPending: number
  live: boolean
}
export interface SeatView {
  rules: RuleSet
  mode: EnforcementMode
  phase: Phase
  you: SeatID
  turn: SeatID
  hand: Card[]
  shukhPending: number
  opponents: OpponentView[]
  table: TableCard[]
  discard: number
  talon: number
  live: Record<number, boolean>
  finish: SeatID[]
}
```

2) Добавить `legal` в `GameSnapshot`:
```ts
export interface GameSnapshot {
  roomCode: string
  seats: SeatMeta[]
  view: SeatView | null // null в лобби (партия ещё не началась)
  legal: Action[] // легальные ходы текущего игрока (зеркало LegalActions); [] когда не наш ход
}
```

3) Заменить хелпер `isYourTurn` и добавить новые (в конце файла):
```ts
// Хелперы уровня контракта (используются UI и транспортом).
export function isYourTurn(view: SeatView): boolean {
  return view.turn === view.you
}

// Стабильный ключ карты: в колоде (36/52) карты уникальны по рангу+масти.
export function cardKey(card: Card): string {
  return `${card.rank}${card.suit}`
}

// Каноничный ключ действия — для сравнения (легальность, сверка со сценарием).
function actionKey(a: Action): string {
  switch (a.type) {
    case 'playCard':
      return `playCard:${cardKey(a.card)}`
    case 'giveShukhCard':
      return `giveShukhCard:${cardKey(a.card)}`
    case 'claimShukh':
      return `claimShukh:${a.target}:${a.code}`
    case 'takeShukhCards':
      return `takeShukhCards:${a.seat}`
    default:
      return a.type // takeBottomAndPass | podkladkaWest
  }
}

export function actionsEqual(a: Action, b: Action): boolean {
  return actionKey(a) === actionKey(b)
}

export function isLegal(legal: Action[], action: Action): boolean {
  return legal.some((a) => actionsEqual(a, action))
}

export function isCardPlayable(legal: Action[], card: Card): boolean {
  return isLegal(legal, { type: 'playCard', card })
}
```

4) Добавить `legal` в `web/src/fixtures/game.ts` (иначе новый обязательный `GameSnapshot.legal` роняет тайпчек фикстуры). В объект `gameSnapshot`, на верхнем уровне снапшота (снаружи `view`, рядом с ним), добавить:
```ts
  legal: [
    { type: 'playCard', card: { suit: '♥', rank: 12 } }, // Дама♥ бьёт что угодно (R-3.7)
    { type: 'takeBottomAndPass' },
  ],
```
(Иллюстративный набор для юнит-тестов фикстуры; точная per-step легальность живёт в сценарии, Task 2.)

- [ ] **Step 4: Прогнать тест — PASS**

Run (из `web/`): `npx vitest run src/contract/types.test.ts`
Expected: PASS.

- [ ] **Step 5: Полный гейт (задача самодостаточна и зелёная)**

Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное. Если typecheck падает на имени `View` где-то ещё (кроме `types.test.ts`, уже переписанного) — заменить импорт на `SeatView` в том файле и дописать в отчёт.

- [ ] **Step 6: Commit**

```bash
git add web/src/contract
git commit -m "feat(web): contract sync — SeatView + legal in snapshot + card/action helpers

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Скриптованный транспорт + демо-сценарий

**Files:**
- Create: `web/src/transport/scripted.ts`, `web/src/fixtures/scenario.ts`
- Test: `web/src/transport/scripted.test.ts`

(Фикстура `game.ts` уже получила `legal` в Task 1.)

**Interfaces:**
- Consumes: `Transport` (contract), `Action`/`GameEvent`/`GameSnapshot`/`actionsEqual` (Task 1).
- Produces:
  - `interface Step { kind: 'auto' | 'await'; expect?: Action; events: GameEvent[]; snapshot: GameSnapshot; delayMs?: number }`
  - `type Scenario = Step[]`
  - `type Scheduler = (fn: () => void, ms: number) => void`
  - `function createScriptedTransport(scenario: Scenario, schedule?: Scheduler): Transport`
  - `const demoScenario: Scenario` (в `fixtures/scenario.ts`)

- [ ] **Step 1: Написать падающий тест `web/src/transport/scripted.test.ts`**

```ts
import { createScriptedTransport, type Scenario, type Scheduler } from './scripted'
import type { GameSnapshot, GameEvent } from '../contract/types'

function snap(hand: number): GameSnapshot {
  return {
    roomCode: 'T',
    seats: [{ seat: 0, name: 'p', ready: true }],
    view: {
      rules: { deckSize: 36, podkladkaSnizu: false, jokers: false },
      mode: 'middle',
      phase: 'playing',
      you: 0,
      turn: 0,
      hand: Array.from({ length: hand }, () => ({ suit: '♦', rank: 9 })),
      shukhPending: 0,
      opponents: [],
      table: [],
      discard: 0,
      talon: 0,
      live: { 0: true },
      finish: [],
    },
    legal: [],
  }
}

// синхронный планировщик — детерминизм в тестах
const sync: Scheduler = (fn) => fn()

const scenario: Scenario = [
  { kind: 'auto', events: [{ type: 'gameStarted', turn: 0 }], snapshot: snap(2) },
  {
    kind: 'await',
    expect: { type: 'playCard', card: { suit: '♦', rank: 9 } },
    events: [{ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: 9 } }],
    snapshot: snap(1),
  },
  { kind: 'auto', events: [{ type: 'cardPlayed', seat: 1, card: { suit: '♦', rank: 10 } }], snapshot: snap(1) },
]

test('subscribe синхронно отдаёт начальный (auto) снапшот и его события', () => {
  const t = createScriptedTransport(scenario, sync)
  const snaps: GameSnapshot[] = []
  const evs: GameEvent[] = []
  t.subscribe((s) => snaps.push(s), (e) => evs.push(e))
  expect(evs).toEqual([{ type: 'gameStarted', turn: 0 }])
  expect(snaps[0].view?.hand.length).toBe(2)
  // остановились на await — авто-шаг после него ещё не проигран
  expect(snaps.length).toBe(1)
})

test('ожидаемое действие продвигает таймлайн и тянет следующий auto-шаг', () => {
  const t = createScriptedTransport(scenario, sync)
  const snaps: GameSnapshot[] = []
  const evs: GameEvent[] = []
  t.subscribe((s) => snaps.push(s), (e) => evs.push(e))
  t.send({ type: 'playCard', card: { suit: '♦', rank: 9 } })
  // await-шаг проигран (hand→1) + следующий auto (бой Бори) проигран синхронно
  expect(snaps.map((s) => s.view?.hand.length)).toEqual([2, 1, 1])
  expect(evs.map((e) => e.type)).toEqual(['gameStarted', 'cardPlayed', 'cardPlayed'])
})

test('офф-скрипт действие игнорируется (таймлайн не двигается)', () => {
  const t = createScriptedTransport(scenario, sync)
  const snaps: GameSnapshot[] = []
  t.subscribe((s) => snaps.push(s), () => {})
  t.send({ type: 'takeBottomAndPass' }) // не то, что ждёт await
  expect(snaps.length).toBe(1) // ничего не продвинулось
})
```

- [ ] **Step 2: Прогнать — FAIL (нет модуля). Создать `web/src/transport/scripted.ts`**

```ts
import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { actionsEqual } from '../contract/types'

// Шаг сценария. 'auto' проигрывается сам (по таймеру), 'await' ждёт ожидаемое
// действие игрока. snapshot — состояние ПОСЛЕ шага; events эмитятся перед снапшотом.
export interface Step {
  kind: 'auto' | 'await'
  expect?: Action
  events: GameEvent[]
  snapshot: GameSnapshot
  delayMs?: number
}
export type Scenario = Step[]

// Планировщик auto-шагов — инъектируется в тестах (синхронный) ради детерминизма.
export type Scheduler = (fn: () => void, ms: number) => void
const defaultScheduler: Scheduler = (fn, ms) => {
  setTimeout(fn, ms)
}

// Скриптованный транспорт — двойник будущего ws.ts. Начальный (нулевой) auto-шаг
// эмитится СИНХРОННО при подписке (сервер сразу отдаёт стартовое состояние);
// последующие auto-шаги идут через планировщик. Предусловие: scenario[0].kind === 'auto'.
export function createScriptedTransport(scenario: Scenario, schedule: Scheduler = defaultScheduler): Transport {
  let index = 0
  let onSnapshot: ((s: GameSnapshot) => void) | null = null
  let onEvent: ((e: GameEvent) => void) | null = null

  function emit(step: Step) {
    step.events.forEach((e) => onEvent?.(e))
    onSnapshot?.(step.snapshot)
  }

  // Проигрывает подряд идущие auto-шаги через планировщик, пока не упрётся в await/конец.
  function scheduleAutos() {
    const step = scenario[index]
    if (!step || step.kind !== 'auto') return
    schedule(() => {
      index += 1
      emit(step)
      scheduleAutos()
    }, step.delayMs ?? 0)
  }

  return {
    subscribe(snap, ev) {
      onSnapshot = snap
      onEvent = ev
      // стартовое состояние — синхронно
      const first = scenario[index]
      if (first && first.kind === 'auto') {
        index += 1
        emit(first)
      }
      scheduleAutos()
      return () => {
        onSnapshot = null
        onEvent = null
      }
    },
    send(action) {
      const step = scenario[index]
      if (!step || step.kind !== 'await') return // не наш момент
      if (step.expect && !actionsEqual(step.expect, action)) return // офф-скрипт
      index += 1
      emit(step)
      scheduleAutos()
    },
  }
}
```

- [ ] **Step 3: Прогнать — PASS**

Run (из `web/`): `npx vitest run src/transport/scripted.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 4: Создать демо-сценарий `web/src/fixtures/scenario.ts`**

Партия на 3 игрока (you=0 Аня, 1 Боря, 2 Вера), Deck36: раздача → заход 9♦ → Боря бьёт 10♦ → Вера бьёт J♦ → ваш бой Дамой♥ закрывает кон и сметает его. Легальность в каждом снапшоте отражает `LegalActions` для текущего игрока.

```ts
import type { Card, GameSnapshot, SeatView } from '../contract/types'
import type { Scenario } from '../transport/scripted'

const c = (rank: number, suit: Card['suit']): Card => ({ rank, suit })
const SEATS = [
  { seat: 0, name: 'Аня', ready: true },
  { seat: 1, name: 'Боря', ready: true },
  { seat: 2, name: 'Вера', ready: true },
]

// база снапшота: подставляем изменяющиеся поля
function base(over: Partial<SeatView>, legal: GameSnapshot['legal']): GameSnapshot {
  return {
    roomCode: 'DEMO',
    seats: SEATS,
    view: {
      rules: { deckSize: 36, podkladkaSnizu: false, jokers: false },
      mode: 'middle',
      phase: 'playing',
      you: 0,
      turn: 0,
      hand: [],
      shukhPending: 0,
      opponents: [
        { seat: 1, handCount: 5, shukhPending: 0, live: true },
        { seat: 2, handCount: 5, shukhPending: 0, live: true },
      ],
      table: [],
      discard: 0,
      talon: 0,
      live: { 0: true, 1: true, 2: true },
      finish: [],
      ...over,
    },
    legal,
  }
}

const HAND0 = [c(9, '♦'), c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')] // 9♦ Дама♥ 6♠ A♣ 7♦

export const demoScenario: Scenario = [
  // 0. Раздача, ваш заход. Легально всё, кроме Дамы♥ (R-5.2).
  {
    kind: 'auto',
    events: [{ type: 'gameStarted', turn: 0 }],
    snapshot: base({ hand: HAND0, turn: 0 }, [
      { type: 'playCard', card: c(9, '♦') },
      { type: 'playCard', card: c(6, '♠') },
      { type: 'playCard', card: c(14, '♣') },
      { type: 'playCard', card: c(7, '♦') },
    ]),
  },
  // 1. Вы зашли 9♦.
  {
    kind: 'await',
    expect: { type: 'playCard', card: c(9, '♦') },
    events: [{ type: 'cardPlayed', seat: 0, card: c(9, '♦') }],
    snapshot: base(
      {
        hand: [c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [{ card: c(9, '♦'), by: 0 }],
        turn: 1,
        opponents: [
          { seat: 1, handCount: 5, shukhPending: 0, live: true },
          { seat: 2, handCount: 5, shukhPending: 0, live: true },
        ],
      },
      [], // не ваш ход
    ),
  },
  // 2. Боря бьёт 10♦ (auto).
  {
    kind: 'auto',
    delayMs: 800,
    events: [{ type: 'cardPlayed', seat: 1, card: c(10, '♦') }],
    snapshot: base(
      {
        hand: [c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [
          { card: c(9, '♦'), by: 0 },
          { card: c(10, '♦'), by: 1 },
        ],
        turn: 2,
        opponents: [
          { seat: 1, handCount: 4, shukhPending: 0, live: true },
          { seat: 2, handCount: 5, shukhPending: 0, live: true },
        ],
      },
      [],
    ),
  },
  // 3. Вера бьёт J♦ (auto). Ход возвращается к вам: бить Дамой♥ или взять низ.
  {
    kind: 'auto',
    delayMs: 800,
    events: [{ type: 'cardPlayed', seat: 2, card: c(11, '♦') }],
    snapshot: base(
      {
        hand: [c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [
          { card: c(9, '♦'), by: 0 },
          { card: c(10, '♦'), by: 1 },
          { card: c(11, '♦'), by: 2 },
        ],
        turn: 0,
        opponents: [
          { seat: 1, handCount: 4, shukhPending: 0, live: true },
          { seat: 2, handCount: 4, shukhPending: 0, live: true },
        ],
      },
      // Дама♥ бьёт что угодно (R-3.7); плюс всегда можно взять низ (R-5.3b).
      [{ type: 'playCard', card: c(12, '♥') }, { type: 'takeBottomAndPass' }],
    ),
  },
  // 4. Вы бьёте Дамой♥ — кон закрывается (R-3.7.1) и сметается; вы открываете следующий.
  {
    kind: 'await',
    expect: { type: 'playCard', card: c(12, '♥') },
    events: [
      { type: 'cardPlayed', seat: 0, card: c(12, '♥') },
      { type: 'conClosed', by: 0 },
      { type: 'conSwept', cards: [c(9, '♦'), c(10, '♦'), c(11, '♦'), c(12, '♥')] },
    ],
    snapshot: base(
      {
        hand: [c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [],
        discard: 4,
        turn: 0,
        opponents: [
          { seat: 1, handCount: 4, shukhPending: 0, live: true },
          { seat: 2, handCount: 4, shukhPending: 0, live: true },
        ],
      },
      // Новый заход: всё, кроме Дамы♥ (её уже нет). Все три карты легальны.
      [
        { type: 'playCard', card: c(6, '♠') },
        { type: 'playCard', card: c(14, '♣') },
        { type: 'playCard', card: c(7, '♦') },
      ],
    ),
  },
]
```

- [ ] **Step 5: Гейт**

Run (из `web/`): `npm run typecheck && npm run lint && npx vitest run src/transport src/fixtures src/contract`
Expected: зелёно (фикстура уже несёт `legal` из Task 1).

- [ ] **Step 6: Commit**

```bash
git add web/src/transport/scripted.ts web/src/transport/scripted.test.ts web/src/fixtures/scenario.ts
git commit -m "feat(web): scripted transport + demo scenario (server stand-in)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Стор — подключение сценария, selectLegal, кольцевой буфер событий

**Files:**
- Modify: `web/src/store/game.ts`
- Test: `web/src/store/game.test.ts`

**Interfaces:**
- Consumes: `createScriptedTransport`/`demoScenario` (Task 2), `Transport`, `Action`/`GameEvent`/`GameSnapshot` (Task 1).
- Produces:
  - `const EVENTS_CAP = 100` (внутренняя константа).
  - `selectLegal = (s: GameState) => s.snapshot?.legal ?? []`.
  - Singleton `useGameStore` теперь поверх `createScriptedTransport(demoScenario)`.
  - `createGameStore` копит события с ограничением длины (последние `EVENTS_CAP`).

- [ ] **Step 1: Обновить тест `web/src/store/game.test.ts`**

Добавить импорты и три теста (существующие оставить):
```ts
import { createGameStore, selectLegal, EVENTS_CAP } from './game'
import { createScriptedTransport } from '../transport/scripted'
```
Новые тесты:
```ts
test('selectLegal возвращает snapshot.legal', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  f.emitSnapshot({ ...gameSnapshot }) // gameSnapshot теперь несёт legal
  expect(selectLegal(store.getState())).toEqual(gameSnapshot.legal)
})

test('events ограничены EVENTS_CAP (кольцевой буфер)', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  for (let i = 0; i < EVENTS_CAP + 25; i++) {
    f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: 9 } })
  }
  expect(store.getState().events).toHaveLength(EVENTS_CAP)
})

test('стор проходит сценарий: play продвигает снапшот (синхронный планировщик)', () => {
  const store = createGameStore(createScriptedTransport(
    [
      { kind: 'auto', events: [], snapshot: { ...gameSnapshot } },
      {
        kind: 'await',
        expect: { type: 'takeBottomAndPass' },
        events: [{ type: 'cardsTaken', seat: 0, cards: [] }],
        snapshot: { ...gameSnapshot, roomCode: 'AFTER' },
      },
    ],
    (fn) => fn(),
  ))
  expect(store.getState().snapshot?.roomCode).toBe('DEMO')
  store.getState().play({ type: 'takeBottomAndPass' })
  expect(store.getState().snapshot?.roomCode).toBe('AFTER')
})
```
(`gameSnapshot` уже импортирован в этом тест-файле из Task 1 итерации-1; если нет — добавить `import { gameSnapshot } from '../fixtures/game'`.)

- [ ] **Step 2: Прогнать — FAIL (нет `selectLegal`/`EVENTS_CAP`)**

Run (из `web/`): `npx vitest run src/store/game.test.ts`
Expected: FAIL.

- [ ] **Step 3: Изменить `web/src/store/game.ts`**

```ts
import { create } from 'zustand'
import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { createScriptedTransport } from '../transport/scripted'
import { demoScenario } from '../fixtures/scenario'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  play: (action: Action) => void
}

// Предел лога событий — событий за партию много; держим только последние.
export const EVENTS_CAP = 100

// Общие селекторы — чтобы компоненты не дублировали разбор snapshot.
export const selectSeats = (s: GameState) => s.snapshot?.seats ?? []
export const selectView = (s: GameState) => s.snapshot?.view ?? null
export const selectLegal = (s: GameState) => s.snapshot?.legal ?? []

// Создаёт изолированный стор поверх переданного транспорта. Подписка — ПОСЛЕ
// создания стора: транспорт пушит в уже готовый setState.
export function createGameStore(transport: Transport) {
  const store = create<GameState>(() => ({
    snapshot: null,
    events: [],
    play: (action) => transport.send(action),
  }))
  transport.subscribe(
    (snapshot) => store.setState({ snapshot }),
    (event) => store.setState((s) => ({ events: [...s.events, event].slice(-EVENTS_CAP) })),
  )
  return store
}

// Singleton приложения: скриптованный сценарий (двойник ws.ts Спеца 2). Замена на
// transport/ws.ts не затрагивает компоненты — они читают только этот хук.
export const useGameStore = createGameStore(createScriptedTransport(demoScenario))
```

- [ ] **Step 4: Прогнать — PASS**

Run (из `web/`): `npx vitest run src/store/game.test.ts`
Expected: PASS.

- [ ] **Step 5: Полный гейт + сборка + ручная проверка**

Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное (в т.ч. `App.test.tsx` — начальный снапшот сценария синхронен и совместим: seats Аня/Боря/Вера, рука ≥5 карт на стартовом шаге).
Run (из `web/`): `npm run build` — компилируется.
Run (из `web/`): `npm run dev` → открыть; на столе: раздача → заход 9♦ (клик по карте отправляет `playCard` — сценарий ждёт именно 9♦), затем боты бьют 10♦/J♦ с паузами, ваш бой Дамой♥ закрывает и сметает кон. Остановить сервер.

Примечание: полноценная подсветка легальных ходов и выбор+подтверждение — план 2c; здесь клик по карте отправляет `playCard` напрямую (как в итерации 1), сценарий продвигается только на ожидаемый ход.

- [ ] **Step 6: Commit**

```bash
git add web/src/store
git commit -m "feat(web): store on scripted transport — selectLegal + events ring buffer

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Итог 2a

После Task 3: `npm run dev` проигрывает связный скриптованный сценарий партии — стол продвигается на ход игрока и на паузах ботов, легальность едет в снапшоте. Шов `Transport` неизменен (готов под `ws.ts`). Дальше: **2b** (motion + стабильные ключи + анимации переходов), затем **2c** (подсветка легальности + выбор/подтверждение + ритуал ШУХа + голосование).
