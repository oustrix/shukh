import { render, screen } from '@testing-library/react'
import { App } from './App'

test('App рендерит заголовок игры', () => {
  render(<App />)
  expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
})
