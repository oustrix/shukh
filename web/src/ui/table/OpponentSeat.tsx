import type { ReactNode } from 'react'
import type { OpponentView } from '../../contract/types'
import { ShukhZone } from './ShukhZone'
import styles from './Table.module.css'

interface OpponentSeatProps {
  name: string
  opponent: OpponentView
  menuOpen: boolean
  onToggleMenu: () => void
  children?: ReactNode // меню адресных действий (SeatMenu), когда открыто
}

// Заголовок места — кнопка, открывающая SeatMenu (Задача 15): адресные вопросы к
// соседу (askCount R-6, askAboutWest R-9.4.2) и субъективные ШУХи (§7/§8.6).
export function OpponentSeat({ name, opponent, menuOpen, onToggleMenu, children }: OpponentSeatProps) {
  return (
    <div className={styles.seat} data-testid={`seat-${opponent.seat}`}>
      <button
        type="button"
        className={styles.seatName}
        onClick={onToggleMenu}
        aria-expanded={menuOpen}
      >
        {name}
      </button>
      <div className={styles.seatCount}>🂠 {opponent.handCount}</div>
      <ShukhZone count={opponent.shukhPending} label={`ШУХ-зона ${name}: ${opponent.shukhPending}`} />
      {menuOpen && children}
    </div>
  )
}
