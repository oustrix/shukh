import { act, render, screen } from '@testing-library/react'
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
  command: () => {},
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

test('«Одна карта!» отключена, пока declareOneCard не пришёл в legal', () => {
  setSnapshot({
    view: buildSeatView({ hand: [{ suit: '♠', rank: 6 }], live: { 0: true } }),
    legal: [],
  })
  renderTable()
  expect(screen.getByRole('button', { name: 'Одна карта!' })).toBeDisabled()
})

test('«Одна карта!» активна при declareOneCard в legal и шлёт именно его (§6, Ш-11)', async () => {
  setSnapshot({
    view: buildSeatView({ hand: [{ suit: '♠', rank: 6 }], live: { 0: true } }),
    legal: [{ type: 'declareOneCard', seat: 0 }],
  })
  renderTable()
  const btn = screen.getByRole('button', { name: 'Одна карта!' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(sent).toContainEqual({ type: 'declareOneCard', seat: 0 })
})

test('«Западло» отключена, пока podkladkaWest не пришёл в legal', () => {
  setSnapshot({ legal: [] })
  renderTable()
  expect(screen.getByRole('button', { name: 'Западло' })).toBeDisabled()
})

test('«Западло» активна при podkladkaWest в legal и шлёт именно его', async () => {
  setSnapshot({ legal: [{ type: 'podkladkaWest' }] })
  renderTable()
  const btn = screen.getByRole('button', { name: 'Западло' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(sent).toContainEqual({ type: 'podkladkaWest' })
})

test('«Сбросить Запад» отключена, пока discardWest не пришёл в legal', () => {
  setSnapshot({ legal: [] })
  renderTable()
  expect(screen.getByRole('button', { name: 'Сбросить Запад' })).toBeDisabled()
})

test('«Сбросить Запад» активна при discardWest в legal и шлёт именно его (R-9.4.2.1)', async () => {
  setSnapshot({ legal: [{ type: 'discardWest' }] })
  renderTable()
  const btn = screen.getByRole('button', { name: 'Сбросить Запад' })
  expect(btn).toBeEnabled()
  await userEvent.click(btn)
  expect(sent).toContainEqual({ type: 'discardWest' })
})

test('исход разбора показывается баннером по событию voteResolved, а не через view.vote', () => {
  // Регрессия бага: сервер обнуляет view.vote тем же апдейтом, что несёт voteResolved,
  // так что модалка к этому моменту уже размонтирована — исход обязан жить по событию.
  setSnapshot({})
  renderTable()
  expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
  act(() => {
    store.setState((s) => ({ events: [...s.events, { type: 'voteResolved', code: 6, overturned: false }] }))
  })
  expect(screen.getByTestId('vote-outcome-banner')).toHaveTextContent(/подтверждён/i)
})
