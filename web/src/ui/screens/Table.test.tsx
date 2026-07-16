import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { create } from 'zustand'
import type { Action, GameSnapshot } from '../../contract/types'

// Изолированный стор-даблинг: подменяем useGameStore локальным zustand-стором.
// Так тестируем Table без реального транспорта.
const sent: Action[] = []
let snapshot: GameSnapshot

vi.mock('../../store/game', async () => {
  const actual = await vi.importActual<typeof import('../../store/game')>('../../store/game')
  const store = create<import('../../store/game').GameState>(() => ({
    snapshot: null,
    events: [],
    play: (a: Action) => sent.push(a),
  }))
  return { ...actual, useGameStore: store }
})

import { useGameStore } from '../../store/game'
import { Table } from './Table'
import { buildSeatView } from '../../fixtures/seatView'

const SEATS = [
  { seat: 0, name: 'Аня', ready: true },
  { seat: 1, name: 'Боря', ready: true },
]

function setSnapshot(over: Partial<GameSnapshot>) {
  snapshot = {
    roomCode: 'DEMO',
    seats: SEATS,
    view: buildSeatView({ opponents: [{ seat: 1, handCount: 3, shukhPending: 0, live: true }] }),
    legal: [],
    shukhVote: null,
    ...over,
  }
  ;(useGameStore as unknown as { setState: (s: Partial<unknown>) => void }).setState({ snapshot })
}

beforeEach(() => {
  sent.length = 0
})

test('«ШУХ!» активна и шлёт конкретный claimShukh из legal', async () => {
  setSnapshot({
    view: buildSeatView({
      hand: [{ suit: '♠', rank: 6 }],
      opponents: [{ seat: 1, handCount: 1, shukhPending: 0, live: true }],
    }),
    legal: [{ type: 'claimShukh', target: 1, code: 11 }],
  })
  render(<Table />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(sent).toContainEqual({ type: 'claimShukh', target: 1, code: 11 })
})

test('«Одна карта!» пульсирует при 1 карте на руке и гасится по клику', async () => {
  setSnapshot({
    view: buildSeatView({ hand: [{ suit: '♠', rank: 6 }], live: { 0: true } }),
    legal: [],
  })
  render(<Table />)
  const btn = screen.getByRole('button', { name: 'Одна карта!' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(btn).toBeDisabled() // announced → пульсация/доступность гаснут
})
