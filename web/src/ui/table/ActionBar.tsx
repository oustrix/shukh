import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface ActionBarProps {
  yourTurn: boolean
  onShukh: () => void
  onOneCard: () => void
  onTakeBottom: () => void
}

export function ActionBar({ yourTurn, onShukh, onOneCard, onTakeBottom }: ActionBarProps) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      <Button onClick={onShukh}>ШУХ!</Button>
      <Button onClick={onOneCard}>Одна карта!</Button>
      <Button onClick={onTakeBottom} disabled={!yourTurn}>
        Взять низ
      </Button>
    </div>
  )
}
