# Веб-клиент «Шух» — итерация 2c (играбельный стол + ритуал ШУХа) — план

> **For agentic workers:** REQUIRED SUB-SKILL: используй superpowers:subagent-driven-development
> (рекомендуется) или superpowers:executing-plans для исполнения плана задача-за-задачей.
> Шаги размечены чекбоксами (`- [ ]`).

**Goal:** Довести стол до играбельного состояния — подсветка легальных ходов из снапшота,
выбор+подтверждение карты (по `cardKey`, без «фантомного подъёма»), A11y-клавиатура — и собрать
ритуал ШУХа: ШУХ-зоны у мест, живые кнопки «ШУХ!»/«Одна карта!», модалка голосования (R-8.6,
скриптованные голоса), анимация оплаты ШУХа. Демо-сценарий расширяется rules-корректным ШУХ-окном.

**Architecture:** Клиент остаётся **зрителем пушей** (W2-1): всё интерактивное гейтится списком
`snapshot.legal` (W2-2) — клиент правил НЕ считает. Скриптованный транспорт (двойник будущего
`ws.ts`) продвигает таймлайн по ожидаемому действию. Голосование по ШУХу и его исход — данные
снапшота (`GameSnapshot.shukhVote`), сгенерированные сценарием (W2-7, кворум не считаем). Анимации —
на `motion`, reduced-motion уже включён глобально (`<MotionConfig reducedMotion="user">`).

**Tech Stack:** Vite + React 19 + TypeScript (strict) + Vitest + @testing-library/react +
zustand + motion. Всё уже установлено — **новых зависимостей в 2c нет**.

## Global Constraints

- **Гейт (из `web/`), обязателен в конце КАЖДОЙ задачи:** `npm run typecheck && npm run lint && npm test` — всё зелёное.
- **Клиент НЕ считает правила (W2-2).** Любая интерактивность (какая карта кликабельна, доступна ли «ШУХ!»/«Взять низ»/взятие ШУХ-зоны) выводится ТОЛЬКО из `snapshot.legal` через хелперы `isCardPlayable`/`isLegal`. Никакой игровой логики в UI.
- **Ручной миррор движка (W-3).** `SeatView`, `Action`, `GameEvent` — зеркала `engine/*.go`; НЕ добавляй в них полей, которых нет в движке. `ShukhVote` и поле `shukhVote` кладутся на `GameSnapshot` (клиент/сервер-DTO Спеца 2), НЕ на `SeatView`.
- **Стабильный ключ карты (W2-4).** React-ключ/`layoutId`/выбор — по `cardKey(card)` (`rank+suit`), НИКОГДА по индексу.
- **reduced-motion** уже обеспечен глобально в `web/src/main.tsx`; по-компонентно ничего делать не нужно, но CSS-анимации (`pulse`) обязаны иметь `@media (prefers-reduced-motion: reduce)`-выключение.
- **Демо-сценарий обязан быть rules-корректным** — сверяй каждый шаг с `docs/shukh-rules.md`, цитируя `R-`/`Ш-`/`I-` в комментариях.
- **Коммиты** заканчиваются трейлером:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- **Каждая задача — свежий сабагент**, свой цикл тестов, заканчивается зелёным гейтом; леджер `.superpowers/sdd/progress.md` (свежий под 2c).

---

## Обзор структуры файлов

**Контракт (`web/src/contract/`):**
- `types.ts` — добавить `ShukhVote`, поле `GameSnapshot.shukhVote?`, хелперы `claimShukhInLegal`, `isShukhTakeable`.
- `types.test.ts` — ветки `actionsEqual` (claimShukh/giveShukhCard/takeShukhCards), `isShukhTakeable`.

**Транспорт (`web/src/transport/`):**
- `scripted.ts` — `Step` → дискриминированное объединение `AutoStep | AwaitStep` (`expect` обязателен на `await`).

**Стор (`web/src/store/`):**
- `game.ts` — селектор `selectShukhVote`.
- `game.test.ts` — assert «events keep-last» (буфер хранит ПОСЛЕДНИЕ, не первые).

**UI (`web/src/ui/table/`):**
- `Card.tsx` (+ `Card.module.css`) — проп `dimmed`, `onKeyDown` (Enter/Space), видимый фокус.
- `Hand.tsx` — пропы `selectedKey`/`playableKeys`/`onSelect(card)` вместо позиционных.
- `ActionBar.tsx` — «Сходить»(confirm), «Взять низ», «ШУХ!», «Одна карта!» — состояния по легальности/owes.
- `ShukhZone.tsx` (+ тест) — **новый**: отложенная стопка (счётчик, рубашкой), кликабельна при takeable.
- `ShukhVoteModal.tsx` (+ тест) — **новый**: модалка R-8.6 (цель/код/голоса/исход).
- `OpponentSeat.tsx` (+ тест) — рендерить `ShukhZone` вместо `shukhBadge`.
- `Table.module.css` — стили выбранной/подтверждаемой карты, ШУХ-зоны, модалки, pulse, фокуса.

**Экран (`web/src/ui/screens/`):**
- `Table.tsx` — интегратор: `selectedKey`-стейт (фикс фантомного подъёма), playableKeys из legal, confirm-on-second-click, ШУХ-зона, модалка, «ШУХ!»/«Одна карта!».

**Фикстуры (`web/src/fixtures/`):**
- `scenario.ts` — расширить ШУХ-окном + голосованием + оплатой (rules-корректно).
- `game.ts` — добавить `shukhVote: null` (совместимость типа), при необходимости — снапшот с своей ШУХ-зоной для теста.

---

## Task 1: Контракт и транспорт — фундамент 2c

**Files:**
- Modify: `web/src/contract/types.ts`
- Modify: `web/src/contract/types.test.ts`
- Modify: `web/src/transport/scripted.ts`
- Modify: `web/src/store/game.ts`
- Modify: `web/src/store/game.test.ts`
- Modify: `web/src/fixtures/game.ts`

**Interfaces:**
- Produces:
  - `interface ShukhVote { claimant: SeatID; target: SeatID; code: ShukhCode; votes: { seat: SeatID; up: boolean }[]; outcome: 'upheld' | 'overturned'; resolved: boolean }`
  - `GameSnapshot.shukhVote?: ShukhVote | null`
  - `claimShukhInLegal(legal: Action[]): Extract<Action, { type: 'claimShukh' }> | undefined`
  - `isShukhTakeable(legal: Action[], seat: SeatID): boolean`
  - `selectShukhVote(s: GameState): ShukhVote | null`
  - `Step` теперь `AutoStep | AwaitStep` (`AwaitStep.expect: Action` — обязателен)

- [ ] **Step 1: Тесты веток `actionsEqual` и `isShukhTakeable` (падают)**

В `web/src/contract/types.test.ts` добавить в конец. Обнови импорт: добавь `isShukhTakeable` к списку из `./types`.

```ts
test('actionsEqual: claimShukh сравнивает target и code', () => {
  expect(
    actionsEqual(
      { type: 'claimShukh', target: 1, code: 2 },
      { type: 'claimShukh', target: 1, code: 2 },
    ),
  ).toBe(true)
  expect(
    actionsEqual(
      { type: 'claimShukh', target: 1, code: 2 },
      { type: 'claimShukh', target: 1, code: 11 },
    ),
  ).toBe(false)
})

test('actionsEqual: giveShukhCard сравнивает карту, takeShukhCards — место', () => {
  expect(
    actionsEqual(
      { type: 'giveShukhCard', card: { suit: '♣', rank: 5 } },
      { type: 'giveShukhCard', card: { suit: '♣', rank: 5 } },
    ),
  ).toBe(true)
  expect(
    actionsEqual({ type: 'takeShukhCards', seat: 0 }, { type: 'takeShukhCards', seat: 0 }),
  ).toBe(true)
  expect(
    actionsEqual({ type: 'takeShukhCards', seat: 0 }, { type: 'takeShukhCards', seat: 1 }),
  ).toBe(false)
})

test('isShukhTakeable: true когда takeShukhCards на своё место легально', () => {
  const legal: Action[] = [{ type: 'takeShukhCards', seat: 0 }]
  expect(isShukhTakeable(legal, 0)).toBe(true)
  expect(isShukhTakeable(legal, 1)).toBe(false)
  expect(isShukhTakeable([], 0)).toBe(false)
})
```

