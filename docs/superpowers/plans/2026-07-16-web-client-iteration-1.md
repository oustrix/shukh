# Веб-клиент «Шух» — итерация 1 (каркас) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Поднять каркас браузерного клиента «Шух» — слоистый (contract → transport → store → ui), работающий на фейковом транспорте и фикстуре, с тремя экранами (Вход → Лобби → Стол).

**Architecture:** Слой 3 по `docs/architecture.md` (D-6). UI не знает про сеть: транспорт спрятан за интерфейсом `Transport`, сейчас его реализует мок на фикстуре; появление WebSocket-адаптера Спеца 2 = новая реализация `Transport`, без правок `ui/`/`store/`. Состояние — Zustand-стор, куда транспорт пушит снапшоты/события.

**Tech Stack:** Vite 6 + React 19 + TypeScript 5.7 (strict), react-router-dom 7, zustand 5, стили — CSS Modules + CSS-переменные, тесты — Vitest 3 + @testing-library/react 16 (jsdom). Спека: `docs/superpowers/specs/2026-07-16-web-client-iteration-1-design.md`.

## Global Constraints

- Всё живёт в каталоге `web/` в корне репозитория (worktree). Все `npm`-команды выполняются **из `web/`**.
- TS-типы в `src/contract/types.ts` — **ручные зеркала** `engine/*.go` (W-3). Каждую группу помечать комментарием `// зеркало engine/<file>.go — синхронизировать вручную`. Точное JSON-кодирование согласуем с DTO сервера Спеца 2 позже.
- Ни в одном компоненте `ui/`/`store/` **нельзя импортировать `transport/mock`** напрямую — только через инъекцию (шов `Transport`, W-2). Единственная точка, где мок связывается со стором — `src/store/game.ts`.
- Карты рисуем **SVG-компонентом**, без растровых картинок и без внешних карточных библиотек (W-7).
- Вне итерации (YAGNI, не реализовывать): реальный WebSocket/реконнект, генерация типов из Go, реальный `LegalActions`, голосование R-8.6, богатые анимации раздачи/боя, аккаунты.
- TDD: сначала падающий тест, потом минимальная реализация. Частые коммиты.
- Трейлер каждого коммита:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

```
web/
  index.html                     # точка входа Vite
  package.json  tsconfig.json  tsconfig.node.json  vite.config.ts
  eslint.config.js  .prettierrc  .gitignore
  src/
    main.tsx                     # bootstrap: BrowserRouter + App, импорт theme.css
    App.tsx                      # маршруты: / , /room/:code , /room/:code/table
    vite-env.d.ts                # типы vite/client (в т.ч. *.module.css)
    setupTests.ts                # @testing-library/jest-dom
    contract/
      types.ts                   # Card/View/OpponentView/Action/GameEvent + helpers (зеркала engine)
      transport.ts               # interface Transport (шов к Спецу 2)
    fixtures/
      game.ts                    # один GameSnapshot: seats(имена) + view(партия) для Лобби и Стола
    transport/
      mock.ts                    # createMockTransport(snapshot): Transport
    store/
      game.ts                    # createGameStore(transport) + singleton useGameStore(мок)
    ui/
      kit/
        theme.css                # дизайн-токены (CSS-переменные)
        Button.tsx  Button.module.css
      table/
        cardText.ts              # rankLabel/cardLabel/isRedSuit
        Card.tsx  Card.module.css
        Hand.tsx  Con.tsx  OpponentSeat.tsx  ActionBar.tsx  (+ *.module.css)
      screens/
        Join.tsx  Lobby.tsx  Table.tsx  (+ *.module.css)
```

**Ответственности:** `contract/` — формы данных и шов; `transport/` — источник данных (сейчас мок); `store/` — единственный держатель состояния для UI; `ui/kit` — примитивы+токены; `ui/table` — куски стола; `ui/screens` — экраны/маршруты.

---

### Task 1: Скелет проекта + тест-харнес + тулинг

Поднять `web/` с Vite/React/TS, Vitest, ESLint/Prettier; App — временная заглушка `<h1>Шух</h1>`, покрытая smoke-тестом. Конфиги входят в эту задачу (без них нет тестируемого дифа).

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/index.html`, `web/eslint.config.js`, `web/.prettierrc`, `web/.gitignore`, `web/src/vite-env.d.ts`, `web/src/setupTests.ts`, `web/src/main.tsx`, `web/src/App.tsx`
- Test: `web/src/App.test.tsx`

**Interfaces:**
- Produces: `App` (именованный экспорт из `src/App.tsx`) — React-компонент без пропсов.

- [ ] **Step 1: Создать `web/package.json`**

```json
{
  "name": "shukh-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit",
    "lint": "eslint .",
    "format": "prettier --write src"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.1.0",
    "zustand": "^5.0.0"
  },
  "devDependencies": {
    "@eslint/js": "^9.17.0",
    "@testing-library/jest-dom": "^6.6.0",
    "@testing-library/react": "^16.1.0",
    "@testing-library/user-event": "^14.5.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "eslint": "^9.17.0",
    "eslint-plugin-react-hooks": "^5.1.0",
    "eslint-plugin-react-refresh": "^0.4.16",
    "globals": "^15.14.0",
    "jsdom": "^25.0.0",
    "prettier": "^3.4.0",
    "typescript": "^5.7.0",
    "typescript-eslint": "^8.19.0",
    "vite": "^6.0.0",
    "vitest": "^3.0.0"
  }
}
```

- [ ] **Step 2: Создать `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vite/client", "vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Создать `web/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 4: Создать `web/vite.config.ts`**

```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/setupTests.ts',
  },
})
```

- [ ] **Step 5: Создать `web/index.html`**

```html
<!doctype html>
<html lang="ru">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Шух</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Создать `web/eslint.config.js`, `web/.prettierrc`, `web/.gitignore`, `web/src/vite-env.d.ts`, `web/src/setupTests.ts`**

