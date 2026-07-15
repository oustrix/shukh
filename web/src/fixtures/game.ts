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
