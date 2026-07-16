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
