import { AnimatePresence } from 'motion/react'
import type { Card as CardT } from '../../contract/types'
import { cardKey } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface HandProps {
  cards: CardT[]
  selectedIndex: number | null
  onSelect: (index: number) => void
}

export function Hand({ cards, selectedIndex, onSelect }: HandProps) {
  return (
    <div className={styles.hand} data-testid="hand">
      <AnimatePresence>
        {cards.map((c, i) => (
          <Card key={cardKey(c)} card={c} selected={i === selectedIndex} onClick={() => onSelect(i)} />
        ))}
      </AnimatePresence>
    </div>
  )
}