- [ ] **Step 2: Прогнать — падает на отсутствии `isShukhTakeable`**

Run: `cd web && npx vitest run src/contract/types.test.ts`
Expected: FAIL (`isShukhTakeable is not a function` / import error).

- [ ] **Step 3: Добавить `ShukhVote`, поле снапшота и хелперы в `types.ts`**

В `web/src/contract/types.ts` после `GameSnapshot` (после строки с `legal: Action[]`) добавить поле в интерфейс — измени интерфейс `GameSnapshot` так:

```ts
export interface GameSnapshot {
  roomCode: string
  seats: SeatMeta[]
  view: SeatView | null // null в лобби (партия ещё не началась)
  legal: Action[] // легальные ходы текущего игрока (зеркало LegalActions); [] когда не наш ход
  shukhVote?: ShukhVote | null // активное голосование по ШУХу (R-8.6); скриптовано (W2-7)
}

// Голосование/оспаривание ШУХа (R-8.6). Это клиент/сервер-DTO Спеца 2, НЕ engine.SeatView:
// голоса и исход присылает сервер; на моке — сценарий (кворум по-настоящему не считаем, W2-7).
export interface ShukhVote {
  claimant: SeatID // кто предъявил ШУХ
  target: SeatID // на кого
  code: ShukhCode
  votes: { seat: SeatID; up: boolean }[] // голоса судящих (R-8.9); скриптованы
  outcome: 'upheld' | 'overturned' // overturned → Ш-8 предъявившему
  resolved: boolean // false — идёт голосование; true — показать исход
}
```

В конец файла (после `isCardPlayable`) добавить хелперы:

```ts
// Первый claimShukh в списке легальных (открыто ли ШУХ-окно). Клиент не судит —
// сервер кладёт конкретный предъявляемый ШУХ в legal, кнопка лишь его отправляет.
export function claimShukhInLegal(
  legal: Action[],
): Extract<Action, { type: 'claimShukh' }> | undefined {
  return legal.find((a): a is Extract<Action, { type: 'claimShukh' }> => a.type === 'claimShukh')
}

// Можно ли забрать свои отложенные ШУХ-карты (R-8.3 — только по завершении кона).
// Гейтится legal: сервер добавляет takeShukhCards, когда взятие законно.
export function isShukhTakeable(legal: Action[], seat: SeatID): boolean {
  return isLegal(legal, { type: 'takeShukhCards', seat })
}
```

- [ ] **Step 4: Дискриминированное объединение `Step` в `scripted.ts`**

В `web/src/transport/scripted.ts` заменить блок объявления `Step`/`Scenario` (строки с `export interface Step { … }` и `export type Scenario`) на:

```ts
// Шаг сценария. 'auto' проигрывается сам (по таймеру), 'await' ждёт ожидаемое
// действие игрока. snapshot — состояние ПОСЛЕ шага; events эмитятся перед снапшотом.
interface StepBase {
  events: GameEvent[]
  snapshot: GameSnapshot
}
export interface AutoStep extends StepBase {
  kind: 'auto'
  delayMs?: number // пауза перед проигрыванием
}
export interface AwaitStep extends StepBase {
  kind: 'await'
  expect: Action // ожидаемое действие игрока — обязателен на await-шаге
}
export type Step = AutoStep | AwaitStep
export type Scenario = Step[]
```

В `send(action)` упростить проверку (теперь `expect` гарантирован сужением типа) — заменить тело:

```ts
    send(action) {
      const step = scenario[index]
      if (!step || step.kind !== 'await') return // не наш момент
      if (!actionsEqual(step.expect, action)) return // офф-скрипт
      index += 1
      emit(step)
      scheduleAutos()
    },
```

- [ ] **Step 5: Селектор `selectShukhVote` в `game.ts`**

В `web/src/store/game.ts` рядом с `selectLegal` добавить:

```ts
export const selectShukhVote = (s: GameState) => s.snapshot?.shukhVote ?? null
```

- [ ] **Step 6: Assert «events keep-last» в `game.test.ts`**

В `web/src/store/game.test.ts` добавить (рядом с тестом про EVENTS_CAP):

```ts
test('events keep-last: буфер хранит ПОСЛЕДНИЕ события, не первые', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  const N = EVENTS_CAP + 5
  for (let i = 0; i < N; i++) {
    f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: i } })
  }
  const evs = store.getState().events as Extract<GameEvent, { type: 'cardPlayed' }>[]
  expect(evs).toHaveLength(EVENTS_CAP)
  expect(evs[EVENTS_CAP - 1].card.rank).toBe(N - 1) // самое свежее — последнее
  expect(evs[0].card.rank).toBe(N - EVENTS_CAP) // первые 5 вытеснены
})
```

- [ ] **Step 7: `shukhVote: null` в фикстуре `game.ts`**

В `web/src/fixtures/game.ts` в объект `gameSnapshot` добавить поле сразу после массива `legal`:

```ts
  shukhVote: null,
```

- [ ] **Step 8: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS (все тесты зелёные; новые ветки `actionsEqual`/`isShukhTakeable`/keep-last проходят).

- [ ] **Step 9: Commit**

```bash
git add web/src/contract web/src/transport/scripted.ts web/src/store web/src/fixtures/game.ts
git commit -m "feat(web): 2c contract — ShukhVote DTO, claim/take helpers, Step union, keep-last assert

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Подсветка легальных ходов + выбор по `cardKey` (фикс фантомного подъёма)

**Files:**
- Modify: `web/src/ui/table/Card.tsx`
- Modify: `web/src/ui/table/Card.module.css`
- Modify: `web/src/ui/table/Hand.tsx`
- Modify: `web/src/ui/table/Hand.test.tsx`
- Modify: `web/src/ui/table/animation.test.tsx`
- Modify: `web/src/ui/screens/Table.tsx`

**Interfaces:**
- Consumes (Task 1): `cardKey`, `isCardPlayable`, `selectLegal`.
- Produces:
  - `Card` props: `+ dimmed?: boolean`
  - `Hand` props: `{ cards: Card[]; selectedKey: string | null; playableKeys: Set<string>; onSelect: (card: Card) => void }`
  - `Table` держит `selectedKey: string | null` (позиционный `selected: number` уходит).

- [ ] **Step 1: Тесты Hand — подсветка/дим по playableKeys, выбор по карте (падают)**

Заменить содержимое `web/src/ui/table/Hand.test.tsx` на:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Hand } from './Hand'
import { cardKey, type Card } from '../../contract/types'

const cards: Card[] = [
  { suit: '♦', rank: 9 },
  { suit: '♥', rank: 12 },
]
const playable = new Set([cardKey(cards[0])]) // легальна только 9♦

test('Hand рендерит по карте на каждую в руке', () => {
  render(<Hand cards={cards} selectedKey={null} playableKeys={playable} onSelect={() => {}} />)
  expect(screen.getAllByTestId('card-face')).toHaveLength(2)
})

test('клик по легальной карте зовёт onSelect с этой картой', async () => {
  const onSelect = vi.fn()
  render(<Hand cards={cards} selectedKey={null} playableKeys={playable} onSelect={onSelect} />)
  await userEvent.click(screen.getByRole('button', { name: '9♦' }))
  expect(onSelect).toHaveBeenCalledWith(cards[0])
})

test('нелегальная карта некликабельна (нет роли button)', () => {
  render(<Hand cards={cards} selectedKey={null} playableKeys={playable} onSelect={() => {}} />)
  // 9♦ — button (легальна); Дама♥ — img (нелегальна, дим)
  expect(screen.getByRole('button', { name: '9♦' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '12♥' })).toBeNull()
})
```

- [ ] **Step 2: Прогнать — падает (старые пропы Hand)**

Run: `cd web && npx vitest run src/ui/table/Hand.test.tsx`
Expected: FAIL (type/props mismatch, `playableKeys` неизвестен).

- [ ] **Step 3: Проп `dimmed` в `Card.tsx`**

Заменить `web/src/ui/table/Card.tsx` на:

