import { Button } from '../kit/Button'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

export interface BarAction {
  label: string
  enabled: boolean
  onClick: () => void
  pulse?: boolean
}

// Панель ничего не знает о правилах: список действий собирает стол из snapshot.legal (W2-2).
export function ActionBar({ actions }: { actions: BarAction[] }) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      {actions.map((a) => (
        <Button
          key={a.label}
          onClick={a.onClick}
          disabled={!a.enabled}
          className={cx(a.pulse && a.enabled && styles.pulse)}
        >
          {a.label}
        </Button>
      ))}
    </div>
  )
}
