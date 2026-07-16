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