```tsx
import { motion } from 'motion/react'
import { cardKey, type Card as CardT } from '../../contract/types'
import { cx } from '../kit/cx'
import { rankLabel, isRedSuit, cardLabel } from './cardText'
import styles from './Card.module.css'

interface CardProps {
  card?: CardT
  faceDown?: boolean
  selected?: boolean
  dimmed?: boolean
  onClick?: () => void
}

export function Card({ card, faceDown, selected, dimmed, onClick }: CardProps) {
  const hidden = faceDown || !card
  const red = card ? isRedSuit(card.suit) : false
  const interactive = Boolean(onClick)
  const cls = cx(styles.card, interactive && styles.clickable, dimmed && styles.dimmed)
  return (
    <motion.svg
      layout
      layoutId={card && !hidden ? cardKey(card) : undefined}
      initial={{ opacity: 0, scale: 0.85 }}
      animate={{ opacity: 1, scale: 1, y: selected ? -12 : 0 }}
      exit={{ opacity: 0, scale: 0.85 }}
      transition={{ type: 'spring', stiffness: 500, damping: 40 }}
      viewBox="0 0 60 84"
      className={cls}
      role={interactive ? 'button' : 'img'}
      aria-label={card && !hidden ? cardLabel(card) : 'закрытая карта'}
      aria-disabled={dimmed || undefined}
      tabIndex={interactive ? 0 : undefined}
      onClick={onClick}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onClick!()
              }
            }
          : undefined
      }
      data-testid={hidden ? 'card-back' : 'card-face'}
    >
      <rect x="1" y="1" width="58" height="82" rx="6" className={styles.frame} />
      {hidden ? (
        <rect x="6" y="6" width="48" height="72" rx="4" className={styles.back} />
      ) : (
        <g className={red ? styles.red : styles.black}>
          <text x="7" y="18" fontSize="14" fontWeight="700">
            {rankLabel(card!.rank)}
          </text>
          <text x="30" y="54" fontSize="30" textAnchor="middle">
            {card!.suit}
          </text>
        </g>
      )}
    </motion.svg>
  )
}
```

> Примечание: `onKeyDown` и видимый фокус вводятся здесь ради связности `Card` (тест A11y —
> в Task 3). Это допустимо: правка одного компонента, гейт остаётся зелёным.

- [ ] **Step 4: Стили `dimmed`/фокус в `Card.module.css`**

В `web/src/ui/table/Card.module.css` добавить в конец:

```css
.dimmed {
  opacity: 0.4;
}
.clickable:focus-visible {
  outline: 3px solid var(--accent);
  outline-offset: 2px;
}
```

- [ ] **Step 5: Новые пропы `Hand.tsx`**

Заменить `web/src/ui/table/Hand.tsx` на:

```tsx
import { AnimatePresence } from 'motion/react'
import { cardKey, type Card as CardT } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface HandProps {
  cards: CardT[]
  selectedKey: string | null
  playableKeys: Set<string>
  onSelect: (card: CardT) => void
}

export function Hand({ cards, selectedKey, playableKeys, onSelect }: HandProps) {
  return (
    <div className={styles.hand} data-testid="hand">
      <AnimatePresence>
        {cards.map((c) => {
          const key = cardKey(c)
          const playable = playableKeys.has(key)
          return (
            <Card
              key={key}
              card={c}
              selected={key === selectedKey}
              dimmed={!playable}
              onClick={playable ? () => onSelect(c) : undefined}
            />
          )
        })}
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 6: Починить `animation.test.tsx` (сигнатура Hand)**

В `web/src/ui/table/animation.test.tsx` заменить строку рендера Hand:

```tsx
  render(<Hand cards={hand} selectedIndex={null} onSelect={() => {}} />)
```

на:

```tsx
  render(<Hand cards={hand} selectedKey={null} playableKeys={new Set()} onSelect={() => {}} />)
```

- [ ] **Step 7: Перевести `Table.tsx` на `selectedKey` + legal (частично)**

Заменить `web/src/ui/screens/Table.tsx` на версию с `selectedKey` и подсветкой. Кнопки ActionBar
пока оставляем в старой форме (полноценно — Task 3/5), но выбор карты уже по ключу:

```tsx
import { useState } from 'react'
import { cardKey, isCardPlayable, isYourTurn } from '../../contract/types'
import { useGameStore, selectSeats, selectView, selectLegal } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ActionBar } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGameStore(selectView)
  const seats = useGameStore(selectSeats)
  const legal = useGameStore(selectLegal)
  const play = useGameStore((s) => s.play)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameBySeat = new Map(seats.map((s) => [s.seat, s.name]))
  const nameOf = (seat: number) => nameBySeat.get(seat) ?? `Игрок ${seat}`

  const playableKeys = new Set(view.hand.filter((c) => isCardPlayable(legal, c)).map(cardKey))

  const onSelect = (card: (typeof view.hand)[number]) => {
    const key = cardKey(card)
    if (key === selectedKey) {
      play({ type: 'playCard', card })
      setSelectedKey(null)
      return
    }
    setSelectedKey(key)
  }

  return (
    <div className={styles.table}>
      <div className={styles.opponents}>
        {view.opponents.map((o) => (
          <OpponentSeat key={o.seat} name={nameOf(o.seat)} opponent={o} />
        ))}
      </div>
      <Con table={view.table} />
      <ActionBar
        yourTurn={isYourTurn(view)}
        onShukh={() => play({ type: 'claimShukh', target: view.turn, code: 2 })}
        onOneCard={() => {
          /* полноценно — Task 5 */
        }}
        onTakeBottom={() => play({ type: 'takeBottomAndPass' })}
      />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={playableKeys}
        onSelect={onSelect}
      />
    </div>
  )
}
```

> **Фикс фантомного подъёма:** после хода `selectedKey` очищается; даже если бы не очищался, он
> сравнивается по `cardKey`, а не по индексу — новая карта с индексом 0 больше не «подпрыгивает»
> (leдджер 2b). Селект переживает перерисовку руки корректно.

- [ ] **Step 8: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add web/src/ui/table/Card.tsx web/src/ui/table/Card.module.css web/src/ui/table/Hand.tsx web/src/ui/table/Hand.test.tsx web/src/ui/table/animation.test.tsx web/src/ui/screens/Table.tsx
git commit -m "feat(web): legal-driven hand highlight + select-by-cardKey (fixes phantom lift)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Выбор + подтверждение + A11y-клавиатура

**Files:**
- Modify: `web/src/ui/table/ActionBar.tsx`
- Modify: `web/src/ui/table/ActionBar.test.tsx`
- Modify: `web/src/ui/table/Card.test.tsx`
- Modify: `web/src/ui/screens/Table.tsx`

**Interfaces:**
- Consumes (Task 1): `isLegal`, `claimShukhInLegal`, `isShukhTakeable`, `selectLegal`.
- Produces:
  - `ActionBar` props: `{ canConfirm: boolean; onConfirm: () => void; canTakeBottom: boolean; onTakeBottom: () => void; canShukh: boolean; onShukh: () => void; owesOneCard: boolean; onOneCard: () => void }`
  - `Table` подтверждает ход по второму клику ИЛИ кнопке «Сходить».

- [ ] **Step 1: Тест A11y-активации карты клавиатурой (падает)**

В `web/src/ui/table/Card.test.tsx` добавить импорт userEvent сверху и тест:

```tsx
import userEvent from '@testing-library/user-event'
```

```tsx
test('Enter активирует карту (A11y)', async () => {
  const onClick = vi.fn()
  render(<Card card={{ suit: '♦', rank: 9 }} onClick={onClick} />)
  screen.getByRole('button', { name: '9♦' }).focus()
  await userEvent.keyboard('{Enter}')
  expect(onClick).toHaveBeenCalled()
})

