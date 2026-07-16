import type { SeatView } from '../contract/types'

// Дефолтная форма SeatView для фикстур и тестов; нужные поля переопределяются
// через `over`. Держит ручной миррор engine/view.go (W-3) в ОДНОМ месте — иначе
// 13-полевой объект дублируется по фикстурам/тестам и молча расходится.
export function buildSeatView(over: Partial<SeatView> = {}): SeatView {
  return {
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
    ...over,
  }
}
