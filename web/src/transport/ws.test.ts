import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { createWsTransport } from './ws'
import type { WsLike } from './ws'
import type { ConnStatus, TransportHandlers } from '../contract/transport'
import type { GameSnapshot } from '../contract/types'
import type { MeResult } from '../net/rooms'

// Литерал вынесен в переменную — как в contract/wire.test.ts: инлайновый
// new URL('...', import.meta.url) Vite переписывает статически в asset-URL.
const playingPath = '../../../server/testdata/wire/playing.json'
const playingJSON = readFileSync(fileURLToPath(new URL(playingPath, import.meta.url)), 'utf8')

class FakeSocket implements WsLike {
  static last: FakeSocket | null = null
  static created = 0
  sent: string[] = []
  closed = false
  onopen: ((e?: unknown) => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: ((e?: unknown) => void) | null = null
  onerror: ((e?: unknown) => void) | null = null
  constructor(readonly url: string) {
    FakeSocket.last = this
    FakeSocket.created += 1
  }
  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.closed = true
  }
  open() {
    this.onopen?.()
  }
  message(raw: string) {
    this.onmessage?.({ data: raw })
  }
  drop() {
    this.onclose?.()
  }
}

interface Harness {
  statuses: ConnStatus[]
  snapshots: GameSnapshot[]
  transport: ReturnType<typeof createWsTransport>
  runTimers: () => void
  unsubscribe: () => void
}

function harness(probeResult: 'seat' | 'seatNotFound' | 'roomNotFound' = 'seat'): Harness {
  FakeSocket.last = null
  FakeSocket.created = 0
  const pending: (() => void)[] = []
  const statuses: ConnStatus[] = []
  const snapshots: GameSnapshot[] = []
  const transport = createWsTransport('ABCD', {
    socketFactory: (url) => new FakeSocket(url),
    schedule: (fn) => {
      pending.push(fn)
      return () => {}
    },
    probe: async (): Promise<MeResult> =>
      probeResult === 'seat' ? { kind: 'seat', seat: 1 } : { kind: probeResult },
  })
  const handlers: TransportHandlers = {
    onSnapshot: (s) => snapshots.push(s),
    onEvent: () => {},
    onStatus: (s) => statuses.push(s),
  }
  const unsubscribe = transport.subscribe(handlers)
  return {
    statuses,
    snapshots,
    transport,
    runTimers: () => pending.splice(0).forEach((fn) => fn()),
    unsubscribe,
  }
}