`web/eslint.config.js`:
```js
import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'

export default tseslint.config(
  { ignores: ['dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: { ecmaVersion: 2022, globals: globals.browser },
    plugins: { 'react-hooks': reactHooks, 'react-refresh': reactRefresh },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    },
  },
)
```

`web/.prettierrc`:
```json
{ "semi": false, "singleQuote": true, "printWidth": 100 }
```

`web/.gitignore`:
```
node_modules
dist
*.local
```

`web/src/vite-env.d.ts`:
```ts
/// <reference types="vite/client" />
```

`web/src/setupTests.ts`:
```ts
import '@testing-library/jest-dom'
```

- [ ] **Step 7: Создать `web/src/App.tsx` (временная заглушка) и `web/src/main.tsx`**

`web/src/App.tsx`:
```tsx
export function App() {
  return <h1>Шух</h1>
}
```

`web/src/main.tsx`:
```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 8: Написать падающий smoke-тест `web/src/App.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { App } from './App'

test('App рендерит заголовок игры', () => {
  render(<App />)
  expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
})
```

- [ ] **Step 9: Установить зависимости и прогнать тест — убедиться, что падает по отсутствию модулей/сборки**

Run (из `web/`): `npm install && npm test`
Expected: сначала FAIL (или падение установки, если сеть) → после `npm install` тест PASS. Если тест уже зелёный после install — норм, заглушка тривиальна; главное, харнес работает.

- [ ] **Step 10: Прогнать полный гейт**

Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное; `npm test` — 1 passed.

- [ ] **Step 11: Commit**

```bash
git add web
git commit -m "feat(web): Vite+React+TS scaffold with Vitest/ESLint harness

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Слой контракта — типы движка + интерфейс Transport

**Files:**
- Create: `web/src/contract/types.ts`, `web/src/contract/transport.ts`
- Test: `web/src/contract/types.test.ts`

**Interfaces:**
- Produces:
  - `Suit = '♠' | '♥' | '♦' | '♣'`; `Rank = number`; `interface Card { suit: Suit; rank: Rank }`
  - `SeatID = number`; `Phase = 'playing' | 'finished'`; `EnforcementMode = 'guard' | 'middle' | 'culture'`
  - `interface RuleSet { deckSize: 36 | 52; podkladkaSnizu: boolean; jokers: boolean }`
  - `interface TableCard { card: Card; by: SeatID }`
  - `interface OpponentView { seat: SeatID; handCount: number; shukhPending: number; live: boolean }`
  - `interface View { rules; mode; phase; you; turn; hand: Card[]; shukhPending: number; opponents: OpponentView[]; table: TableCard[]; discard: number; talon: number; live: Record<number, boolean>; finish: SeatID[] }`
  - `interface SeatMeta { seat: SeatID; name: string; ready: boolean }`
  - `interface GameSnapshot { roomCode: string; seats: SeatMeta[]; view: View | null }`
  - `type ShukhCode = 2 | 3 | 11 | 12`
  - `type Action` (union, поле `type`); `type GameEvent` (union, поле `type`)
  - `function isYourTurn(view: View): boolean`
  - `interface Transport { subscribe(onSnapshot, onEvent): () => void; send(action: Action): void }`

- [ ] **Step 1: Написать падающий тест `web/src/contract/types.test.ts`**

```ts
import { isYourTurn, type View } from './types'

const baseView: View = {
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
```

- [ ] **Step 2: Прогнать тест — убедиться, что падает**

Run (из `web/`): `npx vitest run src/contract/types.test.ts`
Expected: FAIL — `Cannot find module './types'`.

- [ ] **Step 3: Создать `web/src/contract/types.ts`**

