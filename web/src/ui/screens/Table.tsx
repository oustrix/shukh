import { useState } from 'react'
import { isYourTurn } from '../../contract/types'
import { useGameStore } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ActionBar } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGameStore((s) => s.snapshot?.view ?? null)
  const seats = useGameStore((s) => s.snapshot?.seats ?? [])
  const play = useGameStore((s) => s.play)
  const [selected, setSelected] = useState<number | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameOf = (seat: number) => seats.find((s) => s.seat === seat)?.name ?? `Игрок ${seat}`

  return (
    <div className={styles.table}>
      <div className={styles.opponents}>
        {view.opponents.map((o) => (
          <OpponentSeat key={o.seat} name={nameOf(o.seat)} opponent={o} />
        ))}
      </div>
      <Con table={view.table} />
      <ActionBar
        yourTurn={isYourTurn(view)}
        onShukh={() => play({ type: 'claimShukh', target: view.turn, code: 2 })}
        onOneCard={() => {
          /* объявление «Одна карта!» (§6) — заглушка до Спеца 2 */
        }}
        onTakeBottom={() => play({ type: 'takeBottomAndPass' })}
      />
      <Hand
        cards={view.hand}
        selectedIndex={selected}
        onSelect={(i) => {
          setSelected(i)
          play({ type: 'playCard', card: view.hand[i] })
        }}
      />
    </div>
  )
}