describe('transport/ws', () => {
  it('открытие сокета переводит в open, update отдаёт снапшот', () => {
    const h = harness()
    expect(h.statuses.at(-1)?.state).toBe('connecting')
    FakeSocket.last!.open()
    expect(h.statuses.at(-1)?.state).toBe('open')
    FakeSocket.last!.message(playingJSON)
    expect(h.snapshots).toHaveLength(1)
    expect(h.snapshots[0].stage).toBe('playing')
  })

  it('send уходит на провод с reqId; ack не меняет снапшот', () => {
    const h = harness()
    FakeSocket.last!.open()
    h.transport.send({ type: 'takeBottomAndPass' })
    const sent = JSON.parse(FakeSocket.last!.sent[0])
    expect(sent.type).toBe('action')
    expect(sent.action).toEqual({ type: 'takeBottomAndPass' })
    expect(typeof sent.reqId).toBe('string')
    FakeSocket.last!.message(JSON.stringify({ type: 'ack', reqId: sent.reqId }))
    expect(h.snapshots).toHaveLength(0)
  })

  it('error кладётся в статус, не роняя соединение', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.message(JSON.stringify({ type: 'error', code: 'notYours', message: 'не твой ход' }))
    expect(h.statuses.at(-1)).toEqual({ state: 'open', error: { code: 'notYours', message: 'не твой ход' } })
  })

  it('обрыв ведёт в reconnecting и пересоздаёт сокет по таймеру', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.drop()
    expect(h.statuses.at(-1)?.state).toBe('reconnecting')
    expect(FakeSocket.created).toBe(1)
    h.runTimers()
    expect(FakeSocket.created).toBe(2)
  })

  it('в reconnecting отправка отбрасывается (W3-5)', () => {
    const h = harness()
    FakeSocket.last!.open()
    const socket = FakeSocket.last!
    socket.drop()
    h.transport.send({ type: 'takeBottomAndPass' })
    expect(socket.sent).toHaveLength(0)
  })

  it('error{seatNotFound} по сокету — терминальный lost, реконнекта нет', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.message(JSON.stringify({ type: 'error', code: 'seatNotFound', message: 'gone' }))
    expect(h.statuses.at(-1)?.state).toBe('lost')
    h.runTimers()
    expect(FakeSocket.created).toBe(1)
  })

  it('close() закрывает сокет и прекращает реконнект', () => {
    const h = harness()
    FakeSocket.last!.open()
    h.transport.close()
    expect(FakeSocket.last!.closed).toBe(true)
    h.runTimers()
    expect(FakeSocket.created).toBe(1)
  })

  it('сообщение на том же сокете после terminal lost игнорируется', () => {
    const h = harness()
    FakeSocket.last!.open()
    const socket = FakeSocket.last!
    socket.message(JSON.stringify({ type: 'error', code: 'seatNotFound', message: 'gone' }))
    expect(h.statuses.at(-1)?.state).toBe('lost')
    // Кадр, пришедший на тот же (уже закрытый нами, но не отписанный от событий
    // в фейке) сокет, не должен просочиться в onSnapshot — терминальность обязана
    // быть терминальной.
    socket.message(playingJSON)
    expect(h.snapshots).toHaveLength(0)
  })

  it('unsubscribe из subscribe() гасит запланированный реконнект (эффект-cleanup = close)', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.drop()
    expect(h.statuses.at(-1)?.state).toBe('reconnecting')
    expect(FakeSocket.created).toBe(1)
    h.unsubscribe()
    h.runTimers()
    expect(FakeSocket.created).toBe(1)
  })

  it('проба, резолвящаяся после close(), не создаёт сокет и не меняет статус', async () => {
    FakeSocket.last = null
    FakeSocket.created = 0
    const pending: (() => void)[] = []
    const statuses: ConnStatus[] = []
    let resolveProbe: ((r: MeResult) => void) | null = null
    const transport = createWsTransport('ABCD', {
      socketFactory: (url) => new FakeSocket(url),
      schedule: (fn) => {
        pending.push(fn)
        return () => {}
      },
      probe: () =>
        new Promise<MeResult>((resolve) => {
          resolveProbe = resolve
        }),
    })
    transport.subscribe({ onSnapshot: () => {}, onEvent: () => {}, onStatus: (s) => statuses.push(s) })
    // Обрыв до открытия идёт через пробу, а не сразу в бэкофф (§7.7).
    FakeSocket.last!.drop()
    expect(statuses.at(-1)?.state).toBe('reconnecting')
    transport.close()
    resolveProbe!({ kind: 'seat', seat: 1 })
    await Promise.resolve()
    await Promise.resolve()
    expect(FakeSocket.created).toBe(1)
    expect(statuses.at(-1)?.state).toBe('reconnecting')
    expect(pending).toHaveLength(0)
  })

  it('упавшая проба не вешает реконнект: попытки продолжаются (§8, бэкофф бесконечен)', async () => {
    // me() делает голый fetch — оффлайн/DNS/лежащий сервер РЕДЖЕКТЯТ промис.
    // Без обработки реджекта retryLater() не вызывался бы никогда, и транспорт
    // навсегда застревал в reconnecting, продолжая обещать баннером переподключение.
    FakeSocket.last = null
    FakeSocket.created = 0
    const pending: (() => void)[] = []
    const statuses: ConnStatus[] = []
    const transport = createWsTransport('ABCD', {
      socketFactory: (url) => new FakeSocket(url),
      schedule: (fn) => {
        pending.push(fn)
        return () => {}
      },
      probe: () => Promise.reject(new Error('offline')),
    })
    transport.subscribe({ onSnapshot: () => {}, onEvent: () => {}, onStatus: (s) => statuses.push(s) })
    FakeSocket.last!.drop() // сокет не открылся → диагностика пробой
    await Promise.resolve()
    await Promise.resolve()
    expect(statuses.at(-1)?.state).toBe('reconnecting')
    expect(pending).toHaveLength(1) // попытка запланирована, а не потеряна
    pending.splice(0).forEach((fn) => fn())
    expect(FakeSocket.created).toBe(2)
  })
})
