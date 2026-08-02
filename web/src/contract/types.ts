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

// Коды ШУХов (§7 правил): объективные ловит движок, субъективные идут через голосование R-8.6.
export type ShukhCode = 2 | 3 | 6 | 8 | 9 | 10 | 11 | 12

// Полный перечень кодов ШУХ как рантайм-значение — единственное место, откуда его
// берут (напр. кодек для проверки code с провода); саму цифру нигде больше не дублируем.
export const ALL_SHUKH_CODES = [2, 3, 6, 8, 9, 10, 11, 12] as const satisfies readonly ShukhCode[]

// Ш-6 «завис» (R-8.4), Ш-9 «зря крикнул» (R-8.7), Ш-10 «небрежность» (R-8.8) —
// единственные, что предъявляются вручную через claimSubjective.
export const SUBJECTIVE_CODES = [6, 9, 10] as const satisfies readonly ShukhCode[]

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
// Публичная сводка открытого разбора R-8.6 (зеркало engine.VoteView, §8.3 Слоя 2).
// voted — ФАКТ голосования, без содержания: бюллетень тайный до резолва (§8.4).
export interface VoteView {
  claimant: SeatID
  target: SeatID
  code: ShukhCode
  voted: SeatID[]
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
  vote?: VoteView // открытый разбор R-8.6; отсутствует, когда разбора нет
}

export type Stage = 'lobby' | 'playing' | 'finished'

// Метаданные комнаты (Слой 1): game.SeatMeta это {Seat, Name} — «готовности» в Слое 1 нет.
export interface SeatMeta {
  seat: SeatID
  name: string
}
export interface GameSnapshot {
  roomCode: string
  you: SeatID // своё место известно и в лобби, где view === null
  stage: Stage
  host: SeatID // чьи Старт/настройки (мигрирует, L2-3)
  seats: SeatMeta[]
  view: SeatView | null // null в лобби (партия ещё не началась)
  legal: Action[] // легальные ходы текущего игрока (зеркало LegalActions); [] когда не наш ход
  voteDeadline?: number // unix-мс, пока идёт разбор R-8.6
}

// зеркало engine/action.go — синхронизировать вручную
export type Action =
  | { type: 'playCard'; card: Card }
  | { type: 'takeBottomAndPass' }
  | { type: 'podkladkaWest' }
  | { type: 'discardWest' }
  | { type: 'claimShukh'; target: SeatID; code: ShukhCode }
  | { type: 'giveShukhCard'; card: Card }
  | { type: 'takeShukhCards'; seat: SeatID }
  | { type: 'declareOneCard'; seat: SeatID }
  | { type: 'askCount'; target: SeatID }
  | { type: 'askAboutWest'; target: SeatID }
  | { type: 'claimSubjective'; claimant: SeatID; target: SeatID; code: ShukhCode }
  | { type: 'vote'; vote: 'forShukh' | 'againstShukh' }

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
  | { type: 'oneCardDeclared'; seat: SeatID }
  | { type: 'westDiscarded'; seat: SeatID }
  | { type: 'voteOpened'; claimant: SeatID; target: SeatID; code: ShukhCode }
  | { type: 'voteResolved'; code: ShukhCode; overturned: boolean }

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
    case 'askCount':
    case 'askAboutWest':
      return `${a.type}:${a.target}`
    case 'declareOneCard':
      return `declareOneCard:${a.seat}`
    case 'claimSubjective':
      return `claimSubjective:${a.target}:${a.code}`
    case 'vote':
      return `vote:${a.vote}`
    default:
      return a.type // takeBottomAndPass | podkladkaWest | discardWest
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

// Ключи карт, которые можно отдать в оплату ШУХа (§8). Непустой набор = гейт открыт
// ИМЕННО на нас: движок перечисляет giveShukhCard только платящему (engine/legal.go),
// остальным при открытом гейте достаётся пустой legal. Последней карты в наборе не
// будет — её отдавать нельзя (R-8.1.1/I-2), и движок её не предлагает.
export function giveShukhKeys(legal: Action[]): Set<string> {
  return new Set(
    legal.filter((a) => a.type === 'giveShukhCard').map((a) => cardKey(a.card)),
  )
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