```ts
// Ручные TS-зеркала типов движка. СИНХРОНИЗИРОВАТЬ ВРУЧНУЮ (W-3).
// Точное JSON-кодирование согласуется с DTO сервера Спеца 2, когда он появится.

// зеркало engine/card.go — синхронизировать вручную
export type Suit = '♠' | '♥' | '♦' | '♣' // Spades|Hearts|Diamonds|Clubs; ♦ — козырь (R-2.5)
export type Rank = number // 2..14; 11=J 12=Q 13=K 14=A (R-2.2)
export interface Card {
  suit: Suit
  rank: Rank
}

// зеркало engine/state.go — синхронизировать вручную
export type SeatID = number
export type Phase = 'playing' | 'finished'
export type EnforcementMode = 'guard' | 'middle' | 'culture'
export type ShukhCode = 2 | 3 | 11 | 12
export interface RuleSet {
  deckSize: 36 | 52
  podkladkaSnizu: boolean
  jokers: boolean
}
export interface TableCard {
  card: Card
  by: SeatID
}

// зеркало engine/view.go (per-seat проекция, D-9) — синхронизировать вручную
export interface OpponentView {
  seat: SeatID
  handCount: number
  shukhPending: number
  live: boolean
}
export interface View {
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

// Метаданные комнаты (Слой 1) — имена/готовность НЕ входят в engine.View.
export interface SeatMeta {
  seat: SeatID
  name: string
  ready: boolean
}
export interface GameSnapshot {
  roomCode: string
  seats: SeatMeta[]
  view: View | null // null в лобби (партия ещё не началась)
}

// зеркало engine/action.go — синхронизировать вручную
export type Action =
  | { type: 'playCard'; card: Card }
  | { type: 'takeBottomAndPass' }
  | { type: 'podkladkaWest' }
  | { type: 'claimShukh'; target: SeatID; code: ShukhCode }
  | { type: 'giveShukhCard'; card: Card }
  | { type: 'takeShukhCards'; seat: SeatID }

// зеркало engine/event.go + state.go — синхронизировать вручную
export type GameEvent =
  | { type: 'gameStarted'; turn: SeatID }
  | { type: 'cardPlayed'; seat: SeatID; card: Card }
  | { type: 'conClosed'; by: SeatID }
  | { type: 'conSwept'; cards: Card[] }
  | { type: 'playerFinished'; seat: SeatID; place: number }
  | { type: 'gameFinished'; finish: SeatID[] }
  | { type: 'cardsTaken'; seat: SeatID; cards: Card[] }
  | { type: 'podkladkaPlayed'; seat: SeatID; eater: SeatID }
  | { type: 'turnSkipped'; seat: SeatID }
  | { type: 'shukhAssessed'; offender: SeatID; code: ShukhCode }
  | { type: 'actionReverted'; seat: SeatID }
  | { type: 'shukhPaid'; offender: SeatID; from: SeatID; card: Card }
  | { type: 'shukhCardsTaken'; seat: SeatID; cards: Card[] }

// Хелпер уровня контракта (используется UI).
export function isYourTurn(view: View): boolean {
  return view.turn === view.you
}
```

- [ ] **Step 4: Создать `web/src/contract/transport.ts`**

```ts
import type { Action, GameEvent, GameSnapshot } from './types'

// Шов между UI и источником данных (W-2). Сейчас реализуется моком на фикстуре;
// позже — WebSocket-адаптером Спеца 2, БЕЗ правок ui/ и store/.
export interface Transport {
  // Подписка на пуш-обновления. Возвращает функцию отписки.
  subscribe(onSnapshot: (s: GameSnapshot) => void, onEvent: (e: GameEvent) => void): () => void
  // Отправка действия игрока (§5 / ШУХи).
  send(action: Action): void
}
```

- [ ] **Step 5: Прогнать тест — PASS**

Run (из `web/`): `npx vitest run src/contract/types.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 6: Гейт + Commit**

Run (из `web/`): `npm run typecheck && npm test`
```bash
git add web/src/contract
git commit -m "feat(web): contract layer — engine type mirrors + Transport seam

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Фикстура + мок-транспорт

**Files:**
- Create: `web/src/fixtures/game.ts`, `web/src/transport/mock.ts`
- Test: `web/src/transport/mock.test.ts`

**Interfaces:**
- Consumes: `GameSnapshot`, `Transport`, `Action` (Task 2).
- Produces:
  - `const gameSnapshot: GameSnapshot` — 3 игрока (you=0), середина партии.
  - `function createMockTransport(snapshot: GameSnapshot): Transport & { sent: Action[] }` — эмитит снапшот **синхронно** при `subscribe`; `send` пишет в `sent` и логирует.

- [ ] **Step 1: Создать фикстуру `web/src/fixtures/game.ts`**

```ts
import type { GameSnapshot } from '../contract/types'

// Один снапшот на итерацию: seats (имена/готовность) для Лобби + view для Стола.
export const gameSnapshot: GameSnapshot = {
  roomCode: 'DEMO',
  seats: [
    { seat: 0, name: 'Аня', ready: true },
    { seat: 1, name: 'Боря', ready: true },
    { seat: 2, name: 'Вера', ready: true },
  ],
  view: {
    rules: { deckSize: 36, podkladkaSnizu: false, jokers: false },
    mode: 'middle',
    phase: 'playing',
    you: 0,
    turn: 0,
    hand: [
      { suit: '♦', rank: 9 }, // 9♦ — заход первого кона (R-5.1)
      { suit: '♥', rank: 12 }, // Дама♥
      { suit: '♠', rank: 6 },
      { suit: '♣', rank: 14 }, // A♣
      { suit: '♦', rank: 7 },
    ],
    shukhPending: 0,
    opponents: [
      { seat: 1, handCount: 5, shukhPending: 1, live: true },
      { seat: 2, handCount: 4, shukhPending: 0, live: true },
    ],
    table: [
      { card: { suit: '♠', rank: 8 }, by: 2 },
      { card: { suit: '♠', rank: 11 }, by: 0 }, // J♠ поверх 8♠
    ],
    discard: 6,
    talon: 0,
    live: { 0: true, 1: true, 2: true },
    finish: [],
  },
}
```

