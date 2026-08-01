import type { Action, GameEvent, GameSnapshot } from './types'

// Состояние соединения (§8 спека). lost — терминальное: место или комната потеряны,
// повторять подключение бессмысленно.
export type ConnState = 'connecting' | 'open' | 'reconnecting' | 'lost'

// Ошибка протокола: code — стабильный код Слоя 2 (§10 дизайна Слоя 2), message — текст.
export interface ProtocolError {
  code: string
  message: string
}

export interface ConnStatus {
  state: ConnState
  error?: ProtocolError
}

export interface TransportHandlers {
  onSnapshot: (s: GameSnapshot) => void
  onEvent: (e: GameEvent) => void
  onStatus: (s: ConnStatus) => void
}

// Шов между UI и сетью (W-2). Единственная реализация — transport/ws.ts; тесты
// подставляют инлайновый двойник.
export interface Transport {
  subscribe(h: TransportHandlers): () => void
  send(action: Action): void
  close(): void
}
