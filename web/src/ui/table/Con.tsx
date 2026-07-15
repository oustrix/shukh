import type { TableCard } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface ConProps {
  table: TableCard[]
}

export function Con({ table }: ConProps) {
  return (
    <div className={styles.con} data-testid="con">
      {table.length === 0 ? (
        <span className={styles.empty}>кон пуст</span>
      ) : (
        table.map((tc, i) => <Card key={i} card={tc.card} />)
      )}
    </div>
  )
}
