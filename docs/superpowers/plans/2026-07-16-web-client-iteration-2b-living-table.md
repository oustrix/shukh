# Веб-итерация 2b — Living table (анимации на motion) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Оживить стол: карты плавно едут между зонами (раздача → рука, рука → кон, смётание кона), используя `motion` (framer-motion) с layout-анимациями; уважать `prefers-reduced-motion`.

**Architecture:** Поверх 2a (скриптованный транспорт уже гоняет поток снапшотов/событий). Анимации — чисто presentational: карта несёт стабильный `layoutId = cardKey(card)`, и когда снапшот перемещает её из массива `hand` в `table`, `motion` анимирует перелёт (FLIP). Правил/данных не трогаем. Спека: `docs/superpowers/specs/2026-07-16-web-client-iteration-2-design.md` (§4, W2-4/W2-5/W2-8).

**Tech Stack:** существующий стек 2a + **`motion`** (одна новая зависимость; import из `motion/react`). Если пакет `motion` не встанет — эквивалентный `framer-motion` с тем же API (`import { motion, AnimatePresence, MotionConfig } from 'framer-motion'`).

## Global Constraints

- Все команды `npm` — из `web/`. Гейт: `npm run typecheck && npm run lint && npm test`.
- Анимации **presentational**: клиент не считает правил, данные берёт из стора (2a). Стабильный ключ карты — `cardKey(card)` из `src/contract/types.ts`.
- Шов `Transport` и стор не трогаем; меняем только `ui/` + `main.tsx` + тест-инфру.
- `prefers-reduced-motion` уважаем (W2-8): в этом режиме переходы мгновенные. В тестах включаем reduced-motion через полифилл `matchMedia`, чтобы вывод был чистый и детерминированный.
- Существующие тесты (`Card`, `Hand`, `OpponentSeat`, `ActionBar`, `App`) должны продолжать проходить: motion-обёртки прокидывают `data-testid`/`role`/`aria-label`, DOM не меняется по смыслу. Ассерты **не ослаблять**.
- Анимации визуально не юнит-тестим (jsdom не считает layout); тесты — регрессия рендера. Финальная проверка — `npm run dev`.
- TDD там, где есть тестируемое поведение; для чисто визуальных шагов — регрессионный смоук + dev-проверка. Частые коммиты.
- Трейлер каждого коммита: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

```
web/
  package.json            # MODIFY: +motion (через npm install)
  src/
    main.tsx              # MODIFY: обернуть <App/> в <MotionConfig reducedMotion="user">
    setupTests.ts         # MODIFY: полифилл window.matchMedia (reduced-motion ON в тестах)
    ui/
      table/
        Card.tsx          # MODIFY: <svg> → <motion.svg> + layoutId + enter/exit + selected через animate
        Card.module.css   # MODIFY: убрать transform у .selected (теперь через motion)
        Hand.tsx          # MODIFY: key={cardKey}, обернуть в <AnimatePresence>
        Con.tsx           # MODIFY: key={cardKey}, обернуть в <AnimatePresence>
```

`OpponentSeat`/`ActionBar`/`Table`/store/contract — не меняются.

---

### Task 1: motion + стабильные ключи + MotionConfig + matchMedia-полифилл

Ставит зависимость и инфраструктуру; визуального поведения ещё не добавляет (карты пока не motion), поэтому все существующие тесты остаются зелёными.

**Files:**
- Modify: `web/package.json` (через `npm install`), `web/src/main.tsx`, `web/src/setupTests.ts`, `web/src/ui/table/Hand.tsx`, `web/src/ui/table/Con.tsx`
- Test: существующие (регрессия) + проверка, что `matchMedia` доступен

**Interfaces:**
- Consumes: `cardKey` из `src/contract/types.ts` (2a).
- Produces: `motion` доступен; `App` в дереве обёрнут `MotionConfig`; `cardKey` — React-ключ в `Hand`/`Con`.

- [ ] **Step 1: Установить `motion`**

Run (из `web/`): `npm install motion`
Expected: пакет добавлен в `dependencies`, `npm ls motion` показывает версию. (Если `motion` не резолвится — `npm install framer-motion` и далее импортировать из `framer-motion` вместо `motion/react`; зафиксировать выбор в отчёте.)

- [ ] **Step 2: Полифилл `matchMedia` в `web/src/setupTests.ts`**

Дописать в конец файла:
```ts
// jsdom не реализует matchMedia; motion его использует для prefers-reduced-motion.
// В тестах включаем reduced-motion → анимации мгновенные, вывод чистый.
if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}
```

