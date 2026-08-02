import { useEffect } from 'react'
import { isLegal } from '../../contract/types'
import type { Action, SeatID, ShukhCode } from '../../contract/types'
import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface SeatMenuProps {
  seat: SeatID
  name: string
  you: SeatID
  legal: Action[]
  onAction: (a: Action) => void
  onClose: () => void
}

// Субъективные ШУХи (R-8.4/R-8.7/R-8.8) движок в legal НЕ перечисляет — это всегда
// доступная социальная кнопка, законность которой сервер проверяет на сабмите.
const SUBJECTIVE: { code: ShukhCode; label: string }[] = [
  { code: 6, label: 'ШУХ: завис (Ш-6)' },
  { code: 9, label: 'ШУХ: зря крикнул (Ш-9)' },
  { code: 10, label: 'ШУХ: небрежность (Ш-10)' },
]

export function SeatMenu({ seat, name, you, legal, onAction, onClose }: SeatMenuProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const fire = (a: Action) => {
    onAction(a)
    onClose()
  }
  const askCount: Action = { type: 'askCount', target: seat }
  const askWest: Action = { type: 'askAboutWest', target: seat }

  // Без role="menu"/"menuitem": полноценный ARIA-menu виджет требует roving tabindex
  // и навигации стрелками, которых тут нет. Половинчатый паттерн — регресс для
  // скринридеров, поэтому кнопки остаются обычными <button> (нативная фокус-модель).
  return (
    <div className={styles.seatMenu} aria-label={`Действия: ${name}`}>
      {isLegal(legal, askCount) && (
        <Button onClick={() => fire(askCount)}>Сколько карт?</Button>
      )}
      {isLegal(legal, askWest) && <Button onClick={() => fire(askWest)}>Есть Запад?</Button>}
      {SUBJECTIVE.map((s) => (
        <Button
          key={s.code}
          onClick={() => fire({ type: 'claimSubjective', claimant: you, target: seat, code: s.code })}
        >
          {s.label}
        </Button>
      ))}
    </div>
  )
}
