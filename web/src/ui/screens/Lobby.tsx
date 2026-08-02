import { useParams } from 'react-router-dom'
import { useGame } from '../../store/GameProvider'
import { selectSeats } from '../../store/game'
import styles from './Screens.module.css'

// «Начать» (только у хоста, отправка действия старта) — Task 13: здесь пока только
// состав комнаты, стадию переключает сервер (W3-1).
export function Lobby() {
  const { code } = useParams()
  const seats = useGame(selectSeats)
  return (
    <div className={styles.centered}>
      <h2>Комната {code}</h2>
      <ul data-testid="players" className={styles.players}>
        {seats.map((s) => (
          <li key={s.seat}>{s.name}</li>
        ))}
      </ul>
    </div>
  )
}
