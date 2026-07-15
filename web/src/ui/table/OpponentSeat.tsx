import type { OpponentView } from '../../contract/types'
import styles from './Table.module.css'

interface OpponentSeatProps {
  name: string
  opponent: OpponentView
}

export function OpponentSeat({ name, opponent }: OpponentSeatProps) {
  return (
    <div className={styles.seat} data-testid={`seat-${opponent.seat}`}>
      <div className={styles.seatName}>{name}</div>
      <div className={styles.seatCount}>🂠 {opponent.handCount}</div>
      {opponent.shukhPending > 0 && (
        <div className={styles.shukhBadge} data-testid="shukh-badge" title="ожидает ШУХ-карт">
          ШУХ {opponent.shukhPending}
        </div>
      )}
    </div>
  )
}
