import type {
  Action,
  Card,
  GameEvent,
  GameSnapshot,
  SeatID,
  SeatMeta,
  SeatView,
  ShukhCode,
  Stage,
  VoteView,
} from './types'
import type { ProtocolError } from './transport'

// Расхождение ручных зеркал (W-3) — единственная поломка, которую мы обязаны заметить
// немедленно, поэтому кодек не «прощает» неизвестное, а падает.
export class WireError extends Error {
  constructor(message: string) {
    super(`wire: ${message}`)
    this.name = 'WireError'
  }
}

export type Decoded =
  | { kind: 'update'; snapshot: GameSnapshot; events: GameEvent[] }
  | { kind: 'ack'; reqId: string }
  | { kind: 'error'; reqId?: string; error: ProtocolError }

type Obj = Record<string, unknown>

function obj(v: unknown, what: string): Obj {
  if (typeof v !== 'object' || v === null || Array.isArray(v))
    throw new WireError(`${what} must be an object`)
  return v as Obj
}
function num(v: unknown, what: string): number {
  if (typeof v !== 'number' || Number.isNaN(v)) throw new WireError(`${what} must be a number`)
  return v
}
function str(v: unknown, what: string): string {
  if (typeof v !== 'string') throw new WireError(`${what} must be a string`)
  return v
}
function bool(v: unknown, what: string): boolean {
  if (typeof v !== 'boolean') throw new WireError(`${what} must be a boolean`)
  return v
}
function arr(v: unknown, what: string): unknown[] {
  if (!Array.isArray(v)) throw new WireError(`${what} must be an array`)
  return v
}

function decodeCard(v: unknown): Card {
  const o = obj(v, 'card')
  const suit = str(o.suit, 'card.suit')
  if (suit !== '♠' && suit !== '♥' && suit !== '♦' && suit !== '♣')
    throw new WireError(`unknown suit ${suit}`)
  return { suit, rank: num(o.rank, 'card.rank') }
}
const decodeCards = (v: unknown): Card[] => arr(v, 'cards').map(decodeCard)
const decodeSeats = (v: unknown): SeatID[] => arr(v, 'seats').map((s) => num(s, 'seat'))
const decodeCode = (v: unknown): ShukhCode => num(v, 'code') as ShukhCode

function decodeVote(v: unknown): VoteView {
  const o = obj(v, 'vote')
  return {
    claimant: num(o.claimant, 'vote.claimant'),
    target: num(o.target, 'vote.target'),
    code: decodeCode(o.code),
    voted: decodeSeats(o.voted),
  }
}

function decodeView(v: unknown): SeatView {
  const o = obj(v, 'view')
  const rules = obj(o.rules, 'view.rules')
  const mode = str(o.mode, 'view.mode')
  if (mode !== 'guard' && mode !== 'middle' && mode !== 'culture')
    throw new WireError(`unknown mode ${mode}`)
  const phase = str(o.phase, 'view.phase')
  if (phase !== 'playing' && phase !== 'finished') throw new WireError(`unknown phase ${phase}`)
  const deckSize = num(rules.deckSize, 'rules.deckSize')
  if (deckSize !== 36 && deckSize !== 52) throw new WireError(`unknown deck size ${deckSize}`)
  const live: Record<number, boolean> = {}
  for (const [k, val] of Object.entries(obj(o.live, 'view.live')))
    live[Number(k)] = bool(val, 'view.live[]')
  const view: SeatView = {
    rules: {
      deckSize,
      podkladkaSnizu: bool(rules.podkladkaSnizu, 'rules.podkladkaSnizu'),
      jokers: bool(rules.jokers, 'rules.jokers'),
    },
    mode,
    phase,
    you: num(o.you, 'view.you'),
    turn: num(o.turn, 'view.turn'),
    hand: decodeCards(o.hand),
    shukhPending: num(o.shukhPending, 'view.shukhPending'),
    opponents: arr(o.opponents, 'view.opponents').map((x) => {
      const p = obj(x, 'opponent')
      return {
        seat: num(p.seat, 'opponent.seat'),
        handCount: num(p.handCount, 'opponent.handCount'),
        shukhPending: num(p.shukhPending, 'opponent.shukhPending'),
        live: bool(p.live, 'opponent.live'),
      }
    }),
    table: arr(o.table, 'view.table').map((x) => {
      const t = obj(x, 'tableCard')
      return { card: decodeCard(t.card), by: num(t.by, 'tableCard.by') }
    }),
    discard: num(o.discard, 'view.discard'),
    talon: num(o.talon, 'view.talon'),
    live,
    finish: decodeSeats(o.finish),
  }
  if (o.vote !== undefined) view.vote = decodeVote(o.vote)
  return view
}