- [ ] **Step 2: Написать падающий тест `web/src/transport/mock.test.ts`**

```ts
import { createMockTransport } from './mock'
import { gameSnapshot } from '../fixtures/game'
import type { GameSnapshot } from '../contract/types'

test('subscribe синхронно доставляет снапшот', () => {
  const t = createMockTransport(gameSnapshot)
  let got: GameSnapshot | null = null
  t.subscribe((s) => (got = s), () => {})
  expect(got).toBe(gameSnapshot)
})

test('send записывает действие в sent', () => {
  const t = createMockTransport(gameSnapshot)
  t.send({ type: 'takeBottomAndPass' })
  expect(t.sent).toEqual([{ type: 'takeBottomAndPass' }])
})

test('subscribe возвращает функцию отписки; снапшот приходит ровно один раз (emit-once)', () => {
  const t = createMockTransport(gameSnapshot)
  let calls = 0
  const unsub = t.subscribe(() => (calls += 1), () => {})
  expect(typeof unsub).toBe('function')
  expect(calls).toBe(1) // мок пушит один раз при подписке; повторных пушей нет
  expect(() => unsub()).not.toThrow()
})
```

- [ ] **Step 3: Прогнать — убедиться, что падает**

Run (из `web/`): `npx vitest run src/transport/mock.test.ts`
Expected: FAIL — `Cannot find module './mock'`.

- [ ] **Step 4: Создать `web/src/transport/mock.ts`**

```ts
import type { Transport } from '../contract/transport'
import type { Action, GameSnapshot } from '../contract/types'

// Фейковый транспорт на фикстуре. Синхронный пуш снапшота при подписке —
// простая, детерминированная модель для UI и тестов. send() копит действия.
export function createMockTransport(snapshot: GameSnapshot): Transport & { sent: Action[] } {
  const sent: Action[] = []
  return {
    sent,
    subscribe(onSnapshot) {
      onSnapshot(snapshot)
      return () => {}
    },
    send(action) {
      sent.push(action)
      console.debug('[mock] send', action)
    },
  }
}
```

- [ ] **Step 5: Прогнать — PASS**

Run (из `web/`): `npx vitest run src/transport/mock.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 6: Гейт + Commit**

Run (из `web/`): `npm run typecheck && npm test`
```bash
git add web/src/fixtures web/src/transport
git commit -m "feat(web): fixture snapshot + mock transport

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Zustand-стор

**Files:**
- Create: `web/src/store/game.ts`
- Test: `web/src/store/game.test.ts`

**Interfaces:**
- Consumes: `Transport`, `Action`, `GameEvent`, `GameSnapshot` (Task 2), `createMockTransport`, `gameSnapshot` (Task 3).
- Produces:
  - `interface GameState { snapshot: GameSnapshot | null; events: GameEvent[]; play: (a: Action) => void }`
  - `function createGameStore(transport: Transport)` — zustand-хук; подписывается на транспорт после создания стора.
  - `const useGameStore` — singleton поверх `createMockTransport(gameSnapshot)`.

- [ ] **Step 1: Написать падающий тест `web/src/store/game.test.ts`**

```ts
import { vi } from 'vitest'
import { createGameStore } from './game'
import type { Transport } from '../contract/transport'
import type { GameEvent, GameSnapshot } from '../contract/types'
import { gameSnapshot } from '../fixtures/game'

function fakeTransport() {
  let onSnap: ((s: GameSnapshot) => void) | undefined
  let onEv: ((e: GameEvent) => void) | undefined
  const send = vi.fn()
  const transport: Transport = {
    subscribe(s, e) {
      onSnap = s
      onEv = e
      return () => {}
    },
    send,
  }
  return {
    transport,
    send,
    emitSnapshot: (s: GameSnapshot) => onSnap!(s),
    emitEvent: (e: GameEvent) => onEv!(e),
  }
}

test('стор стартует пустым и принимает снапшот из транспорта', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  expect(store.getState().snapshot).toBeNull()
  f.emitSnapshot(gameSnapshot)
  expect(store.getState().snapshot).toBe(gameSnapshot)
})

test('play пробрасывает действие в transport.send', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  store.getState().play({ type: 'takeBottomAndPass' })
  expect(f.send).toHaveBeenCalledWith({ type: 'takeBottomAndPass' })
})

test('события копятся в events', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: 9 } })
  expect(store.getState().events).toHaveLength(1)
})
```

- [ ] **Step 2: Прогнать — убедиться, что падает**

Run (из `web/`): `npx vitest run src/store/game.test.ts`
Expected: FAIL — `Cannot find module './game'`.

- [ ] **Step 3: Создать `web/src/store/game.ts`**

