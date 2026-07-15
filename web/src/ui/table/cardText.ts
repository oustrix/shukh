import type { Card, Suit } from '../../contract/types'

const FACE: Record<number, string> = { 11: 'J', 12: 'Q', 13: 'K', 14: 'A' }

export function rankLabel(rank: number): string {
  return FACE[rank] ?? String(rank)
}

export function cardLabel(card: Card): string {
  return rankLabel(card.rank) + card.suit
}

export function isRedSuit(suit: Suit): boolean {
  return suit === '♥' || suit === '♦'
}
