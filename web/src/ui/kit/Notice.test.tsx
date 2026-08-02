import { act, render, screen } from '@testing-library/react'
import { NoticeArea, useNotify, NOTICE_MS } from './Notice'

function Trigger({ text }: { text: string }) {
  const notify = useNotify()
  return <button onClick={() => notify(text)}>сказать</button>
}

function setup(text = 'первое') {
  return render(
    <NoticeArea>
      <Trigger text={text} />
    </NoticeArea>,
  )
}

describe('уведомление (kit/Notice)', () => {
  it('ничего не показывает, пока не позвали', () => {
    setup()
    expect(screen.queryByTestId('notice')).not.toBeInTheDocument()
  })

  it('показывает текст и сам его убирает через NOTICE_MS', () => {
    vi.useFakeTimers()
    setup('нет связи')
    act(() => screen.getByRole('button', { name: 'сказать' }).click())
    expect(screen.getByTestId('notice')).toHaveTextContent('нет связи')
    act(() => vi.advanceTimersByTime(NOTICE_MS - 1))
    expect(screen.getByTestId('notice')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(2))
    expect(screen.queryByTestId('notice')).not.toBeInTheDocument()
    vi.useRealTimers()
  })

  it('повторный вызов с тем же текстом заводит таймер заново', () => {
    vi.useFakeTimers()
    setup('нет связи')
    const say = screen.getByRole('button', { name: 'сказать' })
    act(() => say.click())
    act(() => vi.advanceTimersByTime(NOTICE_MS - 100))
    act(() => say.click()) // тот же текст — но это НОВОЕ уведомление
    act(() => vi.advanceTimersByTime(200))
    expect(screen.getByTestId('notice')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(NOTICE_MS))
    expect(screen.queryByTestId('notice')).not.toBeInTheDocument()
    vi.useRealTimers()
  })
})
