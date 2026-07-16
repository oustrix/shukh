import type { Transport } from '../contract/transport'
import type { Action, GameSnapshot } from '../contract/types'

// Фейковый транспорт на фикстуре. Синхронный пуш снапшота при подписке —
// простая, детерминированная модель для UI и тестов. send() копит действия.
// NB: приложение теперь использует createScriptedTransport (см. store/game.ts);
// этот мок оставлен намеренно как минимальный пример реализации Transport.
export function createMockTransport(snapshot: GameSnapshot): Transport & { sent: Action[] } {
  const sent: Action[] = []
  return {
    sent,
    subscribe(onSnapshot) {
      onSnapshot(snapshot)
      return () => {}
    },
    send(action) {
      sent.push(action)
      console.debug('[mock] send', action)
    },
  }
}
