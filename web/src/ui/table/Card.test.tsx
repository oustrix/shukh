import { render, screen } from '@testing-library/react'
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
