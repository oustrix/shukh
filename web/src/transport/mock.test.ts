import { createMockTransport } from './mock'
import { gameSnapshot } from '../fixtures/game'
import type { GameSnapshot } from '../contract/types'

test('subscribe синхронно доставляет снапшот', () => {
  const t = createMockTransport(gameSnapshot)
  let got: GameSnapshot | null = null
  t.subscribe({
    onSnapshot: (s) => (got = s),
    onEvent: () => {},
    onStatus: () => {},
  })
  expect(got).toBe(gameSnapshot)
})

test('send записывает действие в sent', () => {
  const t = createMockTransport(gameSnapshot)
  t.send({ type: 'takeBottomAndPass' })
  expect(t.sent).toEqual([{ type: 'takeBottomAndPass' }])
})

test('subscribe возвращает функцию отписки; снапшот приходит ровно один раз (emit-once)', () => {
  const t = createMockTransport(gameSnapshot)
  let calls = 0
  const unsub = t.subscribe({
    onSnapshot: () => (calls += 1),
    onEvent: () => {},
    onStatus: () => {},
  })
  expect(typeof unsub).toBe('function')
  expect(calls).toBe(1) // мок пушит один раз при подписке; повторных пушей нет
  expect(() => unsub()).not.toThrow()
})