test('Space активирует карту (A11y)', async () => {
  const onClick = vi.fn()
  render(<Card card={{ suit: '♦', rank: 9 }} onClick={onClick} />)
  screen.getByRole('button', { name: '9♦' }).focus()
  await userEvent.keyboard('[Space]')
  expect(onClick).toHaveBeenCalled()
})
```

- [ ] **Step 2: Прогнать — Enter/Space уже реализованы в Task 2 → должны пройти сразу**

Run: `cd web && npx vitest run src/ui/table/Card.test.tsx`
Expected: PASS (обработчик `onKeyDown` добавлен в Task 2). Если FAIL — доделать `onKeyDown` в `Card.tsx`.

- [ ] **Step 3: Новые тесты ActionBar (падают)**

Заменить `web/src/ui/table/ActionBar.test.tsx` на:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

const props = {
  canConfirm: false,
  onConfirm: () => {},
  canTakeBottom: false,
  onTakeBottom: () => {},
  canShukh: false,
  onShukh: () => {},
  owesOneCard: false,
  onOneCard: () => {},
}

test('«Сходить» отключена без выбранной легальной карты', () => {
  render(<ActionBar {...props} canConfirm={false} />)
  expect(screen.getByRole('button', { name: 'Сходить' })).toBeDisabled()
})

test('«Сходить» активна и зовёт onConfirm', async () => {
  const onConfirm = vi.fn()
  render(<ActionBar {...props} canConfirm onConfirm={onConfirm} />)
  await userEvent.click(screen.getByRole('button', { name: 'Сходить' }))
  expect(onConfirm).toHaveBeenCalled()
})

test('«ШУХ!» отключена без открытого ШУХ-окна', () => {
  render(<ActionBar {...props} canShukh={false} />)
  expect(screen.getByRole('button', { name: 'ШУХ!' })).toBeDisabled()
})

test('«ШУХ!» активна при canShukh и зовёт onShukh', async () => {
  const onShukh = vi.fn()
  render(<ActionBar {...props} canShukh onShukh={onShukh} />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(onShukh).toHaveBeenCalled()
})

test('«Взять низ» активна только при canTakeBottom', () => {
  const { rerender } = render(<ActionBar {...props} canTakeBottom={false} />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeDisabled()
  rerender(<ActionBar {...props} canTakeBottom />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeEnabled()
})

test('«Одна карта!» активна только когда owesOneCard', () => {
  render(<ActionBar {...props} owesOneCard />)
  expect(screen.getByRole('button', { name: 'Одна карта!' })).toBeEnabled()
})
```

- [ ] **Step 4: Прогнать — падает (старые пропы ActionBar)**

Run: `cd web && npx vitest run src/ui/table/ActionBar.test.tsx`
Expected: FAIL (props mismatch).

- [ ] **Step 5: Переписать `ActionBar.tsx`**

Заменить `web/src/ui/table/ActionBar.tsx` на:

```tsx
import { Button } from '../kit/Button'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

interface ActionBarProps {
  canConfirm: boolean
  onConfirm: () => void
  canTakeBottom: boolean
  onTakeBottom: () => void
  canShukh: boolean
  onShukh: () => void
  owesOneCard: boolean
  onOneCard: () => void
}

export function ActionBar({
  canConfirm,
  onConfirm,
  canTakeBottom,
  onTakeBottom,
  canShukh,
  onShukh,
  owesOneCard,
  onOneCard,
}: ActionBarProps) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      <Button onClick={onConfirm} disabled={!canConfirm}>
        Сходить
      </Button>
      <Button onClick={onTakeBottom} disabled={!canTakeBottom}>
        Взять низ
      </Button>
      <Button onClick={onShukh} disabled={!canShukh}>
        ШУХ!
      </Button>
      <Button
        onClick={onOneCard}
        disabled={!owesOneCard}
        className={cx(owesOneCard && styles.pulse)}
      >
        Одна карта!
      </Button>
    </div>
  )
}
```

- [ ] **Step 6: Стиль `pulse` в `Table.module.css`**

В `web/src/ui/table/Table.module.css` добавить в конец:

```css
.pulse {
  animation: pulse 1s ease-in-out infinite;
}
@keyframes pulse {
  50% {
    box-shadow: 0 0 0 3px var(--accent);
  }
}
@media (prefers-reduced-motion: reduce) {
  .pulse {
    animation: none;
  }
}
```

- [ ] **Step 7: Обновить `Table.tsx` — confirm-логика и новые пропы ActionBar**

Заменить `web/src/ui/screens/Table.tsx` на (кнопки ШУХ/OneCard пока минимальны — полноценно Task 5, но подключаем `canConfirm`/`canTakeBottom`/confirm-on-second-click уже сейчас):

```tsx
import { useState } from 'react'
import { cardKey, isCardPlayable, isLegal } from '../../contract/types'
import { useGameStore, selectSeats, selectView, selectLegal } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ActionBar } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGameStore(selectView)
  const seats = useGameStore(selectSeats)
  const legal = useGameStore(selectLegal)
  const play = useGameStore((s) => s.play)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameBySeat = new Map(seats.map((s) => [s.seat, s.name]))
  const nameOf = (seat: number) => nameBySeat.get(seat) ?? `Игрок ${seat}`

  const playableKeys = new Set(view.hand.filter((c) => isCardPlayable(legal, c)).map(cardKey))
  const selectedCard = view.hand.find((c) => cardKey(c) === selectedKey) ?? null
  const canConfirm = selectedCard != null && isCardPlayable(legal, selectedCard)
  const canTakeBottom = isLegal(legal, { type: 'takeBottomAndPass' })

  const confirmPlay = () => {
    if (!selectedCard) return
    play({ type: 'playCard', card: selectedCard })
    setSelectedKey(null)
  }
  const onSelect = (card: (typeof view.hand)[number]) => {
    const key = cardKey(card)
    if (key === selectedKey) {
      confirmPlay()
      return
    }
    setSelectedKey(key)
  }

  return (
    <div className={styles.table}>
      <div className={styles.opponents}>
        {view.opponents.map((o) => (
          <OpponentSeat key={o.seat} name={nameOf(o.seat)} opponent={o} />
        ))}
      </div>
      <Con table={view.table} />
      <ActionBar
        canConfirm={canConfirm}
        onConfirm={confirmPlay}
        canTakeBottom={canTakeBottom}
        onTakeBottom={() => play({ type: 'takeBottomAndPass' })}
        canShukh={false}
        onShukh={() => {}}
        owesOneCard={false}
        onOneCard={() => {}}
      />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={playableKeys}
        onSelect={onSelect}
      />
    </div>
  )
}
```

- [ ] **Step 8: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add web/src/ui/table/ActionBar.tsx web/src/ui/table/ActionBar.test.tsx web/src/ui/table/Card.test.tsx web/src/ui/table/Table.module.css web/src/ui/screens/Table.tsx
git commit -m "feat(web): select+confirm (second click / «Сходить») + keyboard-activated cards

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `ShukhZone` — отложенная стопка у мест (своя кликабельна при takeable)

**Files:**
- Create: `web/src/ui/table/ShukhZone.tsx`
- Create: `web/src/ui/table/ShukhZone.test.tsx`
- Modify: `web/src/ui/table/Table.module.css`
- Modify: `web/src/ui/table/OpponentSeat.tsx`
- Modify: `web/src/ui/table/OpponentSeat.test.tsx`
- Modify: `web/src/ui/screens/Table.tsx`

**Interfaces:**
- Consumes (Task 1): `isShukhTakeable`, `selectLegal`; `SeatView.shukhPending`, `OpponentView.shukhPending`.
- Produces:
  - `ShukhZone` props: `{ count: number; takeable?: boolean; onTake?: () => void; label?: string }`
  - `OpponentSeat` рендерит `<ShukhZone count={opponent.shukhPending} label={...} />` (вместо `shukhBadge`).
  - `Table` рендерит свою `<ShukhZone count={view.shukhPending} takeable onTake .../>`.

- [ ] **Step 1: Тесты ShukhZone (падают — файла нет)**

Создать `web/src/ui/table/ShukhZone.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ShukhZone } from './ShukhZone'

test('показывает счётчик отложенных карт', () => {
  render(<ShukhZone count={2} />)
  expect(screen.getByTestId('shukh-count')).toHaveTextContent('ШУХ 2')
})

test('пустая не-takeable зона не рендерится', () => {
  const { container } = render(<ShukhZone count={0} />)
  expect(container.firstChild).toBeNull()
})

test('takeable зона кликабельна и зовёт onTake', async () => {
  const onTake = vi.fn()
  render(<ShukhZone count={2} takeable onTake={onTake} label="Ваша ШУХ-зона" />)
  await userEvent.click(screen.getByRole('button', { name: 'Ваша ШУХ-зона' }))
  expect(onTake).toHaveBeenCalled()
})

test('не-takeable зона не имеет роли button', () => {
  render(<ShukhZone count={2} />)
  expect(screen.queryByRole('button')).toBeNull()
})
```