```ts
import { create } from 'zustand'
import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { createMockTransport } from '../transport/mock'
import { gameSnapshot } from '../fixtures/game'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  play: (action: Action) => void
}

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
    (event) => store.setState((s) => ({ events: [...s.events, event] })),
  )
  return store
}

// Singleton приложения: пока на моке с фикстурой. Замена мока на ws.ts (Спец 2)
// не затрагивает компоненты — они читают только этот хук.
export const useGameStore = createGameStore(createMockTransport(gameSnapshot))
```

- [ ] **Step 4: Прогнать — PASS**

Run (из `web/`): `npx vitest run src/store/game.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Гейт + Commit**

Run (из `web/`): `npm run typecheck && npm test`
```bash
git add web/src/store
git commit -m "feat(web): zustand game store fed by transport

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Дизайн-токены + SVG-компонент карты

**Files:**
- Create: `web/src/ui/kit/theme.css`, `web/src/ui/table/cardText.ts`, `web/src/ui/table/Card.tsx`, `web/src/ui/table/Card.module.css`
- Test: `web/src/ui/table/Card.test.tsx`, `web/src/ui/table/cardText.test.ts`

**Interfaces:**
- Consumes: `Card` (тип), `Suit` (Task 2).
- Produces:
  - `cardText.ts`: `rankLabel(rank: number): string`; `cardLabel(card: Card): string`; `isRedSuit(suit: Suit): boolean`
  - `Card.tsx`: `function Card(props: { card?: CardT; faceDown?: boolean; selected?: boolean; onClick?: () => void })` — SVG; `data-testid="card-face"` для открытой, `"card-back"` для закрытой.

- [ ] **Step 1: Написать падающий тест `web/src/ui/table/cardText.test.ts`**

```ts
import { rankLabel, cardLabel, isRedSuit } from './cardText'

test('rankLabel: числа как есть, фигуры как буквы', () => {
  expect(rankLabel(9)).toBe('9')
  expect(rankLabel(12)).toBe('Q')
  expect(rankLabel(14)).toBe('A')
})

test('cardLabel: ранг + масть', () => {
  expect(cardLabel({ suit: '♥', rank: 12 })).toBe('Q♥')
})

test('isRedSuit: черви и бубны — красные', () => {
  expect(isRedSuit('♥')).toBe(true)
  expect(isRedSuit('♦')).toBe(true)
  expect(isRedSuit('♠')).toBe(false)
  expect(isRedSuit('♣')).toBe(false)
})
```

- [ ] **Step 2: Прогнать — FAIL (нет модуля). Создать `web/src/ui/table/cardText.ts`**

```ts
import type { Card, Suit } from '../../contract/types'

const FACE: Record<number, string> = { 11: 'J', 12: 'Q', 13: 'K', 14: 'A' }

export function rankLabel(rank: number): string {
  return FACE[rank] ?? String(rank)
}

export function cardLabel(card: Card): string {
  return rankLabel(card.rank) + card.suit
}

export function isRedSuit(suit: Suit): boolean {
  return suit === '♥' || suit === '♦'
}
```

Run (из `web/`): `npx vitest run src/ui/table/cardText.test.ts` → Expected: PASS (3 tests).

- [ ] **Step 3: Создать токены `web/src/ui/kit/theme.css`**

```css
:root {
  --felt: #16653b;
  --felt-dark: #0f4a2b;
  --card-bg: #fffdf7;
  --card-frame: #d9d2be;
  --card-back: #7c1f2b;
  --suit-red: #c62828;
  --suit-black: #1b1b1b;
  --accent: #e0b64a;
  --text: #f5f3ec;
  --radius: 8px;
  --shadow: 0 2px 6px rgba(0, 0, 0, 0.35);
  --gap: 8px;
  --font: system-ui, 'Segoe UI', Roboto, sans-serif;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: var(--font);
  color: var(--text);
  background: var(--felt-dark);
}
```

- [ ] **Step 4: Написать падающий тест `web/src/ui/table/Card.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { Card } from './Card'

test('открытая карта показывает ранг и масть (Дама♥)', () => {
  render(<Card card={{ suit: '♥', rank: 12 }} />)
  const face = screen.getByTestId('card-face')
  expect(face).toHaveTextContent('Q')
  expect(face).toHaveTextContent('♥')
})

test('закрытая карта рендерит рубашку', () => {
  render(<Card card={{ suit: '♥', rank: 12 }} faceDown />)
  expect(screen.getByTestId('card-back')).toBeInTheDocument()
})
```

- [ ] **Step 5: Прогнать — FAIL. Создать `web/src/ui/table/Card.tsx` и `Card.module.css`**

`web/src/ui/table/Card.tsx`:
```tsx
import type { Card as CardT } from '../../contract/types'
import { rankLabel, isRedSuit, cardLabel } from './cardText'
import styles from './Card.module.css'

interface CardProps {
  card?: CardT
  faceDown?: boolean
  selected?: boolean
  onClick?: () => void
}

export function Card({ card, faceDown, selected, onClick }: CardProps) {
  const hidden = faceDown || !card
  const red = card ? isRedSuit(card.suit) : false
  const cls = [styles.card, selected ? styles.selected : '', onClick ? styles.clickable : '']
    .filter(Boolean)
    .join(' ')
  return (
    <svg
      viewBox="0 0 60 84"
      className={cls}
      role={onClick ? 'button' : 'img'}
      aria-label={card && !hidden ? cardLabel(card) : 'закрытая карта'}
      tabIndex={onClick ? 0 : undefined}
      onClick={onClick}
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
    </svg>
  )
}
```

