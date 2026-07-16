import { useState, useEffect } from 'react'
import {
  cardKey,
  isCardPlayable,
  isLegal,
  isShukhTakeable,
  claimShukhInLegal,
} from '../../contract/types'
import { useGameStore, selectSeats, selectView, selectLegal } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ShukhZone } from '../table/ShukhZone'
import { ActionBar } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGameStore(selectView)
  const seats = useGameStore(selectSeats)
  const legal = useGameStore(selectLegal)
  const play = useGameStore((s) => s.play)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [announced, setAnnounced] = useState(false)
  const handLen = view?.hand.length ?? 0
  useEffect(() => {
    if (handLen !== 1) setAnnounced(false)
  }, [handLen])

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameBySeat = new Map(seats.map((s) => [s.seat, s.name]))
  const nameOf = (seat: number) => nameBySeat.get(seat) ?? `Игрок ${seat}`

  const playableKeys = new Set(view.hand.filter((c) => isCardPlayable(legal, c)).map(cardKey))
  const selectedCard = view.hand.find((c) => cardKey(c) === selectedKey) ?? null
  const canConfirm = selectedCard != null && isCardPlayable(legal, selectedCard)
  const canTakeBottom = isLegal(legal, { type: 'takeBottomAndPass' })
  const yourZoneTakeable = isShukhTakeable(legal, view.you)
  const claim = claimShukhInLegal(legal)
  const owesOneCard = (view.live[view.you] ?? false) && view.hand.length === 1 && !announced

  const confirmPlay = () => {
    if (!selectedCard) return
    play({ type: 'playCard', card: selectedCard })
    setSelectedKey(null)
  }
  const onSelect = (card: (typeof view.hand)[number]) => {
    const key = cardKey(card)
    if (key === selectedKey) {
      confirmPlay()
      return
    }
    setSelectedKey(key)
  }

  return (
    <div className={styles.table}>
      <div className={styles.opponents}>
        {view.opponents.map((o) => (
          <OpponentSeat key={o.seat} name={nameOf(o.seat)} opponent={o} />
        ))}
      </div>
      <Con table={view.table} />
      <ShukhZone
        count={view.shukhPending}
        takeable={yourZoneTakeable}
        onTake={() => play({ type: 'takeShukhCards', seat: view.you })}
        label={`Ваша ШУХ-зона: ${view.shukhPending}`}
      />
      <ActionBar
        canConfirm={canConfirm}
        onConfirm={confirmPlay}
        canTakeBottom={canTakeBottom}
        onTakeBottom={() => play({ type: 'takeBottomAndPass' })}
        canShukh={claim != null}
        onShukh={() => claim && play(claim)}
        owesOneCard={owesOneCard}
        onOneCard={() => setAnnounced(true)}
      />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={playableKeys}
        onSelect={onSelect}
      />
    </div>
  )
}
