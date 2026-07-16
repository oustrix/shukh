import { motion, AnimatePresence } from 'motion/react'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

interface ShukhZoneProps {
  count: number
  takeable?: boolean
  onTake?: () => void
  label?: string
}

// Отложенные ШУХ-карты места (I-3 — не входят в руку). Рубашкой вверх, только счётчик.
// Своя зона кликабельна, когда взятие законно (R-8.3, гейтится legal через takeable).
export function ShukhZone({ count, takeable, onTake, label }: ShukhZoneProps) {
  if (count === 0 && !takeable) return null
  const interactive = Boolean(takeable && onTake)
  return (
    <div
      className={cx(styles.shukhZone, interactive && styles.shukhTakeable)}
      data-testid="shukh-zone"
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      aria-label={label ?? `ШУХ-зона: ${count}`}
      title={takeable ? 'Забрать ШУХ-карты' : 'Отложенные ШУХ-карты'}
      onClick={interactive ? onTake : undefined}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onTake?.()
              }
            }
          : undefined
      }
    >
      <div className={styles.shukhStack}>
        <AnimatePresence>
          {Array.from({ length: count }, (_, i) => (
            <motion.span
              key={i}
              className={styles.shukhChip}
              initial={{ opacity: 0, scale: 0.6 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.6 }}
            />
          ))}
        </AnimatePresence>
      </div>
      <span className={styles.shukhCount} data-testid="shukh-count">
        ШУХ {count}
      </span>
    </div>
  )
}