function decodeAction(v: unknown): Action {
  const o = obj(v, 'action')
  const type = str(o.type, 'action.type')
  switch (type) {
    case 'playCard':
      return { type, card: decodeCard(o.card) }
    case 'takeBottomAndPass':
    case 'podkladkaWest':
    case 'discardWest':
      return { type }
    case 'claimShukh':
      return { type, target: num(o.target, 'target'), code: decodeCode(o.code) }
    case 'giveShukhCard':
      return { type, card: decodeCard(o.card) }
    case 'takeShukhCards':
      return { type, seat: num(o.seat, 'seat') }
    case 'declareOneCard':
      return { type, seat: num(o.seat, 'seat') }
    case 'askCount':
    case 'askAboutWest':
      return { type, target: num(o.target, 'target') }
    case 'claimSubjective':
      return {
        type,
        claimant: num(o.claimant, 'claimant'),
        target: num(o.target, 'target'),
        code: decodeCode(o.code),
      }
    case 'vote': {
      const vote = str(o.vote, 'vote')
      if (vote !== 'forShukh' && vote !== 'againstShukh')
        throw new WireError(`unknown vote ${vote}`)
      return { type, vote }
    }
    default:
      throw new WireError(`unknown action type ${type} — зеркала разошлись с engine/action.go`)
  }
}

function decodeEvent(v: unknown): GameEvent {
  const o = obj(v, 'event')
  const type = str(o.type, 'event.type')
  switch (type) {
    case 'gameStarted':
      return { type, turn: num(o.turn, 'turn') }
    case 'cardPlayed':
      return { type, seat: num(o.seat, 'seat'), card: decodeCard(o.card) }
    case 'conClosed':
      return { type, by: num(o.by, 'by') }
    case 'conSwept':
      return { type, cards: decodeCards(o.cards) }
    case 'playerFinished':
      return { type, seat: num(o.seat, 'seat'), place: num(o.place, 'place') }
    case 'gameFinished':
      return { type, finish: decodeSeats(o.finish) }
    case 'cardsTaken':
      return { type, seat: num(o.seat, 'seat'), cards: decodeCards(o.cards) }
    case 'podkladkaPlayed':
      return { type, seat: num(o.seat, 'seat'), eater: num(o.eater, 'eater') }
    case 'turnSkipped':
    case 'actionReverted':
    case 'oneCardDeclared':
    case 'westDiscarded':
      return { type, seat: num(o.seat, 'seat') }
    case 'shukhAssessed':
      return { type, offender: num(o.offender, 'offender'), code: decodeCode(o.code) }
    case 'shukhPaid':
      return {
        type,
        offender: num(o.offender, 'offender'),
        from: num(o.from, 'from'),
        card: decodeCard(o.card),
      }
    case 'shukhCardsTaken':
      return { type, seat: num(o.seat, 'seat'), cards: decodeCards(o.cards) }
    case 'voteOpened':
      return {
        type,
        claimant: num(o.claimant, 'claimant'),
        target: num(o.target, 'target'),
        code: decodeCode(o.code),
      }
    case 'voteResolved':
      return { type, code: decodeCode(o.code), overturned: bool(o.overturned, 'overturned') }
    default:
      throw new WireError(`unknown event type ${type} — зеркала разошлись с engine/event.go`)
  }
}

function decodeStage(v: unknown): Stage {
  const s = str(v, 'stage')
  if (s !== 'lobby' && s !== 'playing' && s !== 'finished')
    throw new WireError(`unknown stage ${s}`)
  return s
}

function decodeRoster(v: unknown): SeatMeta[] {
  if (v === undefined) return []
  return arr(v, 'roster').map((x) => {
    const m = obj(x, 'seatMeta')
    return { seat: num(m.seat, 'seat'), name: str(m.name, 'name') }
  })
}

export function decodeServerMsg(raw: unknown): Decoded {
  const o = obj(raw, 'message')
  switch (str(o.type, 'type')) {
    case 'update': {
      const snapshot: GameSnapshot = {
        roomCode: str(o.roomCode, 'roomCode'),
        you: num(o.you, 'you'),
        stage: decodeStage(o.stage),
        host: num(o.host, 'host'),
        seats: decodeRoster(o.roster),
        view: o.view === undefined || o.view === null ? null : decodeView(o.view),
        legal: o.legal === undefined ? [] : arr(o.legal, 'legal').map(decodeAction),
      }
      if (o.voteDeadline !== undefined) snapshot.voteDeadline = num(o.voteDeadline, 'voteDeadline')
      const events = o.events === undefined ? [] : arr(o.events, 'events').map(decodeEvent)
      return { kind: 'update', snapshot, events }
    }
    case 'ack':
      return { kind: 'ack', reqId: str(o.reqId, 'reqId') }
    case 'error':
      return {
        kind: 'error',
        reqId: o.reqId === undefined ? undefined : str(o.reqId, 'reqId'),
        error: {
          code: str(o.code, 'code'),
          message: o.message === undefined ? '' : str(o.message, 'message'),
        },
      }
    default:
      throw new WireError(`unknown envelope type ${String(o.type)}`)
  }
}

// Форма действия на проводе совпадает с TS-типом один-в-один (см. decodeAction в
// server/protocol.go), поэтому кодирование тождественно. Функция существует как явный
// шов: если формы разойдутся, правка будет здесь, а не по всему UI.
export function encodeAction(a: Action): unknown {
  return a
}
