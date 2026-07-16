import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Card } from './Card'

test('открытая карта показывает ранг и масть (Дама♥)', () => {
  render(<Card card={{ suit: '♥', rank: 12 }} />)
  const face = screen.getByTestId('card-face')
  expect(face).toHaveTextContent('Q')
  expect(face).toHaveTextContent('♥')
})

test('закрытая карта рендерит рубашку', () => {
  render(<Card card={{ suit: '♥', rank: 12 }} faceDown />)
  expect(screen.getByTestId('card-back')).toBeInTheDocument()
})

test('Enter активирует карту (A11y)', async () => {
  const onClick = vi.fn()
  render(<Card card={{ suit: '♦', rank: 9 }} onClick={onClick} />)
  screen.getByRole('button', { name: '9♦' }).focus()
  await userEvent.keyboard('{Enter}')
  expect(onClick).toHaveBeenCalled()
})

test('Space активирует карту (A11y)', async () => {
  const onClick = vi.fn()
  render(<Card card={{ suit: '♦', rank: 9 }} onClick={onClick} />)
  screen.getByRole('button', { name: '9♦' }).focus()
  await userEvent.keyboard('[Space]')
  expect(onClick).toHaveBeenCalled()
})
