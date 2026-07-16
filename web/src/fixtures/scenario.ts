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
