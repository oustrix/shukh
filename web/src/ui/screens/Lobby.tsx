import { useNavigate, useParams } from 'react-router-dom'
import { useGameStore, selectSeats } from '../../store/game'
import { tablePath } from '../../routes'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Lobby() {
  const { code } = useParams()
  const seats = useGameStore(selectSeats)
  const navigate = useNavigate()
  return (
    <div className={styles.centered}>
      <h2>Комната {code}</h2>
      <ul data-testid="players" className={styles.players}>
        {seats.map((s) => (
          <li key={s.seat}>
            {s.name} {s.ready ? '✓' : '…'}
          </li>
        ))}
      </ul>
      <Button onClick={() => navigate(tablePath(code ?? ''))}>Начать</Button>
    </div>
  )
}