`web/src/ui/table/Card.module.css`:
```css
.card {
  width: 60px;
  height: 84px;
  filter: drop-shadow(var(--shadow));
}
.clickable {
  cursor: pointer;
}
.frame {
  fill: var(--card-bg);
  stroke: var(--card-frame);
  stroke-width: 1;
}
.back {
  fill: var(--card-back);
}
.red text {
  fill: var(--suit-red);
}
.black text {
  fill: var(--suit-black);
}
.selected {
  transform: translateY(-10px);
  transition: transform 0.12s ease;
}
```

- [ ] **Step 6: Прогнать оба теста — PASS**

Run (из `web/`): `npx vitest run src/ui/table/`
Expected: PASS (cardText 3 + Card 2).

- [ ] **Step 7: Гейт + Commit**

Run (из `web/`): `npm run typecheck && npm test`
```bash
git add web/src/ui
git commit -m "feat(web): design tokens + SVG Card component

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Куски стола — Button, Hand, Con, OpponentSeat, ActionBar

**Files:**
- Create: `web/src/ui/kit/Button.tsx`, `web/src/ui/kit/Button.module.css`, `web/src/ui/table/Hand.tsx`, `web/src/ui/table/Con.tsx`, `web/src/ui/table/OpponentSeat.tsx`, `web/src/ui/table/ActionBar.tsx`, `web/src/ui/table/Table.module.css`
- Test: `web/src/ui/table/Hand.test.tsx`, `web/src/ui/table/OpponentSeat.test.tsx`, `web/src/ui/table/ActionBar.test.tsx`

**Interfaces:**
- Consumes: `Card` (Task 5), `Card`/`TableCard`/`OpponentView` типы (Task 2).
- Produces:
  - `Button(props: React.ComponentProps<'button'>)`
  - `Hand(props: { cards: CardT[]; selectedIndex: number | null; onSelect: (i: number) => void })`
  - `Con(props: { table: TableCard[] })`
  - `OpponentSeat(props: { name: string; opponent: OpponentView })`
  - `ActionBar(props: { yourTurn: boolean; onShukh: () => void; onOneCard: () => void; onTakeBottom: () => void })`

- [ ] **Step 1: Создать `web/src/ui/kit/Button.tsx` и `Button.module.css`**

`web/src/ui/kit/Button.tsx`:
```tsx
import type { ComponentProps } from 'react'
import styles from './Button.module.css'

export function Button({ className, ...props }: ComponentProps<'button'>) {
  return <button className={[styles.button, className].filter(Boolean).join(' ')} {...props} />
}
```

`web/src/ui/kit/Button.module.css`:
```css
.button {
  padding: 8px 16px;
  border: none;
  border-radius: var(--radius);
  background: var(--accent);
  color: #201a08;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
```

- [ ] **Step 2: Написать падающий тест `web/src/ui/table/Hand.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Hand } from './Hand'
import type { Card } from '../../contract/types'

const cards: Card[] = [
  { suit: '♦', rank: 9 },
  { suit: '♥', rank: 12 },
]

test('Hand рендерит по карте на каждую в руке', () => {
  render(<Hand cards={cards} selectedIndex={null} onSelect={() => {}} />)
  expect(screen.getAllByTestId('card-face')).toHaveLength(2)
})

test('клик по карте вызывает onSelect с индексом', async () => {
  const onSelect = vi.fn()
  render(<Hand cards={cards} selectedIndex={null} onSelect={onSelect} />)
  await userEvent.click(screen.getByRole('button', { name: '9♦' }))
  expect(onSelect).toHaveBeenCalledWith(0)
})
```

- [ ] **Step 3: Прогнать — FAIL. Создать `web/src/ui/table/Hand.tsx`**

```tsx
import type { Card as CardT } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface HandProps {
  cards: CardT[]
  selectedIndex: number | null
  onSelect: (index: number) => void
}