- [ ] **Step 2: Прогнать — падает (нет файла)**

Run: `cd web && npx vitest run src/ui/table/ShukhZone.test.tsx`
Expected: FAIL (cannot find module `./ShukhZone`).

- [ ] **Step 3: Реализовать `ShukhZone.tsx`**

Создать `web/src/ui/table/ShukhZone.tsx`:

```tsx
import { motion, AnimatePresence } from 'motion/react'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

interface ShukhZoneProps {
  count: number
  takeable?: boolean
  onTake?: () => void
  label?: string
}

// Отложенные ШУХ-карты места (I-3 — не входят в руку). Рубашкой вверх, только счётчик.
// Своя зона кликабельна, когда взятие законно (R-8.3, гейтится legal через takeable).
export function ShukhZone({ count, takeable, onTake, label }: ShukhZoneProps) {
  if (count === 0 && !takeable) return null
  const interactive = Boolean(takeable && onTake)
  return (
    <div
      className={cx(styles.shukhZone, interactive && styles.shukhTakeable)}
      data-testid="shukh-zone"
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      aria-label={label ?? `ШУХ-зона: ${count}`}
      title={takeable ? 'Забрать ШУХ-карты' : 'Отложенные ШУХ-карты'}
      onClick={interactive ? onTake : undefined}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onTake?.()
              }
            }
          : undefined
      }
    >
      <div className={styles.shukhStack}>
        <AnimatePresence>
          {Array.from({ length: count }, (_, i) => (
            <motion.span
              key={i}
              className={styles.shukhChip}
              initial={{ opacity: 0, scale: 0.6 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.6 }}
            />
          ))}
        </AnimatePresence>
      </div>
      <span className={styles.shukhCount} data-testid="shukh-count">
        ШУХ {count}
      </span>
    </div>
  )
}
```

- [ ] **Step 4: Стили ШУХ-зоны в `Table.module.css`**

В `web/src/ui/table/Table.module.css` добавить в конец:

```css
.shukhZone {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  margin-top: 4px;
}
.shukhTakeable {
  cursor: pointer;
}
.shukhTakeable:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.shukhStack {
  display: flex;
  gap: 2px;
  min-height: 16px;
}
.shukhChip {
  width: 10px;
  height: 14px;
  border-radius: 2px;
  background: var(--card-back);
  box-shadow: var(--shadow);
}
.shukhCount {
  font-size: 0.8em;
  opacity: 0.85;
}
```

- [ ] **Step 5: Обновить тест `OpponentSeat.test.tsx`**

Заменить в `web/src/ui/table/OpponentSeat.test.tsx` проверку `shukh-badge` на `shukh-count`.
Прочитай файл и замени идентификаторы: там, где ожидается `getByTestId('shukh-badge')` с текстом
`ШУХ 1`, должно стать `getByTestId('shukh-count')` с текстом `ШУХ 1`. Если тест проверяет
отсутствие бейджа при `shukhPending: 0` — оставить (зона при count=0 и без takeable не рендерится,
`queryByTestId('shukh-zone')` = null).

- [ ] **Step 6: Прогнать — падает (OpponentSeat ещё рендерит shukhBadge)**

Run: `cd web && npx vitest run src/ui/table/OpponentSeat.test.tsx`
Expected: FAIL.

- [ ] **Step 7: Рендерить `ShukhZone` в `OpponentSeat.tsx`**

Заменить `web/src/ui/table/OpponentSeat.tsx` на:

```tsx
import type { OpponentView } from '../../contract/types'
import { ShukhZone } from './ShukhZone'
import styles from './Table.module.css'

interface OpponentSeatProps {
  name: string
  opponent: OpponentView
}

export function OpponentSeat({ name, opponent }: OpponentSeatProps) {
  return (
    <div className={styles.seat} data-testid={`seat-${opponent.seat}`}>
      <div className={styles.seatName}>{name}</div>
      <div className={styles.seatCount}>🂠 {opponent.handCount}</div>
      <ShukhZone count={opponent.shukhPending} label={`ШУХ-зона ${name}: ${opponent.shukhPending}`} />
    </div>
  )
}
```

- [ ] **Step 8: Своя ШУХ-зона в `Table.tsx`**

В `web/src/ui/screens/Table.tsx`:
1. В импорт из `../../contract/types` добавить `isShukhTakeable`.
2. Добавить импорт: `import { ShukhZone } from '../table/ShukhZone'`.
3. После вычисления `canTakeBottom` добавить:

```tsx
  const yourZoneTakeable = isShukhTakeable(legal, view.you)
```

4. В JSX между `<Con .../>` и `<ActionBar .../>` вставить:

```tsx
      <ShukhZone
        count={view.shukhPending}
        takeable={yourZoneTakeable}
        onTake={() => play({ type: 'takeShukhCards', seat: view.you })}
        label={`Ваша ШУХ-зона: ${view.shukhPending}`}
      />
```

5. В `.table` grid (`Table.module.css`) число строк выросло — заменить
   `grid-template-rows: auto 1fr auto auto;` на `grid-template-rows: auto 1fr auto auto auto;`.

- [ ] **Step 9: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add web/src/ui/table/ShukhZone.tsx web/src/ui/table/ShukhZone.test.tsx web/src/ui/table/OpponentSeat.tsx web/src/ui/table/OpponentSeat.test.tsx web/src/ui/table/Table.module.css web/src/ui/screens/Table.tsx
git commit -m "feat(web): ShukhZone — deferred-cards counter per seat, own zone takeable via legal

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Живые «ШУХ!» и «Одна карта!»

**Files:**
- Modify: `web/src/ui/screens/Table.tsx`
- Create: `web/src/ui/screens/Table.test.tsx`

**Interfaces:**
- Consumes (Task 1/3): `claimShukhInLegal`, `ActionBar` пропы `canShukh/onShukh/owesOneCard/onOneCard`.
- Produces: `Table` вычисляет `canShukh` из legal, `owesOneCard` из `view` (client-local), проводит их в ActionBar.

**Замечание по «Одна карта!» (решение дизайна, для ревью владельцем).** В `Action`/`SeatView`
движка НЕТ действия/поля «объявил одну карту» (это устная соц-механика R-6.1). Поэтому моделируем
минимально и честно: `owesOneCard` **выводится** из вида (`live[you] && hand.length === 1`), а клик
по «Одна карта!» — **клиентское** подтверждение (`announced=true`), гасящее пульсацию; в транспорт
НЕ уходит (нет игрового действия). Ветку «забыл → Ш-11» демонстрируем скриптованно на боте (Task 7),
а не интерактивной развилкой. Если владелец захочет провести объявление через транспорт — добавим
действие в контракт отдельным правилом (сейчас — YAGNI, W2-6).

- [ ] **Step 1: Тесты Table — «ШУХ!» и «Одна карта!» (падают — файла нет)**

Создать `web/src/ui/screens/Table.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { create } from 'zustand'
import type { Action, GameSnapshot } from '../../contract/types'

// Изолированный стор-даблинг: подменяем useGameStore локальным zustand-стором.
// Так тестируем Table без реального транспорта.
const sent: Action[] = []
let snapshot: GameSnapshot

vi.mock('../../store/game', async () => {
  const actual = await vi.importActual<typeof import('../../store/game')>('../../store/game')
  const store = create<import('../../store/game').GameState>(() => ({
    snapshot: null,
    events: [],
    play: (a: Action) => sent.push(a),
  }))
  return { ...actual, useGameStore: store }
})

import { useGameStore } from '../../store/game'
import { Table } from './Table'
import { buildSeatView } from '../../fixtures/seatView'

const SEATS = [
  { seat: 0, name: 'Аня', ready: true },
  { seat: 1, name: 'Боря', ready: true },
]

function setSnapshot(over: Partial<GameSnapshot>) {
  snapshot = {
    roomCode: 'DEMO',
    seats: SEATS,
    view: buildSeatView({ opponents: [{ seat: 1, handCount: 3, shukhPending: 0, live: true }] }),
    legal: [],
    shukhVote: null,
    ...over,
  }
  ;(useGameStore as unknown as { setState: (s: Partial<unknown>) => void }).setState({ snapshot })
}

beforeEach(() => {
  sent.length = 0
})

test('«ШУХ!» активна и шлёт конкретный claimShukh из legal', async () => {
  setSnapshot({
    view: buildSeatView({
      hand: [{ suit: '♠', rank: 6 }],
      opponents: [{ seat: 1, handCount: 1, shukhPending: 0, live: true }],
    }),
    legal: [{ type: 'claimShukh', target: 1, code: 11 }],
  })
  render(<Table />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(sent).toContainEqual({ type: 'claimShukh', target: 1, code: 11 })
})

test('«Одна карта!» пульсирует при 1 карте на руке и гасится по клику', async () => {
  setSnapshot({
    view: buildSeatView({ hand: [{ suit: '♠', rank: 6 }], live: { 0: true } }),
    legal: [],
  })
  render(<Table />)
  const btn = screen.getByRole('button', { name: 'Одна карта!' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(btn).toBeDisabled() // announced → пульсация/доступность гаснут
})
```

