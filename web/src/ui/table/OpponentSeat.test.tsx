import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { OpponentSeat } from './OpponentSeat'

test('показывает имя и число карт', () => {
  render(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }}
      menuOpen={false}
      onToggleMenu={vi.fn()}
    />,
  )
  expect(screen.getByText('Боря')).toBeInTheDocument()
  expect(screen.getByText(/5/)).toBeInTheDocument()
})

test('ШУХ-зона показывается только при shukhPending > 0', () => {
  const { rerender } = render(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }}
      menuOpen={false}
      onToggleMenu={vi.fn()}
    />,
  )
  expect(screen.queryByTestId('shukh-zone')).not.toBeInTheDocument()
  rerender(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 2, live: true }}
      menuOpen={false}
      onToggleMenu={vi.fn()}
    />,
  )
  expect(screen.getByTestId('shukh-count')).toHaveTextContent('ШУХ 2')
})

test('заголовок места — кнопка, клик по ней переключает меню (aria-expanded)', async () => {
  const onToggleMenu = vi.fn()
  render(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }}
      menuOpen={false}
      onToggleMenu={onToggleMenu}
    />,
  )
  const btn = screen.getByRole('button', { name: 'Боря' })
  expect(btn).toHaveAttribute('aria-expanded', 'false')
  await userEvent.click(btn)
  expect(onToggleMenu).toHaveBeenCalled()
})

test('рендерит children (SeatMenu) только когда menuOpen', () => {
  const { rerender } = render(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }}
      menuOpen={false}
      onToggleMenu={vi.fn()}
    >
      <div data-testid="seat-menu-stub">меню</div>
    </OpponentSeat>,
  )
  expect(screen.queryByTestId('seat-menu-stub')).not.toBeInTheDocument()
  rerender(
    <OpponentSeat
      name="Боря"
      opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }}
      menuOpen={true}
      onToggleMenu={vi.fn()}
    >
      <div data-testid="seat-menu-stub">меню</div>
    </OpponentSeat>,
  )
  expect(screen.getByTestId('seat-menu-stub')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Боря' })).toHaveAttribute('aria-expanded', 'true')
})