export function Hand({ cards, selectedIndex, onSelect }: HandProps) {
  return (
    <div className={styles.hand} data-testid="hand">
      {cards.map((c, i) => (
        <Card key={i} card={c} selected={i === selectedIndex} onClick={() => onSelect(i)} />
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Создать `web/src/ui/table/Con.tsx`**

```tsx
import type { TableCard } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface ConProps {
  table: TableCard[]
}

export function Con({ table }: ConProps) {
  return (
    <div className={styles.con} data-testid="con">
      {table.length === 0 ? (
        <span className={styles.empty}>кон пуст</span>
      ) : (
        table.map((tc, i) => <Card key={i} card={tc.card} />)
      )}
    </div>
  )
}
```

- [ ] **Step 5: Написать падающий тест `web/src/ui/table/OpponentSeat.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { OpponentSeat } from './OpponentSeat'

test('показывает имя и число карт', () => {
  render(<OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }} />)
  expect(screen.getByText('Боря')).toBeInTheDocument()
  expect(screen.getByText(/5/)).toBeInTheDocument()
})

test('бейдж ШУХ показывается только при shukhPending > 0', () => {
  const { rerender } = render(
    <OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }} />,
  )
  expect(screen.queryByTestId('shukh-badge')).not.toBeInTheDocument()
  rerender(
    <OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 2, live: true }} />,
  )
  expect(screen.getByTestId('shukh-badge')).toHaveTextContent('2')
})
```

- [ ] **Step 6: Прогнать — FAIL. Создать `web/src/ui/table/OpponentSeat.tsx`**

```tsx
import type { OpponentView } from '../../contract/types'
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
      {opponent.shukhPending > 0 && (
        <div className={styles.shukhBadge} data-testid="shukh-badge" title="ожидает ШУХ-карт">
          ШУХ {opponent.shukhPending}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 7: Написать падающий тест `web/src/ui/table/ActionBar.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

const noop = () => {}

test('«Взять низ» отключена не в свой ход', () => {
  render(<ActionBar yourTurn={false} onShukh={noop} onOneCard={noop} onTakeBottom={noop} />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeDisabled()
})

test('«ШУХ!» кликается и зовёт onShukh', async () => {
  const onShukh = vi.fn()
  render(<ActionBar yourTurn onShukh={onShukh} onOneCard={noop} onTakeBottom={noop} />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(onShukh).toHaveBeenCalled()
})
```

- [ ] **Step 8: Прогнать — FAIL. Создать `web/src/ui/table/ActionBar.tsx`**

```tsx
import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface ActionBarProps {
  yourTurn: boolean
  onShukh: () => void
  onOneCard: () => void
  onTakeBottom: () => void
}

export function ActionBar({ yourTurn, onShukh, onOneCard, onTakeBottom }: ActionBarProps) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      <Button onClick={onShukh}>ШУХ!</Button>
      <Button onClick={onOneCard}>Одна карта!</Button>
      <Button onClick={onTakeBottom} disabled={!yourTurn}>
        Взять низ
      </Button>
    </div>
  )
}
```

- [ ] **Step 9: Создать раскладку `web/src/ui/table/Table.module.css`**

```css
.table {
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  gap: var(--gap);
  min-height: 100vh;
  padding: 16px;
  background: radial-gradient(circle at 50% 40%, var(--felt), var(--felt-dark));
}
.opponents {
  display: flex;
  justify-content: center;
  gap: 24px;
  flex-wrap: wrap;
}
.seat {
  text-align: center;
  min-width: 84px;
}
.seatName {
  font-weight: 600;
}
.seatCount {
  opacity: 0.85;
}
.shukhBadge {
  margin-top: 4px;
  display: inline-block;
  padding: 2px 6px;
  border-radius: var(--radius);
  background: var(--card-back);
  color: var(--text);
  font-size: 0.8em;
}
.con {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 4px;
  min-height: 90px;
}
.empty {
  opacity: 0.6;
}
.hand {
  display: flex;
  justify-content: center;
  gap: var(--gap);
  flex-wrap: wrap;
}
.actionBar {
  display: flex;
  justify-content: center;
  gap: var(--gap);
}
```

- [ ] **Step 10: Прогнать все тесты стола — PASS**

Run (из `web/`): `npx vitest run src/ui/table/`
Expected: PASS (Hand 2 + OpponentSeat 2 + ActionBar 2 + прежние Card/cardText).

- [ ] **Step 11: Гейт + Commit**

Run (из `web/`): `npm run typecheck && npm test`
```bash
git add web/src/ui
git commit -m "feat(web): table pieces — Button, Hand, Con, OpponentSeat, ActionBar

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Экраны и маршрутизация — Вход → Лобби → Стол

**Files:**
- Create: `web/src/ui/screens/Join.tsx`, `web/src/ui/screens/Lobby.tsx`, `web/src/ui/screens/Table.tsx`, `web/src/ui/screens/Screens.module.css`
- Modify: `web/src/App.tsx` (маршруты вместо заглушки), `web/src/main.tsx` (обернуть в `BrowserRouter`), `web/src/App.test.tsx` (переписать под маршруты)
- Test: `web/src/App.test.tsx`

**Interfaces:**
- Consumes: `useGameStore` (Task 4), `isYourTurn` (Task 2), `Hand`/`Con`/`OpponentSeat`/`ActionBar` (Task 6), `Button` (Task 6).
- Produces: `App` — `<Routes>` с путями `/`, `/room/:code`, `/room/:code/table`.

- [ ] **Step 1: Создать `web/src/ui/screens/Join.tsx`**

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Join() {
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const navigate = useNavigate()
  const canJoin = name.trim() !== '' && code.trim() !== ''
  return (
    <form
      className={styles.centered}
      onSubmit={(e) => {
        e.preventDefault()
        navigate(`/room/${code.trim()}`)
      }}
    >
      <h1>Шух</h1>
      <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
      <input
        aria-label="Код комнаты"
        placeholder="Код комнаты"
        value={code}
        onChange={(e) => setCode(e.target.value)}
      />
      <Button type="submit" disabled={!canJoin}>
        Войти
      </Button>
    </form>
  )
}
```

- [ ] **Step 2: Создать `web/src/ui/screens/Lobby.tsx`**

```tsx
import { useNavigate, useParams } from 'react-router-dom'
import { useGameStore } from '../../store/game'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Lobby() {
  const { code } = useParams()
  const seats = useGameStore((s) => s.snapshot?.seats ?? [])
  const navigate = useNavigate()
  return (
    <div className={styles.centered}>
      <h2>Комната {code}</h2>
      <ul data-testid="players" className={styles.players}>
        {seats.map((s) => (
          <li key={s.seat}>
            {s.name} {s.ready ? '✓' : '…'}
          </li>
        ))}
      </ul>
      <Button onClick={() => navigate(`/room/${code}/table`)}>Начать</Button>
    </div>
  )
}
```

- [ ] **Step 3: Создать `web/src/ui/screens/Table.tsx`**

```tsx
import { useState } from 'react'
import { isYourTurn } from '../../contract/types'
import { useGameStore } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ActionBar } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGameStore((s) => s.snapshot?.view ?? null)
  const seats = useGameStore((s) => s.snapshot?.seats ?? [])
  const play = useGameStore((s) => s.play)
  const [selected, setSelected] = useState<number | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameOf = (seat: number) => seats.find((s) => s.seat === seat)?.name ?? `Игрок ${seat}`

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
          /* объявление «Одна карта!» (§6) — заглушка до Спеца 2 */
        }}
        onTakeBottom={() => play({ type: 'takeBottomAndPass' })}
      />
      <Hand
        cards={view.hand}
        selectedIndex={selected}
        onSelect={(i) => {
          setSelected(i)
          play({ type: 'playCard', card: view.hand[i] })
        }}
      />
    </div>
  )
}
```

- [ ] **Step 4: Создать `web/src/ui/screens/Screens.module.css`**

```css
.centered {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 48px 16px;
}
.centered input {
  padding: 8px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--card-frame);
  font: inherit;
  min-width: 220px;
}
.players {
  list-style: none;
  padding: 0;
  text-align: center;
}
```

- [ ] **Step 5: Переписать `web/src/App.tsx` под маршруты**

```tsx
import { Routes, Route } from 'react-router-dom'
import { Join } from './ui/screens/Join'
import { Lobby } from './ui/screens/Lobby'
import { Table } from './ui/screens/Table'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Join />} />
      <Route path="/room/:code" element={<Lobby />} />
      <Route path="/room/:code/table" element={<Table />} />
    </Routes>
  )
}
```

- [ ] **Step 6: Обновить `web/src/main.tsx` — обернуть в `BrowserRouter` и импортировать токены**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { App } from './App'
import './ui/kit/theme.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
```

