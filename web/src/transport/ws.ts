import type { Action } from '../contract/types'
import type { ConnState, ConnStatus, ProtocolError, Transport, TransportHandlers } from '../contract/transport'
import { decodeServerMsg, encodeAction, WireError } from '../contract/wire'
import { apiOrigin, me, type MeResult } from '../net/rooms'

// Минимальная часть WebSocket, которой пользуется транспорт: тесты подставляют двойник.
export interface WsLike {
  send(data: string): void
  close(): void
  onopen: ((e?: unknown) => void) | null
  onmessage: ((e: { data: string }) => void) | null
  onclose: ((e?: unknown) => void) | null
  onerror: ((e?: unknown) => void) | null
}

export interface WsDeps {
  socketFactory?: (url: string) => WsLike
  schedule?: (fn: () => void, ms: number) => () => void
  probe?: (code: string) => Promise<MeResult>
}

const BACKOFF_BASE_MS = 500
const BACKOFF_CAP_MS = 8000

// Терминальные причины: место или комната потеряны — повторять бессмысленно (§8).
const TERMINAL = new Set(['seatNotFound', 'roomNotFound'])

function wsURL(code: string): string {
  return `${apiOrigin().replace(/^http/, 'ws')}/ws/${encodeURIComponent(code)}`
}

export function createWsTransport(code: string, deps: WsDeps = {}): Transport {
  const makeSocket = deps.socketFactory ?? ((url: string) => new WebSocket(url) as unknown as WsLike)
  const schedule =
    deps.schedule ??
    ((fn: () => void, ms: number) => {
      const id = setTimeout(fn, ms)
      return () => clearTimeout(id)
    })
  const probe = deps.probe ?? me

  let handlers: TransportHandlers | null = null
  let socket: WsLike | null = null
  let status: ConnStatus = { state: 'connecting' }
  let attempt = 0
  let cancelRetry: (() => void) | null = null
  let stopped = false
  let seq = 0

  function setStatus(state: ConnState, error?: ProtocolError) {
    status = error ? { state, error } : { state }
    handlers?.onStatus(status)
  }

  // Единая точка полного глушения: терминальная ошибка, ручной close() и отписка
  // (см. subscribe ниже) идут через неё, чтобы ни одна из них не забыла погасить
  // таймер реконнекта или закрыть сокет — именно рассинхрон этих трёх путей и был
  // источником обеих находок ревью (сообщение после lost, реконнект после unsubscribe).
  function teardown() {
    stopped = true
    cancelRetry?.()
    cancelRetry = null
    socket?.close()
    socket = null
    handlers = null
  }

  function retryLater() {
    if (stopped) return
    const delay = Math.min(BACKOFF_BASE_MS * 2 ** attempt, BACKOFF_CAP_MS)
    attempt += 1
    cancelRetry = schedule(() => {
      cancelRetry = null
      connect()
    }, delay)
  }

  // Сокет не открылся: браузер не отдаёт статус рукопожатия, поэтому причину узнаём
  // пробой — иначе бесконечный бэкофф в стену на потерянном месте (§7.7).
  function diagnoseFailure() {
    void probe(code).then((res) => {
      if (stopped) return
      if (res.kind === 'seatNotFound' || res.kind === 'roomNotFound') {
        setStatus('lost', { code: res.kind, message: 'место или комната недоступны' })
        return
      }
      retryLater()
    })
  }

  function connect() {
    if (stopped) return
    const s = makeSocket(wsURL(code))
    socket = s
    let opened = false
    s.onopen = () => {
      opened = true
      attempt = 0
      setStatus('open')
    }
    s.onmessage = (e) => {
      // Терминальное состояние обязано быть терминальным: без этой проверки кадр,
      // уже стоявший в очереди микрозадач до close()/lost, всё равно дошёл бы до
      // onSnapshot на мёртвом, но ещё не отписанном от событий сокете.
      if (stopped) return
      let decoded
      try {
        decoded = decodeServerMsg(JSON.parse(e.data))
      } catch (err) {
        // Расхождение зеркал (W-3) — не глотаем: пусть будет видно в консоли и в статусе.
        const message = err instanceof WireError ? err.message : String(err)
        setStatus(status.state, { code: 'wire', message })
        return
      }
      if (decoded.kind === 'update') {
        handlers?.onSnapshot(decoded.snapshot)
        decoded.events.forEach((ev) => handlers?.onEvent(ev))
        return
      }
      if (decoded.kind === 'error') {
        if (TERMINAL.has(decoded.error.code)) {
          // Статус — до teardown(): setStatus читает ещё живой handlers, а сама
          // teardown() следом обнуляет его и глушит сокет/таймер.
          setStatus('lost', decoded.error)
          teardown()
          return
        }
        setStatus(status.state, decoded.error)
      }
      // ack ничего не меняет: рендер идёт только из update (L2-4).
    }
    s.onclose = () => {
      if (stopped) return
      socket = null
      setStatus('reconnecting')
      if (opened) retryLater()
      else diagnoseFailure()
    }
    s.onerror = () => {
      /* onclose придёт следом и разберётся */
    }
  }

  return {
    subscribe(h) {
      handlers = h
      h.onStatus(status)
      connect()
      // subscribe() — не список подписчиков, а единственный канал: отписка означает,
      // что слушателя больше нет вовсе, поэтому она равносильна close(), а не просто
      // снятию колбэков. Следующая задача (room-стор) вызовет её из cleanup React-эффекта
      // при уходе с экрана комнаты — там ожидание ровно такое: уход = полная остановка
      // сокета и отменённый реконнект, а не тихо живущее в фоне соединение.
      return () => teardown()
    },
    send(action: Action) {
      // Отложенная доставка запрещена (W3-5): за время обрыва позиция ушла вперёд.
      if (stopped || status.state !== 'open' || !socket) return
      seq += 1
      socket.send(JSON.stringify({ type: 'action', action: encodeAction(action), reqId: `a${seq}` }))
    },
    close() {
      teardown()
    },
  }
}
