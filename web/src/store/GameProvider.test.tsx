import { StrictMode } from 'react'
import { render, cleanup } from '@testing-library/react'
import type { Transport } from '../contract/transport'

// createWsTransport одноразовый: его subscribe() необратимо глушится через close()/teardown
// (stopped больше никогда не сбрасывается — см. transport/ws.ts). Двойник ниже воспроизводит
// именно это свойство, чтобы тест ловил повторное использование мёртвого инстанса.
const created: Array<{ code: string; closed: boolean }> = []

vi.mock('../transport/ws', () => ({
  createWsTransport: (code: string): Transport => {
    const entry = { code, closed: false }
    created.push(entry)
    return {
      subscribe: () => () => {
        entry.closed = true
      },
      send: () => {},
      close: () => {
        entry.closed = true
      },
    }
  },
}))

import { GameProvider, useGame } from './GameProvider'
import { selectConn } from './game'

function Probe() {
  const conn = useGame(selectConn)
  return <div data-testid="conn">{conn}</div>
}

beforeEach(() => {
  created.length = 0
})

afterEach(() => {
  cleanup()
})

test('StrictMode: после двойного цикла монтирования жив ровно один транспорт', () => {
  render(
    <StrictMode>
      <GameProvider code="ABCD">
        <Probe />
      </GameProvider>
    </StrictMode>,
  )
  const live = created.filter((e) => !e.closed)
  expect(live).toHaveLength(1)
  expect(live[0].code).toBe('ABCD')
})

test('размонтирование закрывает транспорт', () => {
  const { unmount } = render(
    <GameProvider code="ABCD">
      <Probe />
    </GameProvider>,
  )
  expect(created.some((e) => !e.closed)).toBe(true)
  unmount()
  expect(created.every((e) => e.closed)).toBe(true)
})

test('смена code закрывает старый транспорт и создаёт новый — старый инстанс не переиспользуется', () => {
  const { rerender } = render(
    <GameProvider code="AAAA">
      <Probe />
    </GameProvider>,
  )
  rerender(
    <GameProvider code="BBBB">
      <Probe />
    </GameProvider>,
  )
  expect(created.map((e) => e.code)).toEqual(['AAAA', 'BBBB'])
  expect(created[0].closed).toBe(true)
  expect(created[1].closed).toBe(false)
})

test('useGame вне GameProvider бросает', () => {
  expect(() => render(<Probe />)).toThrow('useGame вне GameProvider')
})
