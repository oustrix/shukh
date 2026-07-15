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
