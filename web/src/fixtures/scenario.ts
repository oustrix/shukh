import type { Card, GameSnapshot, SeatView } from '../contract/types'
import type { Scenario } from '../transport/scripted'

const c = (rank: number, suit: Card['suit']): Card => ({ rank, suit })

const SEATS = [
  { seat: 0, name: 'Аня', ready: true },
  { seat: 1, name: 'Боря', ready: true },
  { seat: 2, name: 'Вера', ready: true },
]

const opp = (bori: number, vera: number) => [
  { seat: 1, handCount: bori, shukhPending: 0, live: true },
  { seat: 2, handCount: vera, shukhPending: 0, live: true },
]

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
      opponents: opp(5, 5),
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

// 3 игрока (you=0 Аня, 1 Боря, 2 Вера), Deck36. Демонстрирует оба способа закрытия
// кона (I-4): по счёту (R-5.5: 3 карты = 3 живых игрока) и Дамой♥ рано (R-3.7.1:
// 2 карты < 3 игроков). Закрывший кон открывает следующий (R-5.7).
export const demoScenario: Scenario = [
  // 0. Раздача, ваш заход. Легально всё, кроме Дамы♥ (R-5.2).
  {
    kind: 'auto',
    events: [{ type: 'gameStarted', turn: 0 }],
    snapshot: base(
      { hand: [c(9, '♦'), c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')], turn: 0, opponents: opp(5, 5) },
      [
        { type: 'playCard', card: c(9, '♦') },
        { type: 'playCard', card: c(6, '♠') },
        { type: 'playCard', card: c(14, '♣') },
        { type: 'playCard', card: c(7, '♦') },
      ],
    ),
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
        opponents: opp(5, 5),
      },
      [],
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
        opponents: opp(4, 5),
      },
      [],
    ),
  },
  // 3. Вера бьёт J♦ (auto): 3 карты = 3 игрока → кон закрывается ПО СЧЁТУ (R-5.5),
  //    сметается; Вера (закрывший) открывает следующий (R-5.7).
  {
    kind: 'auto',
    delayMs: 800,
    events: [
      { type: 'cardPlayed', seat: 2, card: c(11, '♦') },
      { type: 'conClosed', by: 2 },
      { type: 'conSwept', cards: [c(9, '♦'), c(10, '♦'), c(11, '♦')] },
    ],
    snapshot: base(
      {
        hand: [c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [],
        discard: 3,
        turn: 2,
        opponents: opp(4, 4),
      },
      [],
    ),
  },
  // 4. Вера заходит 7♣ (auto). Ход к вам: бить или взять низ.
  {
    kind: 'auto',
    delayMs: 800,
    events: [{ type: 'cardPlayed', seat: 2, card: c(7, '♣') }],
    snapshot: base(
      {
        hand: [c(12, '♥'), c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [{ card: c(7, '♣'), by: 2 }],
        discard: 3,
        turn: 0,
        opponents: opp(4, 3),
      },
      // 7♣ бьют: A♣ (старше по масти), 7♦ (козырь), Дама♥ (бьёт что угодно); плюс «взять низ».
      [
        { type: 'playCard', card: c(12, '♥') },
        { type: 'playCard', card: c(14, '♣') },
        { type: 'playCard', card: c(7, '♦') },
        { type: 'takeBottomAndPass' },
      ],
    ),
  },
  // 5. Вы бьёте Дамой♥: 2 карты < 3 игроков → кон закрывается РАНО (R-3.7.1),
  //    сметается; вы (сыгравший Даму♥) открываете следующий (R-5.7).
  {
    kind: 'await',
    expect: { type: 'playCard', card: c(12, '♥') },
    events: [
      { type: 'cardPlayed', seat: 0, card: c(12, '♥') },
      { type: 'conClosed', by: 0 },
      { type: 'conSwept', cards: [c(7, '♣'), c(12, '♥')] },
    ],
    snapshot: base(
      {
        hand: [c(6, '♠'), c(14, '♣'), c(7, '♦')],
        table: [],
        discard: 5,
        turn: 0,
        opponents: opp(4, 3),
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
