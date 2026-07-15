import type { Card as CardT } from '../../contract/types'
import { cx } from '../kit/cx'
import { rankLabel, isRedSuit, cardLabel } from './cardText'
import styles from './Card.module.css'

interface CardProps {
  card?: CardT
  faceDown?: boolean
  selected?: boolean
  onClick?: () => void
}

export function Card({ card, faceDown, selected, onClick }: CardProps) {
  const hidden = faceDown || !card
  const red = card ? isRedSuit(card.suit) : false
  const cls = cx(styles.card, selected && styles.selected, onClick && styles.clickable)
  return (
    <svg
      viewBox="0 0 60 84"
      className={cls}
      role={onClick ? 'button' : 'img'}
      aria-label={card && !hidden ? cardLabel(card) : 'закрытая карта'}
      tabIndex={onClick ? 0 : undefined}
      onClick={onClick}
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
    </svg>
  )
}
