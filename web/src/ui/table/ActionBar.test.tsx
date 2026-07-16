import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

const props = {
  canConfirm: false,
  onConfirm: () => {},
  canTakeBottom: false,
  onTakeBottom: () => {},
  canShukh: false,
  onShukh: () => {},
  owesOneCard: false,
  onOneCard: () => {},
}

test('«Сходить» отключена без выбранной легальной карты', () => {
  render(<ActionBar {...props} canConfirm={false} />)
  expect(screen.getByRole('button', { name: 'Сходить' })).toBeDisabled()
})

test('«Сходить» активна и зовёт onConfirm', async () => {
  const onConfirm = vi.fn()
  render(<ActionBar {...props} canConfirm onConfirm={onConfirm} />)
  await userEvent.click(screen.getByRole('button', { name: 'Сходить' }))
  expect(onConfirm).toHaveBeenCalled()
})

test('«ШУХ!» отключена без открытого ШУХ-окна', () => {
  render(<ActionBar {...props} canShukh={false} />)
  expect(screen.getByRole('button', { name: 'ШУХ!' })).toBeDisabled()
})

test('«ШУХ!» активна при canShukh и зовёт onShukh', async () => {
  const onShukh = vi.fn()
  render(<ActionBar {...props} canShukh onShukh={onShukh} />)
  await userEvent.click(screen.getByRole('button', { name: 'ШУХ!' }))
  expect(onShukh).toHaveBeenCalled()
})

test('«Взять низ» активна только при canTakeBottom', () => {
  const { rerender } = render(<ActionBar {...props} canTakeBottom={false} />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeDisabled()
  rerender(<ActionBar {...props} canTakeBottom />)
  expect(screen.getByRole('button', { name: 'Взять низ' })).toBeEnabled()
})

test('«Одна карта!» активна только когда owesOneCard', () => {
  render(<ActionBar {...props} owesOneCard />)
  expect(screen.getByRole('button', { name: 'Одна карта!' })).toBeEnabled()
})
