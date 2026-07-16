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
  render(<Hand cards={hand} selectedKey={null} playableKeys={new Set()} onSelect={() => {}} />)
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
