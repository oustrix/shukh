import { create } from 'zustand'
import type { ConnState, LobbyCommand, ProtocolError, Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  conn: ConnState
  lastError: ProtocolError | null
  play: (action: Action) => void
  command: (cmd: LobbyCommand) => void
}

// Предел лога событий — событий за партию много; держим только последние.
export const EVENTS_CAP = 100

// Общие селекторы — чтобы компоненты не дублировали разбор snapshot.
export const selectSnapshot = (s: GameState) => s.snapshot
export const selectSeats = (s: GameState) => s.snapshot?.seats ?? []
export const selectView = (s: GameState) => s.snapshot?.view ?? null
export const selectLegal = (s: GameState) => s.snapshot?.legal ?? []
export const selectStage = (s: GameState) => s.snapshot?.stage ?? null
export const selectYou = (s: GameState) => s.snapshot?.you ?? null
export const selectHost = (s: GameState) => s.snapshot?.host ?? null
export const selectVote = (s: GameState) => s.snapshot?.view?.vote ?? null
export const selectVoteDeadline = (s: GameState) => s.snapshot?.voteDeadline ?? null
export const selectConn = (s: GameState) => s.conn
export const selectLastError = (s: GameState) => s.lastError
export const selectEvents = (s: GameState) => s.events

// Создаёт изолированный стор поверх переданного транспорта. Подписка — ПОСЛЕ создания
// стора: транспорт пушит в уже готовый setState.
export function createGameStore(transport: Transport) {
  const store = create<GameState>(() => ({
    snapshot: null,
    events: [],
    conn: 'connecting',
    lastError: null,
    play: (action) => transport.send(action),
    command: (cmd) => transport.command(cmd),
  }))
  transport.subscribe({
    onSnapshot: (snapshot) => store.setState({ snapshot }),
    onEvent: (event) =>
      store.setState((s) => ({ events: [...s.events, event].slice(-EVENTS_CAP) })),
    onStatus: (status) => store.setState({ conn: status.state, lastError: status.error ?? null }),
  })
  return store
}

export type GameStore = ReturnType<typeof createGameStore>
