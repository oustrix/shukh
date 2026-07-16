import { Button } from '../kit/Button'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

interface ActionBarProps {
  canConfirm: boolean
  onConfirm: () => void
  canTakeBottom: boolean
  onTakeBottom: () => void
  canShukh: boolean
  onShukh: () => void
  owesOneCard: boolean
  onOneCard: () => void
}

export function ActionBar({
  canConfirm,
  onConfirm,
  canTakeBottom,
  onTakeBottom,
  canShukh,
  onShukh,
  owesOneCard,
  onOneCard,
}: ActionBarProps) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      <Button onClick={onConfirm} disabled={!canConfirm}>
        Сходить
      </Button>
      <Button onClick={onTakeBottom} disabled={!canTakeBottom}>
        Взять низ
      </Button>
      <Button onClick={onShukh} disabled={!canShukh}>
        ШУХ!
      </Button>
      <Button
        onClick={onOneCard}
        disabled={!owesOneCard}
        className={cx(owesOneCard && styles.pulse)}
      >
        Одна карта!
      </Button>
    </div>
  )
}
