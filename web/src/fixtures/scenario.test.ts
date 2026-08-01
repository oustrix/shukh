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
  t.subscribe({
    onSnapshot: (s) => snaps.push(s),
    onEvent: () => {},
    onStatus: () => {},
  })
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

test('после предъявления ШУХа открывается разбор (SeatView.vote), затем закрывается', () => {
  const snaps = runToEnd()
  const voteIdx = snaps.findIndex((s) => s.view?.vote != null)
  expect(voteIdx).toBeGreaterThanOrEqual(0)
  expect(snaps[voteIdx].view?.vote).toMatchObject({ claimant: 0, target: 2, code: 11 })
  const closedAfter = snaps.slice(voteIdx + 1).some((s) => s.view?.vote == null)
  expect(closedAfter).toBe(true) // разбор закрылся (voteResolved, §8.3)
})

test('оплата ШУХа: нарушитель (Вера, seat 2) получает 2 отложенные карты; ваша рука убыла до 1', () => {
  const snaps = runToEnd()
  const last = snaps[snaps.length - 1]
  const vera = last.view?.opponents.find((o) => o.seat === 2)
  expect(vera?.shukhPending).toBe(2)
  expect(last.view?.hand.length).toBe(1)
  expect(last.view?.vote ?? null).toBeNull() // разбор закрыт после оплаты
})