- [ ] **Step 2: Прогнать — падает (нет файла Table.test.tsx / логики)**

Run: `cd web && npx vitest run src/ui/screens/Table.test.tsx`
Expected: FAIL (файла нет; после создания — на отсутствии canShukh/owesOneCard).

- [ ] **Step 3: Вплести `canShukh`/`owesOneCard` в `Table.tsx`**

В `web/src/ui/screens/Table.tsx`:
1. Импорт из `react`: `import { useState, useEffect } from 'react'`.
2. В импорт из `../../contract/types` добавить `claimShukhInLegal`.
3. После `const [selectedKey, setSelectedKey] = useState<string | null>(null)` добавить:

```tsx
  const [announced, setAnnounced] = useState(false)
  const handLen = view?.hand.length ?? 0
  useEffect(() => {
    if (handLen !== 1) setAnnounced(false)
  }, [handLen])
```

> Хуки — до раннего `return` (правило хуков). `view?.hand.length` безопасно при `view === null`.

4. После `const yourZoneTakeable = ...` добавить:

```tsx
  const claim = claimShukhInLegal(legal)
  const owesOneCard = (view.live[view.you] ?? false) && view.hand.length === 1 && !announced
```

5. В `<ActionBar>` заменить `canShukh`/`onShukh`/`owesOneCard`/`onOneCard` на:

```tsx
        canShukh={claim != null}
        onShukh={() => claim && play(claim)}
        owesOneCard={owesOneCard}
        onOneCard={() => setAnnounced(true)}
```

- [ ] **Step 4: Прогнать таргетно**

Run: `cd web && npx vitest run src/ui/screens/Table.test.tsx`
Expected: PASS.

- [ ] **Step 5: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/ui/screens/Table.tsx web/src/ui/screens/Table.test.tsx
git commit -m "feat(web): live «ШУХ!» (claim from legal) + «Одна карта!» (client-local announce)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Модалка голосования/оспаривания ШУХа (R-8.6)

**Files:**
- Create: `web/src/ui/table/ShukhVoteModal.tsx`
- Create: `web/src/ui/table/ShukhVoteModal.test.tsx`
- Modify: `web/src/ui/table/Table.module.css`
- Modify: `web/src/ui/screens/Table.tsx`

**Interfaces:**
- Consumes (Task 1): `ShukhVote`, `selectShukhVote`.
- Produces: `ShukhVoteModal` props `{ vote: ShukhVote; nameOf: (seat: number) => string }` (без кнопки закрытия — модалка управляется снапшотом, W2-1).

- [ ] **Step 1: Тесты модалки (падают — файла нет)**

Создать `web/src/ui/table/ShukhVoteModal.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { ShukhVoteModal } from './ShukhVoteModal'
import type { ShukhVote } from '../../contract/types'

const nameOf = (s: number) => `Игрок ${s}`

const voting: ShukhVote = {
  claimant: 0,
  target: 1,
  code: 11,
  votes: [{ seat: 2, up: true }],
  outcome: 'upheld',
  resolved: false,
}

test('показывает цель, код и голоса во время голосования', () => {
  render(<ShukhVoteModal vote={voting} nameOf={nameOf} />)
  expect(screen.getByTestId('shukh-vote')).toHaveTextContent('Ш-11')
  expect(screen.getByTestId('shukh-vote')).toHaveTextContent('Игрок 1')
  expect(screen.getByTestId('vote-2')).toHaveTextContent('за')
  expect(screen.queryByTestId('vote-outcome')).toBeNull() // ещё не resolved
})

test('resolved=upheld показывает исход «подтверждён»', () => {
  render(<ShukhVoteModal vote={{ ...voting, resolved: true }} nameOf={nameOf} />)
  expect(screen.getByTestId('vote-outcome')).toHaveTextContent('подтверждён')
})

test('resolved=overturned показывает Ш-8 предъявившему', () => {
  render(
    <ShukhVoteModal
      vote={{ ...voting, resolved: true, outcome: 'overturned' }}
      nameOf={nameOf}
    />,
  )
  expect(screen.getByTestId('vote-outcome')).toHaveTextContent('Ш-8')
})
```

- [ ] **Step 2: Прогнать — падает (нет файла)**

Run: `cd web && npx vitest run src/ui/table/ShukhVoteModal.test.tsx`
Expected: FAIL (cannot find module).

- [ ] **Step 3: Реализовать `ShukhVoteModal.tsx`**

Создать `web/src/ui/table/ShukhVoteModal.tsx`:

```tsx
import type { ShukhVote } from '../../contract/types'
import styles from './Table.module.css'

interface ShukhVoteModalProps {
  vote: ShukhVote
  nameOf: (seat: number) => string
}

// Голосование/оспаривание ШУХа (R-8.6). Данные (голоса/исход) — из снапшота (W2-7,
// кворум не считаем); модалка чисто презентационна и управляется таймлайном сценария.
export function ShukhVoteModal({ vote, nameOf }: ShukhVoteModalProps) {
  const ups = vote.votes.filter((v) => v.up).length
  return (
    <div
      className={styles.modalBackdrop}
      role="dialog"
      aria-modal="true"
      aria-label="Голосование по ШУХу"
      data-testid="shukh-vote"
    >
      <div className={styles.modal}>
        <h3>
          ШУХ на «{nameOf(vote.target)}» (Ш-{vote.code})
        </h3>
        <p>Предъявил: {nameOf(vote.claimant)}</p>
        <ul className={styles.voteList}>
          {vote.votes.map((v) => (
            <li key={v.seat} data-testid={`vote-${v.seat}`}>
              {nameOf(v.seat)}: {v.up ? '✅ за' : '❌ против'}
            </li>
          ))}
        </ul>
        {vote.resolved ? (
          <p className={styles.voteOutcome} data-testid="vote-outcome">
            {vote.outcome === 'upheld'
              ? `ШУХ подтверждён (${ups} за)`
              : 'ШУХ отклонён — Ш-8 предъявившему'}
          </p>
        ) : (
          <p className={styles.voteTallying}>Голосование…</p>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Стили модалки в `Table.module.css`**

В `web/src/ui/table/Table.module.css` добавить в конец:

```css
.modalBackdrop {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
}
.modal {
  max-width: 320px;
  padding: 20px;
  text-align: center;
  border: 1px solid var(--accent);
  border-radius: var(--radius);
  background: var(--felt-dark);
  box-shadow: var(--shadow);
}
.voteList {
  list-style: none;
  margin: 8px 0;
  padding: 0;
}
.voteOutcome {
  font-weight: 700;
  color: var(--accent);
}
.voteTallying {
  opacity: 0.8;
}
```

- [ ] **Step 5: Подключить модалку в `Table.tsx`**

В `web/src/ui/screens/Table.tsx`:
1. Добавить импорт: `import { ShukhVoteModal } from '../table/ShukhVoteModal'`.
2. В импорт стора добавить `selectShukhVote`: `import { useGameStore, selectSeats, selectView, selectLegal, selectShukhVote } from '../../store/game'`.
3. После `const shukhVote`-подобных чтений добавить чтение (рядом с `const legal = ...`):

```tsx
  const shukhVote = useGameStore(selectShukhVote)
