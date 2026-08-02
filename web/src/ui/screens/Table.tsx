import { useState } from 'react'
import {
  cardKey,
  giveShukhKeys,
  isCardPlayable,
  isLegal,
  isShukhTakeable,
  claimShukhInLegal,
  nameOfSeat,
} from '../../contract/types'
import { useGame } from '../../store/GameProvider'
import {
  selectSeats,
  selectView,
  selectLegal,
  selectVote,
  selectVoteDeadline,
  selectEvents,
} from '../../store/game'
import { useDispatch } from '../useDispatch'
import { Hand } from '../table/Hand'
import { Con } from '../table/Con'
import { OpponentSeat } from '../table/OpponentSeat'
import { SeatMenu } from '../table/SeatMenu'
import { ShukhZone } from '../table/ShukhZone'
import { ShukhVoteModal } from '../table/ShukhVoteModal'
import { VoteOutcomeBanner } from '../table/VoteOutcomeBanner'
import { ActionBar, type BarAction } from '../table/ActionBar'
import styles from '../table/Table.module.css'

// Ходы без карты и без цели: подпись — единственное, чем они отличаются друг от друга.
// «Сбросить Запад» (discardWest) — обязательный ход эндшпиля §9.2 (R-9.4.2.1): без него
// стол на двоих встаёт намертво.
type SimpleActionType = 'takeBottomAndPass' | 'podkladkaWest' | 'discardWest'
const SIMPLE_ACTIONS: [SimpleActionType, string][] = [
  ['takeBottomAndPass', 'Взять низ'],
  ['podkladkaWest', 'Западло'],
  ['discardWest', 'Сбросить Запад'],
]

export function Table() {
  const view = useGame(selectView)
  const seats = useGame(selectSeats)
  const legal = useGame(selectLegal)
  const vote = useGame(selectVote)
  const voteDeadline = useGame(selectVoteDeadline)
  const events = useGame(selectEvents)
  const { online, send } = useDispatch()
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [menuSeat, setMenuSeat] = useState<number | null>(null)

  if (!view) return <div className={styles.con}>Загрузка стола…</div>

  const nameOf = (seat: number) => nameOfSeat(seats, seat)

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
  const legalKeys = paying ? giveKeys : playableKeys
  // При обрыве рука неинтерактивна, но УЖЕ выбранная карта остаётся выделенной: выбор —
  // это намерение игрока, и терять его из-за мигнувшей связи незачем. Поэтому «что
  // законно» (legalKeys, из legal) и «что можно трогать сейчас» (handKeys) — разное.
  const handKeys = online ? legalKeys : new Set<string>()
  const selectedCard = view.hand.find((c) => cardKey(c) === selectedKey) ?? null
  const canConfirm = selectedKey != null && legalKeys.has(selectedKey)
  const yourZoneTakeable = isShukhTakeable(legal, view.you)
  const claim = claimShukhInLegal(legal)

  const confirmPlay = () => {
    if (!canConfirm || !selectedCard) return
    // Сброс выбора — только если ход реально уехал: отброшенная при обрыве отправка
    // раньше успевала стереть выделение, и игрок видел лишь исчезнувшую подсветку.
    const sent = send(
      paying ? { type: 'giveShukhCard', card: selectedCard } : { type: 'playCard', card: selectedCard },
    )
    if (sent) setSelectedKey(null)
  }

  // Подпись состояния: игрок обязан понимать, чего от него ждут — и, что не менее
  // важно, когда от него не ждут ничего. Всё выводится из снапшота, правил не считаем.
  const statusText = !online
    ? 'Связь потеряна — действия недоступны, ждём переподключения'
    : paying
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
    { label: paying ? 'Отдать карту' : 'Сходить', enabled: online && canConfirm, onClick: confirmPlay },
    // Бескарточные ходы отличаются только подписью — держим их данными, чтобы
    // следующий такой добавлялся строкой, а не копией блока.
    ...SIMPLE_ACTIONS.map(([type, label]) => ({
      label,
      enabled: online && isLegal(legal, { type }),
      onClick: () => send({ type }),
    })),
    { label: 'ШУХ!', enabled: online && claim != null, onClick: () => claim && send(claim) },
    {
      // Настоящее объявление §6, а не клиент-локальный флаг: теперь Ш-11 ловится по правилам.
      label: 'Одна карта!',
      enabled: online && declareOneCard != null,
      onClick: () => declareOneCard && send(declareOneCard),
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
              onAction={send}
              onClose={() => setMenuSeat(null)}
            />
          </OpponentSeat>
        ))}
      </div>
      <Con table={view.table} />
      <ShukhZone
        count={view.shukhPending}
        takeable={online && yourZoneTakeable}
        onTake={() => send({ type: 'takeShukhCards', seat: view.you })}
        label={`Ваша ШУХ-зона: ${view.shukhPending}`}
      />
      <div className={styles.status} role="status" data-testid="table-status">
        {statusText}
      </div>
      <ActionBar actions={barActions} />
      <Hand
        cards={view.hand}
        selectedKey={selectedKey}
        playableKeys={handKeys}
        onSelect={onSelect}
      />
      {vote && (
        <ShukhVoteModal
          vote={vote}
          deadline={voteDeadline}
          legal={legal}
          nameOf={nameOf}
          onVote={(v) => send({ type: 'vote', vote: v })}
          disabled={!online}
        />
      )}
    </div>
  )
}
