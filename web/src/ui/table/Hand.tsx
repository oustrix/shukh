import { AnimatePresence } from 'motion/react'
import { cardKey, type Card as CardT } from '../../contract/types'
import { Card } from './Card'
import styles from './Table.module.css'

interface HandProps {
  cards: CardT[]
  selectedKey: string | null
  playableKeys: Set<string>
  onSelect: (card: CardT) => void
}

export function Hand({ cards, selectedKey, playableKeys, onSelect }: HandProps) {
  return (
    <div className={styles.hand} data-testid="hand">
      <AnimatePresence>
        {cards.map((c) => {
          const key = cardKey(c)
          const playable = playableKeys.has(key)
          return (
            <Card
              key={key}
              card={c}
              selected={key === selectedKey}
              dimmed={!playable}
              onClick={playable ? () => onSelect(c) : undefined}
            />
          )
        })}
      </AnimatePresence>
    </div>
  )
}
