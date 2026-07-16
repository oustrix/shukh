import { create } from 'zustand'
import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { createScriptedTransport } from '../transport/scripted'
import { demoScenario } from '../fixtures/scenario'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  play: (action: Action) => void
}

// Предел лога событий — событий за партию много; держим только последние.
export const EVENTS_CAP = 100

// Общие селекторы — чтобы компоненты не дублировали разбор snapshot.
export const selectSeats = (s: GameState) => s.snapshot?.seats ?? []
export const selectView = (s: GameState) => s.snapshot?.view ?? null
export const selectLegal = (s: GameState) => s.snapshot?.legal ?? []

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
    (event) => store.setState((s) => ({ events: [...s.events, event].slice(-EVENTS_CAP) })),
  )
  return store
}

// Singleton приложения: скриптованный сценарий (двойник ws.ts Спеца 2). Замена на
// transport/ws.ts не затрагивает компоненты — они читают только этот хук.
export const useGameStore = createGameStore(createScriptedTransport(demoScenario))
