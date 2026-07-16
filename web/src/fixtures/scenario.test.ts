import { createScriptedTransport, type Scheduler } from '../transport/scripted'
import { demoScenario } from './scenario'
import type { GameSnapshot } from '../contract/types'
import { claimShukhInLegal } from '../contract/types'

const sync: Scheduler = (fn) => fn()

// Прогоняет сценарий синхронно, собирая все пуш-снапшоты; на каждом await-шаге
// отправляет его expect (эмулируем игрока, идущего строго по скрипту).
function runToEnd(): GameSnapshot[] {
  const snaps: GameSnapshot[] = []
  const t = createScriptedTransport(demoScenario, sync)
  t.subscribe(
    (s) => snaps.push(s),
    () => {},
  )
  // синхронно докручиваем await-шаги их ожидаемыми действиями
  for (const step of demoScenario) {
    if (step.kind === 'await') t.send(step.expect)
  }
  return snaps
}

test('в ходе сценария открывается ШУХ-окно (claimShukh в legal)', () => {
  const snaps = runToEnd()
  const withClaim = snaps.filter((s) => claimShukhInLegal(s.legal) != null)
  expect(withClaim.length).toBeGreaterThan(0)
  expect(claimShukhInLegal(withClaim[0].legal)).toMatchObject({ target: 2, code: 11 })
})

test('после предъявления ШУХа поднимается голосование, затем исход', () => {
  const snaps = runToEnd()
  const voting = snaps.filter((s) => s.shukhVote && !s.shukhVote.resolved)
  const resolved = snaps.filter((s) => s.shukhVote?.resolved)
  expect(voting.length).toBeGreaterThan(0)
  expect(resolved.length).toBeGreaterThan(0)
  expect(resolved[0].shukhVote?.outcome).toBe('upheld')
})

test('оплата ШУХа: нарушитель (Вера, seat 2) получает 2 отложенные карты; ваша рука убыла до 1', () => {
  const snaps = runToEnd()
  const last = snaps[snaps.length - 1]
  const vera = last.view?.opponents.find((o) => o.seat === 2)
  expect(vera?.shukhPending).toBe(2)
  expect(last.view?.hand.length).toBe(1)
  expect(last.shukhVote ?? null).toBeNull() // модалка закрыта после оплаты
})