- [ ] **Step 3: Обернуть приложение в `MotionConfig` — `web/src/main.tsx`**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { MotionConfig } from 'motion/react'
import { App } from './App'
import './ui/kit/theme.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MotionConfig reducedMotion="user">
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </MotionConfig>
  </StrictMode>,
)
```

- [ ] **Step 4: Стабильные ключи в `Hand.tsx` и `Con.tsx`**

`web/src/ui/table/Hand.tsx` — импорт `cardKey` и ключ по нему (индекс всё ещё нужен для `selected`/`onSelect`):
```tsx
import type { Card as CardT } from '../../contract/types'
import { cardKey } from '../../contract/types'
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
        <Card key={cardKey(c)} card={c} selected={i === selectedIndex} onClick={() => onSelect(i)} />
      ))}
    </div>
  )
}
```

`web/src/ui/table/Con.tsx`:
```tsx
import type { TableCard } from '../../contract/types'
import { cardKey } from '../../contract/types'
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
        table.map((tc) => <Card key={cardKey(tc.card)} card={tc.card} />)
      )}
    </div>
  )
}
```

- [ ] **Step 5: Полный гейт**

Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное; существующие тесты не тронуты (motion пока не в Card, поведение не изменилось).

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/src/main.tsx web/src/setupTests.ts web/src/ui/table/Hand.tsx web/src/ui/table/Con.tsx
git commit -m "feat(web): add motion + stable card keys + reduced-motion infra

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Карта — motion-компонент (layoutId + появление/уход + выбор через animate)

**Files:**
- Modify: `web/src/ui/table/Card.tsx`, `web/src/ui/table/Card.module.css`
- Test: `web/src/ui/table/Card.test.tsx` (существующий — должен проходить без изменений; при необходимости адаптировать под motion, НЕ ослабляя проверок текста/testid)

**Interfaces:**
- Consumes: `cardKey` (2a), `motion` (Task 1).
- Produces: `Card` — `motion.svg` с `layoutId={cardKey(card)}` для открытых карт; появление (`initial/animate`), уход (`exit`), подъём выбранной через `animate.y`. Публичные атрибуты (`data-testid` `card-face`/`card-back`, `role`, `aria-label`, `onClick`, `tabIndex`) сохранены.

- [ ] **Step 1: Проверить существующий тест как якорь**

Run (из `web/`): `npx vitest run src/ui/table/Card.test.tsx`
Expected: PASS (2 теста: открытая карта показывает `Q`/`♥`; закрытая — рубашку). Это регрессионный якорь для рефактора в motion.

- [ ] **Step 2: Переписать `web/src/ui/table/Card.tsx` на `motion.svg`**

```tsx
import { motion } from 'motion/react'
import type { Card as CardT } from '../../contract/types'
import { cardKey } from '../../contract/types'
import { cx } from '../kit/cx'
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
  const cls = cx(styles.card, onClick && styles.clickable)
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
    </motion.svg>
  )
}
```

- [ ] **Step 3: Убрать transform у `.selected` в `web/src/ui/table/Card.module.css`**

Подъём выбранной карты теперь делает motion (`animate.y`), а не CSS-transform (иначе конфликт с motion-трансформами). Удалить блок:
```css
.selected {
  transform: translateY(-10px);
  transition: transform 0.12s ease;
}
```
(Если на `.selected` больше ничего не завязано — удалить правило целиком. Класс `styles.selected` в `Card.tsx` больше не используется — убран из `cx(...)` в Step 2.)

- [ ] **Step 4: Прогнать якорный тест + гейт**

Run (из `web/`): `npx vitest run src/ui/table/Card.test.tsx`
Expected: PASS без изменений (motion.svg прокидывает `data-testid`/`role`/`aria-label`/текст). Если jsdom-рендер motion даёт act()-варнинги — убедиться, что `setupTests` полифилл `matchMedia` включает reduced-motion (Task 1 Step 2), тогда анимации мгновенные и варнингов нет.
Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное.

- [ ] **Step 5: Commit**

```bash
git add web/src/ui/table/Card.tsx web/src/ui/table/Card.module.css
git commit -m "feat(web): Card as motion component — layoutId + enter/exit + lift on select

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Hand + Con в AnimatePresence — раздача, перелёт руки↔кон, смётание

**Files:**
- Modify: `web/src/ui/table/Hand.tsx`, `web/src/ui/table/Con.tsx`
- Test: `web/src/ui/table/Hand.test.tsx` (существующий — должен проходить), `web/src/ui/table/animation.test.tsx` (новый смоук)

**Interfaces:**
- Consumes: `Card` (motion, Task 2), `AnimatePresence` (motion), `cardKey`.
- Produces: `Hand`/`Con` оборачивают карты в `<AnimatePresence>` — карта, ушедшая из массива, проигрывает `exit`; появившаяся — `initial→animate`; общий `layoutId` даёт перелёт руки→кон.

- [ ] **Step 1: Написать смоук-тест `web/src/ui/table/animation.test.tsx`**

