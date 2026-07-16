import type { ShukhVote } from '../../contract/types'
import styles from './Table.module.css'

interface ShukhVoteModalProps {
  vote: ShukhVote
  nameOf: (seat: number) => string
}

// Голосование/оспаривание ШУХа (R-8.6). Данные (голоса/исход) — из снапшота (W2-7,
// кворум не считаем); модалка чисто презентационна и управляется таймлайном сценария.
export function ShukhVoteModal({ vote, nameOf }: ShukhVoteModalProps) {
  const ups = vote.votes.filter((v) => v.up).length
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
          {vote.votes.map((v) => (
            <li key={v.seat} data-testid={`vote-${v.seat}`}>
              {nameOf(v.seat)}: {v.up ? '✅ за' : '❌ против'}
            </li>
          ))}
        </ul>
        {vote.resolved ? (
          <p className={styles.voteOutcome} data-testid="vote-outcome">
            {vote.outcome === 'upheld'
              ? `ШУХ подтверждён (${ups} за)`
              : 'ШУХ отклонён — Ш-8 предъявившему'}
          </p>
        ) : (
          <p className={styles.voteTallying}>Голосование…</p>
        )}
      </div>
    </div>
  )
}
