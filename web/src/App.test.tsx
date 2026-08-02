import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { App } from './App'
import { me } from './net/rooms'

vi.mock('./net/rooms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./net/rooms')>()
  return { ...actual, me: vi.fn(), createRoom: vi.fn(), joinRoom: vi.fn() }
})

afterEach(() => vi.resetAllMocks())

describe('маршруты', () => {
  it('корень — экран входа', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Создать комнату/i })).toBeInTheDocument()
  })

  it('/r/CODE — экран комнаты, а не 404 роутера', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'roomNotFound' })
    render(
      <MemoryRouter initialEntries={['/r/ABCD']}>
        <App />
      </MemoryRouter>,
    )
    expect(await screen.findByText(/Комната не найдена/i)).toBeInTheDocument()
  })
})
