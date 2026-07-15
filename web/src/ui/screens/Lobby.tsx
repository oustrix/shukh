import { useNavigate, useParams } from 'react-router-dom'
import { useGameStore } from '../../store/game'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Lobby() {
  const { code } = useParams()
  const seats = useGameStore((s) => s.snapshot?.seats ?? [])
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
      <Button onClick={() => navigate(`/room/${code}/table`)}>Начать</Button>
    </div>
  )
}
