import { useEffect } from 'react'
import { isLegal, SUBJECTIVE_CODES } from '../../contract/types'
import type { Action, SeatID } from '../../contract/types'
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
// Сам перечень кодов — из контракта (SUBJECTIVE_CODES), здесь только подписи:
// Record по этому же типу заставит компилятор потребовать подпись на новый код.
const SUBJECTIVE_LABEL: Record<(typeof SUBJECTIVE_CODES)[number], string> = {
  6: 'ШУХ: завис (Ш-6)',
  9: 'ШУХ: зря крикнул (Ш-9)',
  10: 'ШУХ: небрежность (Ш-10)',
}

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
      {SUBJECTIVE_CODES.map((code) => (
        <Button
          key={code}
          onClick={() => fire({ type: 'claimSubjective', claimant: you, target: seat, code })}
        >
          {SUBJECTIVE_LABEL[code]}
        </Button>
      ))}
    </div>
  )
}
