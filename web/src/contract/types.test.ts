import {
  isYourTurn,
  cardKey,
  actionsEqual,
  isLegal,
  isCardPlayable,
  isShukhTakeable,
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
    actionsEqual(
      { type: 'playCard', card: { suit: '♦', rank: 9 } },
      { type: 'playCard', card: { suit: '♦', rank: 9 } },
    ),
  ).toBe(true)
  expect(
    actionsEqual(
      { type: 'playCard', card: { suit: '♦', rank: 9 } },
      { type: 'playCard', card: { suit: '♠', rank: 9 } },
    ),
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
