import type { LobbyCommand } from '../contract/transport'
import type { Action } from '../contract/types'
import { selectConn } from '../store/game'
import { useGame } from '../store/GameProvider'
import { useNotify } from './kit/Notice'

// Единственная точка отправки для всех экранов комнаты.
//
// В reconnecting отправлять нельзя (§8): за время обрыва позиция ушла вперёд, и
// отложенная доставка запрещена (W3-5). Транспорт и сам молча отбросил бы действие —
// именно молча, что неотличимо от проглоченного клика; здесь оно отбрасывается ВИДИМО.
//
// Почему общий хук, а не проверка внутри экрана: раньше эта защита жила локально в
// столе, и лобби её не унаследовало — хост жал «Начать» в обрыве и не получал ничего.
// Механизм один на всех, поэтому и живёт он в одном месте.
export function useDispatch(): {
  online: boolean
  send: (action: Action) => boolean
  command: (cmd: LobbyCommand) => boolean
} {
  const conn = useGame(selectConn)
  const play = useGame((s) => s.play)
  const command = useGame((s) => s.command)
  const notify = useNotify()
  const online = conn === 'open'

  // Возвращаемое значение говорит вызывающему, дошло ли отправленное: подтверждение
  // хода не имеет права сбрасывать выбор карты, если ход никуда не уехал.
  function guard<T>(fire: (payload: T) => void): (payload: T) => boolean {
    return (payload) => {
      if (!online) {
        notify('Связь потеряна — действие не отправлено. Повторите после переподключения')
        return false
      }
      fire(payload)
      return true
    }
  }

  return { online, send: guard(play), command: guard(command) }
}
