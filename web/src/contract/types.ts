// Ручные TS-зеркала типов движка. СИНХРОНИЗИРОВАТЬ ВРУЧНУЮ (W-3).
// Точное JSON-кодирование согласуется с DTO сервера Спеца 2, когда он появится.

// зеркало engine/card.go — синхронизировать вручную
export type Suit = '♠' | '♥' | '♦' | '♣' // Spades|Hearts|Diamonds|Clubs; ♦ — козырь (R-2.5)
export type Rank = number // 2..14; 11=J 12=Q 13=K 14=A (R-2.2)
export interface Card {
  suit: Suit
  rank: Rank
}

// зеркало engine/state.go — синхронизировать вручную
export type SeatID = number
export type Phase = 'playing' | 'finished'
export type EnforcementMode = 'guard' | 'middle' | 'culture'
export type ShukhCode = 2 | 3 | 11 | 12
export interface RuleSet {
  deckSize: 36 | 52
  podkladkaSnizu: boolean
  jokers: boolean
}
export interface TableCard {
  card: Card
  by: SeatID
}

// зеркало engine/view.go (SeatView, per-seat проекция, D-9) — синхронизировать вручную
export interface OpponentView {
  seat: SeatID
  handCount: number
  shukhPending: number
  live: boolean
}
export interface SeatView {
  rules: RuleSet
  mode: EnforcementMode
  phase: Phase
  you: SeatID
  turn: SeatID
  hand: Card[]
  shukhPending: number
  opponents: OpponentView[]
  table: TableCard[]
  discard: number
  talon: number
  live: Record<number, boolean>
  finish: SeatID[]
}

// Метаданные комнаты (Слой 1) — имена/готовность НЕ входят в engine.SeatView.
export interface SeatMeta {
  seat: SeatID
  name: string
  ready: boolean
}
export interface GameSnapshot {
  roomCode: string
  seats: SeatMeta[]
  view: SeatView | null // null в лобби (партия ещё не началась)
  legal: Action[] // легальные ходы текущего игрока (зеркало LegalActions); [] когда не наш ход
  shukhVote?: ShukhVote | null // активное голосование по ШУХу (R-8.6); скриптовано (W2-7)
}

// Голосование/оспаривание ШУХа (R-8.6). Это клиент/сервер-DTO Спеца 2, НЕ engine.SeatView:
// голоса и исход присылает сервер; на моке — сценарий (кворум по-настоящему не считаем, W2-7).
export interface ShukhVote {
  claimant: SeatID // кто предъявил ШУХ
  target: SeatID // на кого
  code: ShukhCode
  votes: { seat: SeatID; up: boolean }[] // голоса судящих (R-8.9); скриптованы
  outcome: 'upheld' | 'overturned' // overturned → Ш-8 предъявившему
  resolved: boolean // false — идёт голосование; true — показать исход
}

// зеркало engine/action.go — синхронизировать вручную
export type Action =
  | { type: 'playCard'; card: Card }
  | { type: 'takeBottomAndPass' }
  | { type: 'podkladkaWest' }
  | { type: 'claimShukh'; target: SeatID; code: ShukhCode }
  | { type: 'giveShukhCard'; card: Card }
  | { type: 'takeShukhCards'; seat: SeatID }

// зеркало engine/event.go + state.go — синхронизировать вручную
export type GameEvent =
  | { type: 'gameStarted'; turn: SeatID }
  | { type: 'cardPlayed'; seat: SeatID; card: Card }
  | { type: 'conClosed'; by: SeatID }
  | { type: 'conSwept'; cards: Card[] }
  | { type: 'playerFinished'; seat: SeatID; place: number }
  | { type: 'gameFinished'; finish: SeatID[] }
  | { type: 'cardsTaken'; seat: SeatID; cards: Card[] }
  | { type: 'podkladkaPlayed'; seat: SeatID; eater: SeatID }
  | { type: 'turnSkipped'; seat: SeatID }
  | { type: 'shukhAssessed'; offender: SeatID; code: ShukhCode }
  | { type: 'actionReverted'; seat: SeatID }
  | { type: 'shukhPaid'; offender: SeatID; from: SeatID; card: Card }
  | { type: 'shukhCardsTaken'; seat: SeatID; cards: Card[] }

// Хелперы уровня контракта (используются UI и транспортом).
export function isYourTurn(view: SeatView): boolean {
  return view.turn === view.you
}

// Стабильный ключ карты: в колоде (36/52) карты уникальны по рангу+масти.
export function cardKey(card: Card): string {
  return `${card.rank}${card.suit}`
}

// Каноничный ключ действия — для сравнения (легальность, сверка со сценарием).
function actionKey(a: Action): string {
  switch (a.type) {
    case 'playCard':
      return `playCard:${cardKey(a.card)}`
    case 'giveShukhCard':
      return `giveShukhCard:${cardKey(a.card)}`
    case 'claimShukh':
      return `claimShukh:${a.target}:${a.code}`
    case 'takeShukhCards':
      return `takeShukhCards:${a.seat}`
    default:
      return a.type // takeBottomAndPass | podkladkaWest
  }
}

export function actionsEqual(a: Action, b: Action): boolean {
  return actionKey(a) === actionKey(b)
}

export function isLegal(legal: Action[], action: Action): boolean {
  return legal.some((a) => actionsEqual(a, action))
}

export function isCardPlayable(legal: Action[], card: Card): boolean {
  return isLegal(legal, { type: 'playCard', card })
}

// Первый claimShukh в списке легальных (открыто ли ШУХ-окно). Клиент не судит —
// сервер кладёт конкретный предъявляемый ШУХ в legal, кнопка лишь его отправляет.
export function claimShukhInLegal(
  legal: Action[],
): Extract<Action, { type: 'claimShukh' }> | undefined {
  return legal.find((a): a is Extract<Action, { type: 'claimShukh' }> => a.type === 'claimShukh')
}

// Можно ли забрать свои отложенные ШУХ-карты (R-8.3 — только по завершении кона).
// Гейтится legal: сервер добавляет takeShukhCards, когда взятие законно.
export function isShukhTakeable(legal: Action[], seat: SeatID): boolean {
  return isLegal(legal, { type: 'takeShukhCards', seat })
}
