import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ShukhZone } from './ShukhZone'

test('показывает счётчик отложенных карт', () => {
  render(<ShukhZone count={2} />)
  expect(screen.getByTestId('shukh-count')).toHaveTextContent('ШУХ 2')
})

test('пустая не-takeable зона не рендерится', () => {
  const { container } = render(<ShukhZone count={0} />)
  expect(container.firstChild).toBeNull()
})

test('takeable зона кликабельна и зовёт onTake', async () => {
  const onTake = vi.fn()
  render(<ShukhZone count={2} takeable onTake={onTake} label="Ваша ШУХ-зона" />)
  await userEvent.click(screen.getByRole('button', { name: 'Ваша ШУХ-зона' }))
  expect(onTake).toHaveBeenCalled()
})

test('не-takeable зона не имеет роли button', () => {
  render(<ShukhZone count={2} />)
  expect(screen.queryByRole('button')).toBeNull()
})
