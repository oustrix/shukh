import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

const noop = () => {}

test('«Взять низ» отключена не в свой ход', () => {
  render(<ActionBar yourTurn={false} onShukh={noop} onOneCard={noop} onTakeBottom={noop} />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeDisabled()
})

test('«ШУХ!» кликается и зовёт onShukh', async () => {
  const onShukh = vi.fn()
  render(<ActionBar yourTurn onShukh={onShukh} onOneCard={noop} onTakeBottom={noop} />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(onShukh).toHaveBeenCalled()
})
