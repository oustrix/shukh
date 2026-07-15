import { create } from 'zustand'
import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { createMockTransport } from '../transport/mock'
import { gameSnapshot } from '../fixtures/game'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  play: (action: Action) => void
}

// Общие селекторы — чтобы компоненты не дублировали разбор snapshot.
export const selectSeats = (s: GameState) => s.snapshot?.seats ?? []
export const selectView = (s: GameState) => s.snapshot?.view ?? null

// Создаёт изолированный стор поверх переданного транспорта. Подписка — ПОСЛЕ
// создания стора: транспорт пушит в уже готовый setState.
export function createGameStore(transport: Transport) {
  const store = create<GameState>(() => ({
    snapshot: null,
    events: [],
    play: (action) => transport.send(action),
  }))
  transport.subscribe(
    (snapshot) => store.setState({ snapshot }),
    (event) => store.setState((s) => ({ events: [...s.events, event] })),
  )
  return store
}

// Singleton приложения: пока на моке с фикстурой. Замена мока на ws.ts (Спец 2)
// не затрагивает компоненты — они читают только этот хук.
export const useGameStore = createGameStore(createMockTransport(gameSnapshot))