Проверяет, что под motion/AnimatePresence рендер не ломается и данные на месте (регрессия, не кадры анимации):
```tsx
import { render, screen } from '@testing-library/react'
import { Hand } from './Hand'
import { Con } from './Con'
import type { Card, TableCard } from '../../contract/types'

const hand: Card[] = [
  { suit: '♦', rank: 9 },
  { suit: '♥', rank: 12 },
]
const table: TableCard[] = [{ card: { suit: '♠', rank: 8 }, by: 1 }]

test('Hand под AnimatePresence рендерит все карты руки', () => {
  render(<Hand cards={hand} selectedIndex={null} onSelect={() => {}} />)
  expect(screen.getAllByTestId('card-face')).toHaveLength(2)
})

test('Con под AnimatePresence рендерит карты кона', () => {
  render(<Con table={table} />)
  expect(screen.getAllByTestId('card-face')).toHaveLength(1)
})

test('пустой кон показывает заглушку', () => {
  render(<Con table={[]} />)
  expect(screen.getByText('кон пуст')).toBeInTheDocument()
})
```

- [ ] **Step 2: Прогнать — покажет текущее поведение (уже зелёное, т.к. AnimatePresence ещё не добавлен, но карты рендерятся)**

Run (из `web/`): `npx vitest run src/ui/table/animation.test.tsx`
Expected: PASS (карты и так рендерятся). Тест — регрессионный якорь для добавления AnimatePresence.

- [ ] **Step 3: Обернуть `Hand.tsx` в `AnimatePresence`**

```tsx
import { AnimatePresence } from 'motion/react'
import type { Card as CardT } from '../../contract/types'
import { cardKey } from '../../contract/types'
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
      <AnimatePresence>
        {cards.map((c, i) => (
          <Card key={cardKey(c)} card={c} selected={i === selectedIndex} onClick={() => onSelect(i)} />
        ))}
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 4: Обернуть `Con.tsx` в `AnimatePresence`**

```tsx
import { AnimatePresence } from 'motion/react'
import type { TableCard } from '../../contract/types'
import { cardKey } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface ConProps {
  table: TableCard[]
}

export function Con({ table }: ConProps) {
  return (
    <div className={styles.con} data-testid="con">
      <AnimatePresence mode="popLayout">
        {table.length === 0 ? (
          <span key="empty" className={styles.empty}>
            кон пуст
          </span>
        ) : (
          table.map((tc) => <Card key={cardKey(tc.card)} card={tc.card} />)
        )}
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 5: Прогнать смоук + полный гейт**

Run (из `web/`): `npx vitest run src/ui/table/animation.test.tsx src/ui/table/Hand.test.tsx`
Expected: PASS.
Run (из `web/`): `npm run typecheck && npm run lint && npm test`
Expected: всё зелёное (App.test и прочие — без изменений).

- [ ] **Step 6: Ручная проверка анимаций в браузере**

Run (из `web/`): `npm run build` — компилируется.
Run (из `web/`): `npm run dev` → открыть `/room/DEMO/table`. Наблюдать:
- раздача: карты руки появляются (fade+scale);
- ваш заход 9♦ (клик): карта перелетает из руки в кон (общий `layoutId`);
- боты бьют 10♦/J♦: карты появляются в коне; на J♦ кон закрывается по счёту и **сметается** (карты уходят exit-анимацией), затем Вера заходит 7♣;
- ваш бой Дамой♥: перелёт руки→кон, ранний закрыт+смётание.
Проверить `prefers-reduced-motion` (в системе/DevTools): переходы мгновенные, стол функционален.
Остановить сервер.

- [ ] **Step 7: Commit**

```bash
git add web/src/ui/table/Hand.tsx web/src/ui/table/Con.tsx web/src/ui/table/animation.test.tsx
git commit -m "feat(web): AnimatePresence on Hand/Con — deal-in, hand↔con flight, con sweep

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Итог 2b

Стол ожил: карты появляются при раздаче, перелетают рука↔кон по общему `layoutId`, кон сметается exit-анимацией; `prefers-reduced-motion` уважается. Данные/правила/шов не тронуты. Дальше — **2c**: подсветка легальных ходов (из `snapshot.legal`), выбор+подтверждение, a11y-клавиатура, ритуал ШУХа (ШУХ-зоны, кнопки, модалка голосования).

> **Замечание про доводку.** Точный «фил» анимаций (easing, длительности, перелёт в отбой) доводится в `npm run dev` — jsdom не считает layout, поэтому тесты гарантируют лишь отсутствие регрессии рендера, а не кадры. Если общий `layoutId`-перелёт руки→кон в браузере ведёт себя неожиданно (напр. карта не «летит», а появляется), проверить, что и `Hand`, и `Con` находятся в общем дереве под одним `MotionConfig` (они под `Table`), и что `cardKey` совпадает у одной и той же карты в обеих зонах.
