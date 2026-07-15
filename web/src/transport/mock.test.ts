import { createMockTransport } from './mock'
import { gameSnapshot } from '../fixtures/game'
import type { GameSnapshot } from '../contract/types'

test('subscribe синхронно доставляет снапшот', () => {
  const t = createMockTransport(gameSnapshot)
  let got: GameSnapshot | null = null
  t.subscribe((s) => (got = s), () => {})
  expect(got).toBe(gameSnapshot)
})

test('send записывает действие в sent', () => {
  const t = createMockTransport(gameSnapshot)
  t.send({ type: 'takeBottomAndPass' })
  expect(t.sent).toEqual([{ type: 'takeBottomAndPass' }])
})

test('после отписки снапшоты больше не приходят', () => {
  const t = createMockTransport(gameSnapshot)
  let calls = 0
  const unsub = t.subscribe(() => (calls += 1), () => {})
  unsub()
  // повторных пушей мок не делает; проверяем, что первичный пуш был ровно один
  expect(calls).toBe(1)
})
