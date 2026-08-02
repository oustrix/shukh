import { useState } from 'react'
import {
  cardKey,
  isCardPlayable,
  isLegal,
  isShukhTakeable,
  claimShukhInLegal,
} from '../../contract/types'
import { useGame } from '../../store/GameProvider'
import { selectSeats, selectView, selectLegal } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { ShukhZone } from '../table/ShukhZone'
import { ActionBar, type BarAction } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGame(selectView)
  const seats = useGame(selectSeats)
  const legal = useGame(selectLegal)
  const play = useGame((s) => s.play)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameBySeat = new Map(seats.map((s) => [s.seat, s.name]))
  const nameOf = (seat: number) => nameBySeat.get(seat) ?? `Игрок ${seat}`

  const playableKeys = new Set(view.hand.filter((c) => isCardPlayable(legal, c)).map(cardKey))
  const selectedCard = view.hand.find((c) => cardKey(c) === selectedKey) ?? null
  const canConfirm = selectedKey != null && playableKeys.has(selectedKey)
  const canTakeBottom = isLegal(legal, { type: 'takeBottomAndPass' })
  const yourZoneTakeable = isShukhTakeable(legal, view.you)
  const claim = claimShukhInLegal(legal)

  const confirmPlay = () => {
    if (!canConfirm || !selectedCard) return
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

  const declareOneCard = legal.find((a) => a.type === 'declareOneCard')
  const barActions: BarAction[] = [
    { label: 'Сходить', enabled: canConfirm, onClick: confirmPlay },
    {
      label: 'Взять низ',
      enabled: canTakeBottom,
      onClick: () => play({ type: 'takeBottomAndPass' }),
    },
    {
      label: 'Западло',
      enabled: isLegal(legal, { type: 'podkladkaWest' }),
      onClick: () => play({ type: 'podkladkaWest' }),
    },
    {
      // R-9.4.2.1: в эндшпиле §9.2 сброс 6(2)♥ — обязательный ход, без него стол встаёт.
      label: 'Сбросить Запад',
      enabled: isLegal(legal, { type: 'discardWest' }),
      onClick: () => play({ type: 'discardWest' }),
    },
    { label: 'ШУХ!', enabled: claim != null, onClick: () => claim && play(claim) },
    {
      // Настоящее объявление §6, а не клиент-локальный флаг: теперь Ш-11 ловится по правилам.
      label: 'Одна карта!',
      enabled: declareOneCard != null,
      onClick: () => declareOneCard && play(declareOneCard),
      pulse: true,
    },
  ]

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
      <ActionBar actions={barActions} />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={playableKeys}
        onSelect={onSelect}
      />
    </div>
  )
}
