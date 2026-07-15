import { vi } from 'vitest'
import { createGameStore } from './game'
import type { Transport } from '../contract/transport'
import type { GameEvent, GameSnapshot } from '../contract/types'
import { gameSnapshot } from '../fixtures/game'

function fakeTransport() {
  let onSnap: ((s: GameSnapshot) => void) | undefined
  let onEv: ((e: GameEvent) => void) | undefined
  const send = vi.fn()
  const transport: Transport = {
    subscribe(s, e) {
      onSnap = s
      onEv = e
      return () => {}
    },
    send,
  }
  return {
    transport,
    send,
    emitSnapshot: (s: GameSnapshot) => onSnap!(s),
    emitEvent: (e: GameEvent) => onEv!(e),
  }
}

test('стор стартует пустым и принимает снапшот из транспорта', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  expect(store.getState().snapshot).toBeNull()
  f.emitSnapshot(gameSnapshot)
  expect(store.getState().snapshot).toBe(gameSnapshot)
})

test('play пробрасывает действие в transport.send', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  store.getState().play({ type: 'takeBottomAndPass' })
  expect(f.send).toHaveBeenCalledWith({ type: 'takeBottomAndPass' })
})

test('события копятся в events', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: 9 } })
  expect(store.getState().events).toHaveLength(1)
})