```

4. В JSX перед закрывающим `</div>` таблицы добавить:

```tsx
      {shukhVote && <ShukhVoteModal vote={shukhVote} nameOf={nameOf} />}
```

- [ ] **Step 6: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/ui/table/ShukhVoteModal.tsx web/src/ui/table/ShukhVoteModal.test.tsx web/src/ui/table/Table.module.css web/src/ui/screens/Table.tsx
git commit -m "feat(web): ShukhVoteModal (R-8.6) — scripted votes + outcome, snapshot-driven

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Расширить демо-сценарий ШУХ-окном + голосованием + оплатой; интеграционный тест; визуальная проверка

**Files:**
- Modify: `web/src/fixtures/scenario.ts`
- Create: `web/src/fixtures/scenario.test.ts`

**Interfaces:**
- Consumes всё выше: `GameSnapshot.shukhVote`, `claimShukh`/`shukhPaid`/`shukhAssessed`, `ShukhZone`, `ShukhVoteModal`.
- Produces: `demoScenario` с rules-корректным ШУХ-эпизодом (Ш-11), интеграционный тест на переходы стора.

**Rules-обоснование эпизода (сверено с `docs/shukh-rules.md`):**
- После шага 5 (вы закрыли кон Дамой♥, R-3.7.1) — ваш заход (R-5.7), рука `[6♠,14♣,7♦]`,
  opp Боря=4, Вера=3.
- Кон A: вы заходите 6♠; Боря бьёт 8♠, Вера бьёт 9♠ (пика бьётся старшей пикой, R-3.3;
  6<8<9). 3 карты = 3 живых → кон закрывается **по счёту** (R-5.5); Вера закрыла → Вера
  заходит следующий (R-5.7). Вера: 3→2.
- Кон B: Вера заходит 10♦ (козырь, R-2.5), оставаясь с **1 картой**, и **не объявляет
  «Одна карта!»** (нарушение R-6.1a). У вас `[14♣,7♦]`: козырь 10♦ старше вашего 7♦ и не
  бьётся клубой → бить нечем, легально только «Взять низ». Плюс открыто ШУХ-окно на Веру
  (Ш-11, R-6.2): `legal += claimShukh{target:2, code:11}`.
- Вы жмёте «ШУХ!». Вера **оспаривает** (R-8.6) → голосование: судит третий за столом —
  Боря (R-8.9); скрипт: Боря «за» → большинство **подтверждает** ШУХ (кворум не считаем, W2-7).
- Начисление (R-8.1): каждый прочий игрок отдаёт Вере по карте. Вы отдаёте 14♣ (остаётесь
  с 7♦; последнюю не отдают — I-2/R-8.1.1, у вас 2 карты — можно). Боря отдаёт карту
  (значение для рубашки не важно — уходит в ШУХ-зону Веры лицом вниз; берём 5♣). Вера
  `shukhPending`: 0→2 (I-3 — не в руку, R-8.3). Ваша рука: 2→1 → теперь **у вас** горит
  «Одна карта!».
- Ход возвращается к вам (R-8.5, позиция восстановлена): бить 10♦ нечем → «Взять низ».
  Финал демо.

> Идентичность карт: Дама♥ уже сыграна вами (шаг 5), поэтому Ш-2 (заход с Дамы♥) в этом
> эпизоде невозможен (одна Дама♥ в Deck36) — используем Ш-11, конфликтов уникальности нет.
> Собственную ШУХ-зону-взятие (takeShukhCards) в линейном демо не форсируем (вы — предъявитель,
> карт не получаете); функционально она покрыта тестами (Task 4). Владелец может запросить
> живой «хвост» со взятием на ревью.

- [ ] **Step 1: Интеграционный тест сценария (падает — новых шагов ещё нет)**

Создать `web/src/fixtures/scenario.test.ts`:

```ts
import { createScriptedTransport, type Scheduler } from '../transport/scripted'
import { demoScenario } from './scenario'
import type { GameSnapshot } from '../contract/types'
import { claimShukhInLegal } from '../contract/types'

const sync: Scheduler = (fn) => fn()

// Прогоняет сценарий синхронно, собирая все пуш-снапшоты; на каждом await-шаге
// отправляет его expect (эмулируем игрока, идущего строго по скрипту).
function runToEnd(): GameSnapshot[] {
  const snaps: GameSnapshot[] = []
  const t = createScriptedTransport(demoScenario, sync)
  t.subscribe(
    (s) => snaps.push(s),
    () => {},
  )
  // синхронно докручиваем await-шаги их ожидаемыми действиями
  for (const step of demoScenario) {
    if (step.kind === 'await') t.send(step.expect)
  }
  return snaps
}

test('в ходе сценария открывается ШУХ-окно (claimShukh в legal)', () => {
  const snaps = runToEnd()
  const withClaim = snaps.filter((s) => claimShukhInLegal(s.legal) != null)
  expect(withClaim.length).toBeGreaterThan(0)
  expect(claimShukhInLegal(withClaim[0].legal)).toMatchObject({ target: 2, code: 11 })
})

test('после предъявления ШУХа поднимается голосование, затем исход', () => {
  const snaps = runToEnd()
  const voting = snaps.filter((s) => s.shukhVote && !s.shukhVote.resolved)
  const resolved = snaps.filter((s) => s.shukhVote?.resolved)
  expect(voting.length).toBeGreaterThan(0)
  expect(resolved.length).toBeGreaterThan(0)
  expect(resolved[0].shukhVote?.outcome).toBe('upheld')
})

