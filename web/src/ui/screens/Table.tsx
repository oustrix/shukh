import { useState } from 'react'
import {
  cardKey,
  giveShukhKeys,
  isCardPlayable,
  isLegal,
  isShukhTakeable,
  claimShukhInLegal,
} from '../../contract/types'
import { useGame } from '../../store/GameProvider'
import { selectSeats, selectView, selectLegal, selectVote, selectVoteDeadline, selectEvents } from '../../store/game'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { SeatMenu } from '../table/SeatMenu'
import { ShukhZone } from '../table/ShukhZone'
import { ShukhVoteModal } from '../table/ShukhVoteModal'
import { VoteOutcomeBanner } from '../table/VoteOutcomeBanner'
import { ActionBar, type BarAction } from '../table/ActionBar'
import styles from '../table/Table.module.css'

export function Table() {
  const view = useGame(selectView)
  const seats = useGame(selectSeats)
  const legal = useGame(selectLegal)
  const vote = useGame(selectVote)
  const voteDeadline = useGame(selectVoteDeadline)
  const events = useGame(selectEvents)
  const play = useGame((s) => s.play)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [menuSeat, setMenuSeat] = useState<number | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameBySeat = new Map(seats.map((s) => [s.seat, s.name]))
  const nameOf = (seat: number) => nameBySeat.get(seat) ?? `Игрок ${seat}`

  // Оплата ШУХа (§8). При открытом гейте движок (engine/legal.go, ветка s.Pending != nil)
  // кладёт платящему ТОЛЬКО GiveShukhCard — по одному на каждую отдаваемую карту; всем
  // остальным за столом достаётся пустой legal. Отсюда и признак «плачу я»: он выводится
  // из legal, а не из собственного счёта правил (клиент правил не считает). Последнюю
  // карту отдавать нельзя (R-8.1.1/I-2) — движок её просто не предложит, так что
  // отдельной проверки тут не нужно: выбираемо ровно то, что перечислено.
  const giveKeys = giveShukhKeys(legal)
  const paying = giveKeys.size > 0

  const playableKeys = new Set(view.hand.filter((c) => isCardPlayable(legal, c)).map(cardKey))
  // Один и тот же механизм выбора в руке обслуживает и ход, и оплату — параллельного нет.
  const selectableKeys = paying ? giveKeys : playableKeys
  const selectedCard = view.hand.find((c) => cardKey(c) === selectedKey) ?? null
  const canConfirm = selectedKey != null && selectableKeys.has(selectedKey)
  const canTakeBottom = isLegal(legal, { type: 'takeBottomAndPass' })
  const yourZoneTakeable = isShukhTakeable(legal, view.you)
  const claim = claimShukhInLegal(legal)

  const confirmPlay = () => {
    if (!canConfirm || !selectedCard) return
    play(paying ? { type: 'giveShukhCard', card: selectedCard } : { type: 'playCard', card: selectedCard })
    setSelectedKey(null)
  }

  // Подпись состояния: игрок обязан понимать, чего от него ждут — и, что не менее
  // важно, когда от него не ждут ничего. Всё выводится из снапшота, правил не считаем.
  const statusText = paying
    ? 'Оплатите ШУХ: отдайте карту'
    : vote
      ? 'Идёт разбор ШУХа (R-8.6)'
      : legal.length === 0
        ? 'Сейчас вы ничего не можете — ждём других игроков'
        : view.turn === view.you
          ? 'Ваш ход'
          : `Ходит ${nameOf(view.turn)}`
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
    { label: paying ? 'Отдать карту' : 'Сходить', enabled: canConfirm, onClick: confirmPlay },
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
      {/* Живёт по событию, а не по view.vote: сервер обнуляет view.vote тем же апдейтом,
          что несёт voteResolved, поэтому исход нельзя вешать на состояние модалки —
          она к этому моменту уже размонтирована. Видна всем за столом, не только спорившим. */}
      <VoteOutcomeBanner events={events} />
      <div className={styles.opponents}>
        {view.opponents.map((o) => (
          <OpponentSeat
            key={o.seat}
            name={nameOf(o.seat)}
            opponent={o}
            menuOpen={menuSeat === o.seat}
            onToggleMenu={() => setMenuSeat(menuSeat === o.seat ? null : o.seat)}
          >
            <SeatMenu
              seat={o.seat}
              name={nameOf(o.seat)}
              you={view.you}
              legal={legal}
              onAction={play}
              onClose={() => setMenuSeat(null)}
            />
          </OpponentSeat>
        ))}
      </div>
      <Con table={view.table} />
      <ShukhZone
        count={view.shukhPending}
        takeable={yourZoneTakeable}
        onTake={() => play({ type: 'takeShukhCards', seat: view.you })}
        label={`Ваша ШУХ-зона: ${view.shukhPending}`}
      />
      <div className={styles.status} role="status" data-testid="table-status">
        {statusText}
      </div>
      <ActionBar actions={barActions} />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={selectableKeys}
        onSelect={onSelect}
      />
      {vote && (
        <ShukhVoteModal
          vote={vote}
          deadline={voteDeadline}
          legal={legal}
          nameOf={nameOf}
          onVote={(v) => play({ type: 'vote', vote: v })}
        />
      )}
    </div>
  )
}
