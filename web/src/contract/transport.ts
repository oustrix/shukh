import type { Action, GameEvent, GameSnapshot } from './types'

// Шов между UI и источником данных (W-2). Сейчас реализуется моком на фикстуре;
// позже — WebSocket-адаптером Спеца 2, БЕЗ правок ui/ и store/.
export interface Transport {
  // Подписка на пуш-обновления. Возвращает функцию отписки.
  subscribe(onSnapshot: (s: GameSnapshot) => void, onEvent: (e: GameEvent) => void): () => void
  // Отправка действия игрока (§5 / ШУХи).
  send(action: Action): void
}
