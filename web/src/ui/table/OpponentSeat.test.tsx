import { render, screen } from '@testing-library/react'
import { OpponentSeat } from './OpponentSeat'

test('показывает имя и число карт', () => {
  render(<OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }} />)
  expect(screen.getByText('Боря')).toBeInTheDocument()
  expect(screen.getByText(/5/)).toBeInTheDocument()
})

test('бейдж ШУХ показывается только при shukhPending > 0', () => {
  const { rerender } = render(
    <OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 0, live: true }} />,
  )
  expect(screen.queryByTestId('shukh-badge')).not.toBeInTheDocument()
  rerender(
    <OpponentSeat name="Боря" opponent={{ seat: 1, handCount: 5, shukhPending: 2, live: true }} />,
  )
  expect(screen.getByTestId('shukh-badge')).toHaveTextContent('2')
})
