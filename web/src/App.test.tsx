import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { App } from './App'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

test('корень показывает экран входа', () => {
  renderAt('/')
  expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
  expect(screen.getByLabelText('Код комнаты')).toBeInTheDocument()
})

test('лобби показывает игроков комнаты из стора', () => {
  renderAt('/room/DEMO')
  const players = screen.getByTestId('players')
  expect(players).toHaveTextContent('Аня')
  expect(players).toHaveTextContent('Боря')
})

test('стол рендерит руку и мест соперников из снапшота', () => {
  renderAt('/room/DEMO/table')
  // 5 карт руки из фикстуры
  expect(screen.getAllByTestId('card-face').length).toBeGreaterThanOrEqual(5)
  expect(screen.getByTestId('action-bar')).toBeInTheDocument()
  expect(screen.getByTestId('seat-1')).toBeInTheDocument()
  expect(screen.getByTestId('seat-2')).toBeInTheDocument()
})
