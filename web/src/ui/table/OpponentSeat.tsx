import type { OpponentView } from '../../contract/types'
import { ShukhZone } from './ShukhZone'
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
      <ShukhZone count={opponent.shukhPending} label={`ШУХ-зона ${name}: ${opponent.shukhPending}`} />
    </div>
  )
}
