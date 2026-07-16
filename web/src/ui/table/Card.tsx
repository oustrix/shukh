import { motion } from 'motion/react'
import { cardKey, type Card as CardT } from '../../contract/types'
import { cx } from '../kit/cx'
import { rankLabel, isRedSuit, cardLabel } from './cardText'
import styles from './Card.module.css'

interface CardProps {
  card?: CardT
  faceDown?: boolean
  selected?: boolean
  dimmed?: boolean
  onClick?: () => void
}

export function Card({ card, faceDown, selected, dimmed, onClick }: CardProps) {
  const hidden = faceDown || !card
  const red = card ? isRedSuit(card.suit) : false
  const interactive = Boolean(onClick)
  const cls = cx(styles.card, interactive && styles.clickable, dimmed && styles.dimmed)
  return (
    <motion.svg
      layout
      layoutId={card && !hidden ? cardKey(card) : undefined}
      initial={{ opacity: 0, scale: 0.85 }}
      animate={{ opacity: 1, scale: 1, y: selected ? -12 : 0 }}
      exit={{ opacity: 0, scale: 0.85 }}
      transition={{ type: 'spring', stiffness: 500, damping: 40 }}
      viewBox="0 0 60 84"
      className={cls}
      role={interactive ? 'button' : 'img'}
      aria-label={card && !hidden ? cardLabel(card) : 'закрытая карта'}
      aria-disabled={(interactive && dimmed) || undefined}
      tabIndex={interactive ? 0 : undefined}
      onClick={onClick}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onClick!()
              }
            }
          : undefined
      }
      data-testid={hidden ? 'card-back' : 'card-face'}
    >
      <rect x="1" y="1" width="58" height="82" rx="6" className={styles.frame} />
      {hidden ? (
        <rect x="6" y="6" width="48" height="72" rx="4" className={styles.back} />
      ) : (
        <g className={red ? styles.red : styles.black}>
          <text x="7" y="18" fontSize="14" fontWeight="700">
            {rankLabel(card!.rank)}
          </text>
          <text x="30" y="54" fontSize="30" textAnchor="middle">
            {card!.suit}
          </text>
        </g>
      )}
    </motion.svg>
  )
}
