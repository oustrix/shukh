import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { actionsEqual } from '../contract/types'

// Шаг сценария. 'auto' проигрывается сам (по таймеру), 'await' ждёт ожидаемое
// действие игрока. snapshot — состояние ПОСЛЕ шага; events эмитятся перед снапшотом.
export interface Step {
  kind: 'auto' | 'await'
  expect?: Action
  events: GameEvent[]
  snapshot: GameSnapshot
  delayMs?: number
}
export type Scenario = Step[]

// Планировщик auto-шагов — инъектируется в тестах (синхронный) ради детерминизма.
export type Scheduler = (fn: () => void, ms: number) => void
const defaultScheduler: Scheduler = (fn, ms) => {
  setTimeout(fn, ms)
}

// Скриптованный транспорт — двойник будущего ws.ts. Начальный (нулевой) auto-шаг
// эмитится СИНХРОННО при подписке (сервер сразу отдаёт стартовое состояние);
// последующие auto-шаги идут через планировщик. Предусловие: scenario[0].kind === 'auto'.
export function createScriptedTransport(scenario: Scenario, schedule: Scheduler = defaultScheduler): Transport {
  let index = 0
  let onSnapshot: ((s: GameSnapshot) => void) | null = null
  let onEvent: ((e: GameEvent) => void) | null = null

  function emit(step: Step) {
    step.events.forEach((e) => onEvent?.(e))
    onSnapshot?.(step.snapshot)
  }

  // Проигрывает подряд идущие auto-шаги через планировщик, пока не упрётся в await/конец.
  function scheduleAutos() {
    const step = scenario[index]
    if (!step || step.kind !== 'auto') return
    schedule(() => {
      index += 1
      emit(step)
      scheduleAutos()
    }, step.delayMs ?? 0)
  }

  return {
    subscribe(snap, ev) {
      onSnapshot = snap
      onEvent = ev
      // стартовое состояние — синхронно
      const first = scenario[index]
      if (first && first.kind === 'auto') {
        index += 1
        emit(first)
      }
      scheduleAutos()
      return () => {
        onSnapshot = null
        onEvent = null
      }
    },
    send(action) {
      const step = scenario[index]
      if (!step || step.kind !== 'await') return // не наш момент
      if (step.expect && !actionsEqual(step.expect, action)) return // офф-скрипт
      index += 1
      emit(step)
      scheduleAutos()
    },
  }
}
