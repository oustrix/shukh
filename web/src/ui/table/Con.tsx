import { AnimatePresence } from 'motion/react'
import { cardKey, type TableCard } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface ConProps {
  table: TableCard[]
}

export function Con({ table }: ConProps) {
  return (
    <div className={styles.con} data-testid="con">
      <AnimatePresence mode="popLayout">
        {table.length === 0 ? (
          <span key="empty" className={styles.empty}>
            кон пуст
          </span>
        ) : (
          table.map((tc) => <Card key={cardKey(tc.card)} card={tc.card} />)
        )}
      </AnimatePresence>
    </div>
  )
}
