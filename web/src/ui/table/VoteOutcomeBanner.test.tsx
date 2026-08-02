import { act, render, screen } from '@testing-library/react'
import { VoteOutcomeBanner } from './VoteOutcomeBanner'
import type { GameEvent } from '../../contract/types'

const voteResolvedConfirmed: GameEvent = { type: 'voteResolved', code: 6, overturned: false }
const voteResolvedOverturned: GameEvent = { type: 'voteResolved', code: 6, overturned: true }

// Каждый .test — НОВЫЙ объект события (как это делает store/game.ts: events: [...s.events, event]),
// иначе сравнение по ссылке в компоненте не отличило бы «новое» событие от «уже виденного».
function fresh(e: GameEvent): GameEvent {
  return { ...e }
}

describe('баннер исхода разбора (R-8.6)', () => {
  it('ничего не рендерит без событий', () => {
    render(<VoteOutcomeBanner events={[]} />)
    expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
  })

  it('не показывает баннер по событию, уже бывшему в буфере на момент монтирования', () => {
    // Регрессия: буфер накопленный, а не «что пришло сейчас» (EVENTS_CAP, store/game.ts).
    // Если voteResolved уже был последним при первом рендере — это не «получение», молчим.
    render(<VoteOutcomeBanner events={[fresh(voteResolvedConfirmed)]} />)
    expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
  })

  it('появляется, когда в буфер добавляется новый voteResolved (ШУХ подтверждён)', () => {
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(<VoteOutcomeBanner events={[fresh(voteResolvedConfirmed)]} />)
    expect(screen.getByTestId('vote-outcome-banner')).toHaveTextContent(/подтверждён/i)
  })

  it('текст при overturned=true объясняет, что штраф Ш-8 достаётся предъявителю', () => {
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(<VoteOutcomeBanner events={[fresh(voteResolvedOverturned)]} />)
    expect(screen.getByTestId('vote-outcome-banner')).toHaveTextContent(/отклонён/i)
    expect(screen.getByTestId('vote-outcome-banner')).toHaveTextContent(/предъявител/i)
  })

  it('не всплывает повторно, пока список событий не меняется (не залипает по «наличию» в буфере)', () => {
    vi.useFakeTimers()
    const events = [fresh(voteResolvedConfirmed)]
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(<VoteOutcomeBanner events={events} />)
    expect(screen.getByTestId('vote-outcome-banner')).toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(6000)
    })
    rerender(<VoteOutcomeBanner events={events} />)
    expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
    vi.useRealTimers()
  })

  it('исчезает сам через несколько секунд (детерминированно, без реальных задержек)', () => {
    vi.useFakeTimers()
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(<VoteOutcomeBanner events={[fresh(voteResolvedConfirmed)]} />)
    expect(screen.getByTestId('vote-outcome-banner')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(3999)
    })
    expect(screen.getByTestId('vote-outcome-banner')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(2001)
    })
    expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
    vi.useRealTimers()
  })

  it('находит voteResolved, за которым в ТОМ ЖЕ апдейте пришло shukhAssessed', () => {
    // Реальный порядок движка: resolveAdjudication пишет voteResolved, и сразу за ним
    // assessShukh пишет shukhAssessed (engine/apply.go) — оба приходят одним update.
    // Реакция «только на последний элемент буфера» тут молчит навсегда: последним
    // всегда оказывается shukhAssessed, и баннер исхода не появлялся бы в реальной игре.
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(
      <VoteOutcomeBanner
        events={[fresh(voteResolvedOverturned), { type: 'shukhAssessed', offender: 1, code: 8 }]}
      />,
    )
    expect(screen.getByTestId('vote-outcome-banner')).toHaveTextContent(/отклонён/i)
  })

  it('игнорирует события других типов', () => {
    const { rerender } = render(<VoteOutcomeBanner events={[]} />)
    rerender(<VoteOutcomeBanner events={[{ type: 'gameStarted', turn: 0 }]} />)
    expect(screen.queryByTestId('vote-outcome-banner')).not.toBeInTheDocument()
  })
})
