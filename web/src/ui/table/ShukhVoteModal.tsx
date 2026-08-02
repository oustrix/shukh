import { useEffect, useState } from 'react'
import { isLegal } from '../../contract/types'
import type { Action, VoteView } from '../../contract/types'
import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface ShukhVoteModalProps {
  vote: VoteView
  deadline: number | null
  legal: Action[]
  nameOf: (seat: number) => string
  onVote: (v: 'forShukh' | 'againstShukh') => void
  // Обрыв связи: голос заблокирован, как и прочие действия (§8). Кнопки остаются на
  // месте (гасить их подписью «ждём остальных» значило бы соврать — мы ждём не их).
  disabled?: boolean
}

// Разбор R-8.6. Открывается по view.vote (W3-6), поэтому переподключившийся сразу видит
// идущее голосование. Показываем ФАКТ голоса, но не содержание — бюллетень тайный (§8.4).
// Исход (voteResolved) сюда не приходит: сервер обнуляет view.vote тем же апдейтом, что несёт
// это событие, — к моменту исхода модалка уже размонтирована. Исход — отдельный VoteOutcomeBanner.
export function ShukhVoteModal({ vote, deadline, legal, nameOf, onVote, disabled }: ShukhVoteModalProps) {
  const [left, setLeft] = useState(() => remaining(deadline))
  useEffect(() => {
    if (deadline === null) return
    setLeft(remaining(deadline))
    const id = setInterval(() => setLeft(remaining(deadline)), 1000)
    return () => clearInterval(id)
  }, [deadline])

  const forShukh: Action = { type: 'vote', vote: 'forShukh' }
  const against: Action = { type: 'vote', vote: 'againstShukh' }
  const canVote = isLegal(legal, forShukh) || isLegal(legal, against)

  return (
    <div
      className={styles.modalBackdrop}
      role="dialog"
      aria-modal="true"
      aria-label="Голосование по ШУХу"
      data-testid="shukh-vote"
    >
      <div className={styles.modal}>
        <h3>
          ШУХ на «{nameOf(vote.target)}» (Ш-{vote.code})
        </h3>
        <p>Предъявил: {nameOf(vote.claimant)}</p>
        <ul className={styles.voteList}>
          {vote.voted.map((seat) => (
            <li key={seat} data-testid={`voted-${seat}`}>
              {nameOf(seat)}: голос отдан
            </li>
          ))}
        </ul>
        {canVote ? (
          <div className={styles.voteButtons}>
            <Button disabled={disabled} onClick={() => onVote('forShukh')}>
              За ШУХ
            </Button>
            <Button disabled={disabled} onClick={() => onVote('againstShukh')}>
              Против ШУХа
            </Button>
          </div>
        ) : (
          <p className={styles.voteTallying}>
            {left === null ? 'Голосование…' : left > 0 ? `Ждём остальных: ${left} с` : 'Подводим итог…'}
          </p>
        )}
      </div>
    </div>
  )
}

function remaining(deadline: number | null): number | null {
  if (deadline === null) return null
  return Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
}
