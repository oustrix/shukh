import type { Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'
import { actionsEqual } from '../contract/types'

// Шаг сценария. 'auto' проигрывается сам (по таймеру), 'await' ждёт ожидаемое
// действие игрока. snapshot — состояние ПОСЛЕ шага; events эмитятся перед снапшотом.
interface StepBase {
  events: GameEvent[]
  snapshot: GameSnapshot
}
export interface AutoStep extends StepBase {
  kind: 'auto'
  delayMs?: number // пауза перед проигрыванием
}
export interface AwaitStep extends StepBase {
  kind: 'await'
  expect: Action // ожидаемое действие игрока — обязателен на await-шаге
}
export type Step = AutoStep | AwaitStep
export type Scenario = Step[]

// Планировщик auto-шагов — инъектируется в тестах (синхронный) ради детерминизма.
// Возвращает функцию отмены запланированного вызова (или void, если отменять нечего).
export type Scheduler = (fn: () => void, ms: number) => (() => void) | void
const defaultScheduler: Scheduler = (fn, ms) => {
  const id = setTimeout(fn, ms)
  return () => clearTimeout(id)
}

// Скриптованный транспорт — двойник будущего ws.ts. Начальный (нулевой) auto-шаг
// эмитится СИНХРОННО при подписке (сервер сразу отдаёт стартовое состояние);
// последующие auto-шаги идут через планировщик. Предусловие: scenario[0].kind === 'auto'.
export function createScriptedTransport(
  scenario: Scenario,
  schedule: Scheduler = defaultScheduler,
): Transport {
  let index = 0
  let onSnapshot: ((s: GameSnapshot) => void) | null = null
  let onEvent: ((e: GameEvent) => void) | null = null
  let cancelPending: (() => void) | void // отмена ещё не проигранного auto-шага

  function emit(step: Step) {
    step.events.forEach((e) => onEvent?.(e))
    onSnapshot?.(step.snapshot)
  }

  // Проигрывает подряд идущие auto-шаги через планировщик, пока не упрётся в await/конец.
  function scheduleAutos() {
    const step = scenario[index]
    if (!step || step.kind !== 'auto') return
    cancelPending = schedule(() => {
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
        // TODO(ws.ts): отмену таймера юнит-тестом не покрыли — текущие тесты
        // синхронны (отменять нечего). Добавить тест с фейковыми таймерами,
        // когда появится реальный transport/ws.ts с keepalive/reconnect.
        if (typeof cancelPending === 'function') cancelPending() // остановить таймер-цепочку
      }
    },
    send(action) {
      const step = scenario[index]
      if (!step || step.kind !== 'await') return // не наш момент
      if (!actionsEqual(step.expect, action)) return // офф-скрипт
      index += 1
      emit(step)
      scheduleAutos()
    },
  }
}
