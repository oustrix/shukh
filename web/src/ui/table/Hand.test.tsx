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
