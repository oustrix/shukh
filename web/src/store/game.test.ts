import { vi } from 'vitest'
import { createGameStore, selectLegal, EVENTS_CAP } from './game'
import { createScriptedTransport } from '../transport/scripted'
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

test('selectLegal возвращает snapshot.legal', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  f.emitSnapshot({ ...gameSnapshot }) // gameSnapshot теперь несёт legal
  expect(selectLegal(store.getState())).toEqual(gameSnapshot.legal)
})

test('events ограничены EVENTS_CAP (кольцевой буфер)', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  for (let i = 0; i < EVENTS_CAP + 25; i++) {
    f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: 9 } })
  }
  expect(store.getState().events).toHaveLength(EVENTS_CAP)
})

test('events keep-last: буфер хранит ПОСЛЕДНИЕ события, не первые', () => {
  const f = fakeTransport()
  const store = createGameStore(f.transport)
  const N = EVENTS_CAP + 5
  for (let i = 0; i < N; i++) {
    f.emitEvent({ type: 'cardPlayed', seat: 0, card: { suit: '♦', rank: i } })
  }
  const evs = store.getState().events as Extract<GameEvent, { type: 'cardPlayed' }>[]
  expect(evs).toHaveLength(EVENTS_CAP)
  expect(evs[EVENTS_CAP - 1].card.rank).toBe(N - 1) // самое свежее — последнее
  expect(evs[0].card.rank).toBe(N - EVENTS_CAP) // первые 5 вытеснены
})

test('стор проходит сценарий: play продвигает снапшот (синхронный планировщик)', () => {
  const store = createGameStore(
    createScriptedTransport(
      [
        { kind: 'auto', events: [], snapshot: { ...gameSnapshot } },
        {
          kind: 'await',
          expect: { type: 'takeBottomAndPass' },
          events: [{ type: 'cardsTaken', seat: 0, cards: [] }],
          snapshot: { ...gameSnapshot, roomCode: 'AFTER' },
        },
      ],
      (fn) => fn(),
    ),
  )
  expect(store.getState().snapshot?.roomCode).toBe('DEMO')
  store.getState().play({ type: 'takeBottomAndPass' })
  expect(store.getState().snapshot?.roomCode).toBe('AFTER')
})
