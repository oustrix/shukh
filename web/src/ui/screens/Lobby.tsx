import { useState } from 'react'
import { useGame } from '../../store/GameProvider'
import { selectHost, selectSeats, selectSnapshot, selectYou } from '../../store/game'
import type { RoomConfig } from '../../net/rooms'
import { useDispatch } from '../useDispatch'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

// Настройки партии живут у хоста (Слой 1 отвергнет их у остальных); роль хоста может
// мигрировать (L2-3), поэтому сравниваем с host из снапшота, а не запоминаем при входе.
//
// Конфиг держим в локальном состоянии: Update Слоя 1 его не несёт, поэтому единственный
// его носитель до старта — выбор самого хоста. Следствие: остальные игроки настроек в
// лобби не видят (зафиксировано как ограничение, не чиним здесь).
export function Lobby() {
  const snapshot = useGame(selectSnapshot)
  const seats = useGame(selectSeats)
  const you = useGame(selectYou)
  const host = useGame(selectHost)
  // Тот же общий шов отправки, что и у стола: в обрыве команда не уезжает молча,
  // а игрок получает уведомление (§8/W3-5).
  const { online, command } = useDispatch()
  const isHost = you !== null && you === host
  const [config, setConfig] = useState<RoomConfig>({ deckSize: 36, mode: 'middle' })

  // Отправляем ВЕСЬ конфиг: SetConfig заменяет его целиком, поэтому частичная посылка
  // молча сбросила бы соседнее поле к дефолту.
  const push = (next: RoomConfig) => {
    setConfig(next)
    command({ type: 'setConfig', config: next })
  }

  return (
    <div className={styles.centered}>
      <h2>
        Комната <span className={styles.code}>{snapshot?.roomCode}</span>
      </h2>
      <p>Позовите друзей — отправьте им адрес этой страницы.</p>
      <ul data-testid="players" className={styles.players}>
        {seats.map((s) => (
          <li key={s.seat}>
            <span>{s.name}</span>
            {s.seat === host ? ' — хост' : ''}
          </li>
        ))}
      </ul>
      {isHost && (
        <>
          <label>
            Колода
            <select
              value={String(config.deckSize)}
              onChange={(e) => push({ ...config, deckSize: Number(e.target.value) as 36 | 52 })}
            >
              <option value="36">36 карт</option>
              <option value="52">52 карты</option>
            </select>
          </label>
          <label>
            Строгость
            <select
              value={config.mode}
              onChange={(e) => push({ ...config, mode: e.target.value as RoomConfig['mode'] })}
            >
              <option value="guard">Максимум защиты</option>
              <option value="middle">Середина</option>
            </select>
          </label>
          <Button onClick={() => command({ type: 'start' })} disabled={!online || seats.length < 2}>
            Начать
          </Button>
        </>
      )}
      {!isHost && <p>Ждём, пока хост начнёт партию…</p>}
    </div>
  )
}
