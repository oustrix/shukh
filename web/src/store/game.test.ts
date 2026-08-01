import { createGameStore, EVENTS_CAP } from './game'
import type { Transport, TransportHandlers } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'

// Инлайновый двойник транспорта: отдельного файла-двойника больше нет (W3-7).
function fakeTransport() {
  let handlers: TransportHandlers | null = null
  const sent: Action[] = []
  const transport: Transport = {
    subscribe(h) {
      handlers = h
      return () => {
        handlers = null
      }
    },
    send: (a) => sent.push(a),
    close: () => {},
  }
  return {
    transport,
    sent,
    push: (s: GameSnapshot) => handlers?.onSnapshot(s),
    event: (e: GameEvent) => handlers?.onEvent(e),
    status: (s: Parameters<TransportHandlers['onStatus']>[0]) => handlers?.onStatus(s),
  }
}

const snap: GameSnapshot = {
  roomCode: 'ABCD',
  you: 1,
  stage: 'lobby',
  host: 0,
  seats: [{ seat: 0, name: 'Вера' }],
  view: null,
  legal: [],
}

describe('store/game', () => {
  it('снапшот из транспорта попадает в стор', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    f.push(snap)
    expect(store.getState().snapshot?.roomCode).toBe('ABCD')
  })

  it('буфер событий хранит ПОСЛЕДНИЕ EVENTS_CAP', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    for (let i = 0; i < EVENTS_CAP + 5; i += 1) f.event({ type: 'turnSkipped', seat: i })
    const events = store.getState().events
    expect(events).toHaveLength(EVENTS_CAP)
    expect(events[events.length - 1]).toEqual({ type: 'turnSkipped', seat: EVENTS_CAP + 4 })
  })

  it('статус соединения и последняя ошибка видны в сторе', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    f.status({ state: 'reconnecting' })
    expect(store.getState().conn).toBe('reconnecting')
    f.status({ state: 'open', error: { code: 'notYours', message: 'не твой ход' } })
    expect(store.getState().conn).toBe('open')
    expect(store.getState().lastError).toEqual({ code: 'notYours', message: 'не твой ход' })
  })

  it('play уходит в транспорт', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    store.getState().play({ type: 'takeBottomAndPass' })
    expect(f.sent).toEqual([{ type: 'takeBottomAndPass' }])
  })
})