- [ ] **Step 7: Переписать `web/src/App.test.tsx` под маршруты**

```tsx
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { App } from './App'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

test('корень показывает экран входа', () => {
  renderAt('/')
  expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
  expect(screen.getByLabelText('Код комнаты')).toBeInTheDocument()
})

test('лобби показывает игроков комнаты из стора', () => {
  renderAt('/room/DEMO')
  const players = screen.getByTestId('players')
  expect(players).toHaveTextContent('Аня')
  expect(players).toHaveTextContent('Боря')
})

test('стол рендерит руку и мест соперников из снапшота', () => {
  renderAt('/room/DEMO/table')
  // 5 карт руки из фикстуры
  expect(screen.getAllByTestId('card-face').length).toBeGreaterThanOrEqual(5)
  expect(screen.getByTestId('action-bar')).toBeInTheDocument()
  expect(screen.getByTestId('seat-1')).toBeInTheDocument()
  expect(screen.getByTestId('seat-2')).toBeInTheDocument()
})
```

- [ ] **Step 8: Прогнать весь набор тестов — PASS**

Run (из `web/`): `npm test`
Expected: PASS — все файлы (contract, mock, store, cardText, Card, Hand, OpponentSeat, ActionBar, App).

- [ ] **Step 9: Полный гейт + ручная проверка dev-сервера**

Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное.
Run (из `web/`): `npm run dev` → открыть указанный URL; проверить: `/` (форма входа) → ввести имя+код → Лобби (Аня/Боря/Вера) → «Начать» → Стол с рукой из 5 карт, коном (8♠, J♠), соперниками и панелью действий. Остановить сервер (Ctrl+C).

- [ ] **Step 10: Commit**

```bash
git add web/src
git commit -m "feat(web): screens + routing (Join → Lobby → Table) on mock store

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Итог итерации

После Task 7: `npm run dev` поднимает три экрана на фикстуре без Go-сервера; шов `Transport` готов принять WebSocket-адаптер Спеца 2 без правок `ui/`/`store/`. Дальнейшие итерации: реальный `ws.ts`, поток `Event` → анимации раздачи/боя, реальный `LegalActions` для подсветки, лобби с реальной готовностью, экран голосования (R-8.6).
