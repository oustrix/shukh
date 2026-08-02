import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { create } from 'zustand'
import type { Action, GameSnapshot } from '../../contract/types'
import type { GameState } from '../../store/game'
import { GameContext } from '../../store/GameProvider'
import { Table } from './Table'
import { buildSeatView } from '../../fixtures/seatView'

// Изолированный стор-даблинг: подменяем GameContext локальным zustand-стором.
// Так тестируем Table без реального транспорта.
const sent: Action[] = []
let snapshot: GameSnapshot

const store = create<GameState>(() => ({
  snapshot: null,
  events: [],
  conn: 'open',
  lastError: null,
  play: (a: Action) => sent.push(a),
}))

function renderTable() {
  return render(
    <GameContext.Provider value={store}>
      <Table />
    </GameContext.Provider>,
  )
}

const SEATS = [
  { seat: 0, name: 'Аня' },
  { seat: 1, name: 'Боря' },
]

function setSnapshot(over: Partial<GameSnapshot>) {
  snapshot = {
    roomCode: 'DEMO',
    you: 0,
    stage: 'playing',
    host: 0,
    seats: SEATS,
    view: buildSeatView({ opponents: [{ seat: 1, handCount: 3, shukhPending: 0, live: true }] }),
    legal: [],
    ...over,
  }
  store.setState({ snapshot })
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
  renderTable()
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(sent).toContainEqual({ type: 'claimShukh', target: 1, code: 11 })
})

test('«Одна карта!» пульсирует при 1 карте на руке и гасится по клику', async () => {
  setSnapshot({
    view: buildSeatView({ hand: [{ suit: '♠', rank: 6 }], live: { 0: true } }),
    legal: [],
  })
  renderTable()
  const btn = screen.getByRole('button', { name: 'Одна карта!' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(btn).toBeDisabled() // announced → пульсация/доступность гаснут
})