test('оплата ШУХа: нарушитель (Вера, seat 2) получает 2 отложенные карты; ваша рука убыла до 1', () => {
  const snaps = runToEnd()
  const last = snaps[snaps.length - 1]
  const vera = last.view?.opponents.find((o) => o.seat === 2)
  expect(vera?.shukhPending).toBe(2)
  expect(last.view?.hand.length).toBe(1)
  expect(last.shukhVote ?? null).toBeNull() // модалка закрыта после оплаты
})
```

- [ ] **Step 2: Прогнать — падает (эпизода нет)**

Run: `cd web && npx vitest run src/fixtures/scenario.test.ts`
Expected: FAIL (нет ШУХ-окна/голосования/оплаты в текущем сценарии).

- [ ] **Step 3: Дописать ШУХ-эпизод в `scenario.ts`**

В `web/src/fixtures/scenario.ts`:
1. Расширить хелпер `opp`, чтобы принимать `shukhPending` Веры (для отображения роста зоны).
   Заменить функцию `opp` на:

```ts
const opp = (bori: number, vera: number, veraShukh = 0) => [
  { seat: 1, handCount: bori, shukhPending: 0, live: true },
  { seat: 2, handCount: vera, shukhPending: veraShukh, live: true },
]
```

2. Добавить в `base(...)` проброс `shukhVote` — заменить сигнатуру и тело `base` на:

```ts
function base(
  over: Partial<SeatView>,
  legal: GameSnapshot['legal'],
  shukhVote: GameSnapshot['shukhVote'] = null,
): GameSnapshot {
  return {
    roomCode: 'DEMO',
    seats: SEATS,
    view: buildSeatView({ opponents: opp(5, 5), live: { 0: true, 1: true, 2: true }, ...over }),
    legal,
    shukhVote,
  }
}
```

3. Перед закрывающей `]` массива `demoScenario` (после шага 5) добавить шаги 6–12:

```ts
  // 6. Ваш заход 6♠ (Кон A). Рука [14♣,7♦].
  {
    kind: 'await',
    expect: { type: 'playCard', card: c(6, '♠') },
    events: [{ type: 'cardPlayed', seat: 0, card: c(6, '♠') }],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [{ card: c(6, '♠'), by: 0 }],
        discard: 5,
        turn: 1,
        opponents: opp(4, 3),
      },
      [],
    ),
  },
  // 7. Боря бьёт 8♠ (пика старшей пикой, R-3.3) (auto).
  {
    kind: 'auto',
    delayMs: DELAY,
    events: [{ type: 'cardPlayed', seat: 1, card: c(8, '♠') }],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [
          { card: c(6, '♠'), by: 0 },
          { card: c(8, '♠'), by: 1 },
        ],
        discard: 5,
        turn: 2,
        opponents: opp(3, 3),
      },
      [],
    ),
  },
  // 8. Вера бьёт 9♠: 3 карты = 3 игрока → закрытие ПО СЧЁТУ (R-5.5); Вера открывает
  //    следующий (R-5.7). Вера: 3→2 (auto).
  {
    kind: 'auto',
    delayMs: DELAY,
    events: [
      { type: 'cardPlayed', seat: 2, card: c(9, '♠') },
      { type: 'conClosed', by: 2 },
      { type: 'conSwept', cards: [c(6, '♠'), c(8, '♠'), c(9, '♠')] },
    ],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [],
        discard: 8,
        turn: 2,
        opponents: opp(3, 2),
      },
      [],
    ),
  },
  // 9. Вера заходит 10♦ (козырь), оставаясь с 1 картой и НЕ объявив «Одна карта!»
  //    (нарушение R-6.1a). Ход к вам: бить нечем (10♦ старше 7♦, клуба не бьёт козырь) →
  //    только «Взять низ». Открыто ШУХ-окно на Веру (Ш-11, R-6.2) (auto).
  {
    kind: 'auto',
    delayMs: DELAY,
    events: [{ type: 'cardPlayed', seat: 2, card: c(10, '♦') }],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [{ card: c(10, '♦'), by: 2 }],
        discard: 8,
        turn: 0,
        opponents: opp(3, 1),
      },
      [{ type: 'takeBottomAndPass' }, { type: 'claimShukh', target: 2, code: 11 }],
    ),
  },
  // 10. Вы жмёте «ШУХ!» на Веру. Вера оспаривает (R-8.6) → голосование (resolved:false).
  {
    kind: 'await',
    expect: { type: 'claimShukh', target: 2, code: 11 },
    events: [],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [{ card: c(10, '♦'), by: 2 }],
        discard: 8,
        turn: 0,
        opponents: opp(3, 1),
      },
      [],
      {
        claimant: 0,
        target: 2,
        code: 11,
        votes: [{ seat: 1, up: true }], // судит третий за столом — Боря (R-8.9)
        outcome: 'upheld',
        resolved: false,
      },
    ),
  },
  // 11. Голосование завершено: большинство подтвердило ШУХ (W2-7). ShukhAssessed (auto).
  {
    kind: 'auto',
    delayMs: DELAY,
    events: [{ type: 'shukhAssessed', offender: 2, code: 11 }],
    snapshot: base(
      {
        hand: [c(14, '♣'), c(7, '♦')],
        table: [{ card: c(10, '♦'), by: 2 }],
        discard: 8,
        turn: 0,
        opponents: opp(3, 1),
      },
      [],
      {
        claimant: 0,
        target: 2,
        code: 11,
        votes: [{ seat: 1, up: true }],
        outcome: 'upheld',
        resolved: true,
      },
    ),
  },
  // 12. Оплата (R-8.1): вы отдаёте 14♣ (I-2 — не последнюю, у вас 2), Боря отдаёт 5♣.
  //     Карты уходят в ШУХ-зону Веры лицом вниз (I-3, не в руку). Вера pending 0→2;
  //     Боря 3→2. Ваша рука 2→1 → у вас загорается «Одна карта!». Модалка закрывается.
  //     Ход у вас (R-8.5): бить 10♦ нечем → «Взять низ» (auto).
  {
    kind: 'auto',
    delayMs: DELAY,
    events: [
      { type: 'shukhPaid', offender: 2, from: 0, card: c(14, '♣') },
      { type: 'shukhPaid', offender: 2, from: 1, card: c(5, '♣') },
    ],
    snapshot: base(
      {
        hand: [c(7, '♦')],
        table: [{ card: c(10, '♦'), by: 2 }],
        discard: 8,
        turn: 0,
        opponents: opp(2, 1, 2),
      },
      [{ type: 'takeBottomAndPass' }],
    ),
  },
```

- [ ] **Step 4: Прогнать интеграционный тест**

Run: `cd web && npx vitest run src/fixtures/scenario.test.ts`
Expected: PASS.

- [ ] **Step 5: Прогнать гейт**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: PASS.

- [ ] **Step 6: Визуальная проверка в браузере (claude-in-chrome)**

Запустить dev-сервер и пройти сценарий глазами:

```bash
cd web && npm run dev
```

Через штатный браузерный инструмент (claude-in-chrome) открыть `/room/DEMO/table` и проверить:
- легальные карты подсвечены/кликабельны, нелегальные — притушены;
- выбор поднимает карту, второй клик / «Сходить» — ход; фантомного подъёма после хода НЕТ;
- в Кон-B открывается «ШУХ!»; клик → модалка голосования → исход «подтверждён»;
- анимация оплаты: карта уходит из руки, ШУХ-зона Веры растёт до 2;
- после оплаты у вас 1 карта → пульсирует «Одна карта!»; клик гасит пульс.

Зафиксировать результат в леджере (что подтверждено / что осталось).

- [ ] **Step 7: Commit**

```bash
git add web/src/fixtures/scenario.ts web/src/fixtures/scenario.test.ts
git commit -m "feat(web): extend demo scenario — Ш-11 catch window, R-8.6 vote, shukh payment

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Финальное ревью ветки

После Task 7 — whole-branch ревью на самой сильной модели (spec + quality), сверить с §5/§6 спеки,
проверить чистоту гейта на всей ветке. Зафиксировать в леджере статус и отложенные follow-ups.

---

## Self-Review (сверка плана со спекой §5/§6)

**Покрытие спеки:**
- §5 «Подсветка легальных ходов из snapshot.legal, нелегальные притушены/некликабельны» → **Task 2**.
- §5 «ActionBar по легальности» → **Task 3** (canConfirm/canTakeBottom), **Task 5** (canShukh).
- §5 «Выбор + подтверждение; повторный клик/кнопка» → **Task 3**.
- §5 «убрать позиционный selectedIndex → cardKey (фикс фантомного подъёма)» → **Task 2**.
- §5 «A11y onKeyDown (Enter/Space), видимый фокус» → **Task 2** (реализация), **Task 3** (тест).
- §6 «ШУХ-зоны у мест (счётчик, I-3; своя при ShukhTakeable)» → **Task 4** (takeable = `isShukhTakeable` из legal).
- §6 «кнопки «ШУХ!»/«Одна карта!» вживую» → **Task 5**.
- §6 «модалка голосования/оспаривания R-8.6, голоса ботов скриптованы» → **Task 6** (+ данные из сценария Task 7).
- §4/§6 «анимация ShukhPaid — карта уезжает в ШУХ-зону нарушителя» → **Task 4** (зона + AnimatePresence прирост), **Task 7** (события `shukhPaid` + визуальная проверка).
- §6 «расширить демо шагами ШУХ-окна и голосования» → **Task 7**.
- Отложенные minors: ветки `actionsEqual` (**Task 1**), events keep-last assert (**Task 1**), `Step` как discriminated union с обязательным `expect` (**Task 1**).

**Заметки для владельца (решения, вынесенные на ревью плана):**
1. **«Одна карта!»** смоделирована клиент-локально (нет игрового действия в движке); подробности — в Task 5. Забыл→Ш-11 демонстрируется на боте (Task 7), не интерактивной развилкой.
2. **Свою ШУХ-зону-взятие** (`takeShukhCards`) в линейное демо не встраивали (вы — предъявитель); покрыто тестами (Task 4). Могу добавить живой «хвост» со взятием — по желанию.
3. **ShukhPaid**: ШУХ-зоны рубашкой (I-3), поэтому оплата анимируется как «карта покидает руку + стопка зоны прирастает» (без FLIP-полёта конкретной карты в скрытую стопку). Если хочется именно полёт — обсудим отдельным правилом анимации.
4. **`ShukhVote` на `GameSnapshot`, не на `SeatView`** — чтобы не ломать миррор движка (W-3); это честный forward-DTO для Спеца 2.

**Проверка плейсхолдеров:** нет TBD/«добавить обработку ошибок»/«тесты для вышеуказанного» — весь код и тесты приведены дословно.

**Согласованность типов:** `ShukhVote`, `claimShukhInLegal`, `isShukhTakeable`, `selectShukhVote`, пропы `Hand`/`ActionBar`/`ShukhZone`/`ShukhVoteModal` — имена и сигнатуры единообразны между задачами, где потребитель ссылается на продукт предыдущей задачи.
