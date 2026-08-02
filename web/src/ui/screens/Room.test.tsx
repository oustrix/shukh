import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { Room } from './Room'
import { ROOM_ROUTE } from '../../routes'
import { ApiError, joinRoom, me } from '../../net/rooms'

// ESM-экспорты не патчатся через vi.spyOn — модуль подменяется целиком (vi.mock),
// ApiError берём настоящий, чтобы проверять реальную ветку обработки ошибки.
vi.mock('../../net/rooms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../net/rooms')>()
  return { ...actual, me: vi.fn(), joinRoom: vi.fn() }
})

function renderAt(code: string) {
  return render(
    <MemoryRouter initialEntries={[`/r/${code}`]}>
      <Routes>
        <Route path={ROOM_ROUTE} element={<Room />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => vi.resetAllMocks())

describe('экран комнаты', () => {
  it('несуществующая комната → понятный экран, а не бесконечная загрузка', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'roomNotFound' })
    renderAt('ZZZZ')
    expect(await screen.findByText(/Комната не найдена/i)).toBeInTheDocument()
  })

  it('переход по инвайт-ссылке без куки → форма имени прямо здесь (§4)', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'seatNotFound' })
    vi.mocked(joinRoom).mockResolvedValue({ seat: 1 })
    renderAt('ABCD')
    const input = await screen.findByLabelText('Имя')
    await userEvent.type(input, 'Боря')
    await userEvent.click(screen.getByRole('button', { name: /Занять место/i }))
    expect(joinRoom).toHaveBeenCalledWith('ABCD', 'Боря')
  })

  it('409 full показывает причину под формой', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'seatNotFound' })
    vi.mocked(joinRoom).mockRejectedValue(new ApiError('full', 'нет мест'))
    renderAt('ABCD')
    await userEvent.type(await screen.findByLabelText('Имя'), 'Боря')
    await userEvent.click(screen.getByRole('button', { name: /Занять место/i }))
    expect(await screen.findByText(/Комната заполнена/i)).toBeInTheDocument()
  })
})
