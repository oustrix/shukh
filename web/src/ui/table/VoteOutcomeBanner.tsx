import { useEffect, useRef, useState } from 'react'
import type { GameEvent, ShukhCode } from '../../contract/types'
import styles from './Table.module.css'

// Сколько баннер держится на экране (4-6с — разумный компромисс: успеваешь прочитать,
// но он не мозолит глаза до конца кона).
const VISIBLE_MS = 5000

interface VoteOutcomeBannerProps {
  events: GameEvent[]
}

interface Outcome {
  code: ShukhCode
  overturned: boolean
}

// Буфер событий в сторе (selectEvents, store/game.ts) — НАКОПЛЕННЫЙ лог, а не «что пришло
// только что»: voteResolved, однажды туда попав, остаётся там до вытеснения по EVENTS_CAP.
// Рендерить баннер по «последнему voteResolved в списке» нельзя — он всплывал бы заново
// при каждом ре-рендере Table (любой ход соперника меняет events). Поэтому реагируем не
// на НАЛИЧИЕ события в буфере, а на его ПОЯВЛЕНИЕ: держим ссылку на последний уже виденный
// элемент буфера и сравниваем — стор всегда кладёт новый объект (иммутабельный push), так
// что смена ссылки на конце массива и есть «пришло новое событие».
//
// ВАЖНО: «новое» — это ВСЕ элементы после уже виденного, а не только хвост. Один update
// несёт несколько событий: resolveAdjudication пишет voteResolved, и сразу за ним
// assessShukh пишет shukhAssessed (engine/apply.go) — последним оказывается вовсе не
// исход разбора. Смотреть только на events.at(-1) значило бы не показать баннер никогда.
export function VoteOutcomeBanner({ events }: VoteOutcomeBannerProps) {
  const [outcome, setOutcome] = useState<Outcome | null>(null)
  // Инициализируем текущим последним событием синхронно при первом рендере — история,
  // уже лежавшая в буфере на момент монтирования (например, после реконнекта), не должна
  // сама по себе всплывать баннером: баннер — про «получили сейчас», а не про факт наличия.
  const lastSeen = useRef<GameEvent | undefined>(events[events.length - 1])

  useEffect(() => {
    const prev = lastSeen.current
    // Свежие — всё, что легло после уже виденного. prev === undefined значит «монтировались
    // на пустом буфере», тогда свежее — весь буфер. Если prev в буфере не найден, его
    // вытеснил EVENTS_CAP (больше сотни событий за один апдейт — практически невозможно):
    // тогда молча пересинхронизируемся, но НЕ проигрываем историю заново — иначе всплыл бы
    // баннер по давно закрытому разбору, ровно то, от чего защищает инициализация выше.
    const at = prev === undefined ? -1 : events.lastIndexOf(prev)
    const fresh = prev === undefined ? events : at >= 0 ? events.slice(at + 1) : []
    if (events.length > 0) lastSeen.current = events[events.length - 1]
    // Берём ПОСЛЕДНИЙ исход из порции: если бы их пришло два, актуален поздний.
    const resolved = fresh.filter((e) => e.type === 'voteResolved').at(-1)
    if (!resolved) return
    setOutcome({ code: resolved.code, overturned: resolved.overturned })
    // Таймер скрытия — во втором эффекте (ниже), завязанном на outcome, а не на events:
    // events меняется на КАЖДЫЙ игровой апдейт, и если бы clearTimeout висел на этом
    // эффекте, любой посторонний ход соперника до истечения VISIBLE_MS отменял бы скрытие
    // навсегда (эффект перезапускается → cleanup рвёт таймер → новый не ставится).
  }, [events])

  useEffect(() => {
    if (!outcome) return
    const id = setTimeout(() => setOutcome(null), VISIBLE_MS)
    return () => clearTimeout(id)
  }, [outcome])

  if (!outcome) return null

  return (
    <div className={styles.voteOutcomeBanner} role="status" data-testid="vote-outcome-banner">
      {outcome.overturned
        ? `ШУХ отклонён столом — штраф (Ш-8) переходит на предъявителя (R-8.6)`
        : `ШУХ подтверждён столом — штраф остаётся на том, кому его предъявили`}
    </div>
  )
}
