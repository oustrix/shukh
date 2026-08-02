import type { ReactNode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { Room } from './Room'
import { ROOM_ROUTE } from '../../routes'
import { ApiError, joinRoom, me, type MeResult } from '../../net/rooms'
import type { GameState } from '../../store/game'
import { buildSeatView } from '../../fixtures/seatView'

// ESM-экспорты не патчатся через vi.spyOn — модуль подменяется целиком (vi.mock),
// ApiError берём настоящий, чтобы проверять реальную ветку обработки ошибки.
vi.mock('../../net/rooms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../net/rooms')>()
  return { ...actual, me: vi.fn(), joinRoom: vi.fn() }
})

// RoomBody (ветвление по stage/conn) читает стор только через useGame — подменяем
// весь модуль GameProvider: GameProvider становится сквозным (без транспорта),
// useGame читает мутируемый gameState, который каждый тест выставляет через setGameState.
let gameState: GameState = {
  snapshot: null,
  events: [],
  conn: 'open',
  lastError: null,
  play: () => {},
  command: () => {},
}

vi.mock('../../store/GameProvider', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../store/GameProvider')>()
  return {
    ...actual,
    GameProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
    useGame: (selector: (s: GameState) => unknown) => selector(gameState),
  }
})

function setGameState(over: Partial<GameState>) {
  gameState = { snapshot: null, events: [], conn: 'open', lastError: null, play: () => {}, command: () => {}, ...over }
}

function renderAt(code: string) {
  return render(
    <MemoryRouter initialEntries={[`/r/${code}`]}>
      <Routes>
        <Route path={ROOM_ROUTE} element={<Room />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  setGameState({})
})

afterEach(() => vi.resetAllMocks())

describe('экран комнаты — проба места', () => {
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

  it('двойной клик «Занять место» не шлёт join дважды (второе место сгорело бы навсегда)', async () => {
    // Room.Join на сервере минтит НОВЫЙ PlayerID на каждый вызов: второй клик занял бы
    // второе место и перезаписал куку, а первое осталось бы за призраком, которого нечем
    // выселить (ни кика, ни тайм-аута хода в MVP). В комнате на двоих это мёртвый стол.
    vi.mocked(me).mockResolvedValue({ kind: 'seatNotFound' })
    let settle: (v: { seat: number }) => void = () => {}
    vi.mocked(joinRoom).mockReturnValue(new Promise((resolve) => (settle = resolve)))
    renderAt('ABCD')
    await userEvent.type(await screen.findByLabelText('Имя'), 'Боря')
    const button = screen.getByRole('button', { name: /Занять место/i })
    fireEvent.click(button) // два клика подряд, пока первый запрос ещё в полёте
    fireEvent.click(button)
    expect(joinRoom).toHaveBeenCalledTimes(1)
    expect(button).toBeDisabled()
    await act(async () => settle({ seat: 1 }))
  })

  it('двойной клик «Повторить» не шлёт пробу дважды', async () => {
    vi.mocked(me).mockRejectedValueOnce(new Error('network down'))
    let settle: (v: MeResult) => void = () => {}
    vi.mocked(me).mockReturnValueOnce(new Promise((resolve) => (settle = resolve)))
    renderAt('ABCD')
    const retry = await screen.findByRole('button', { name: /Повторить/i })
    fireEvent.click(retry)
    fireEvent.click(retry)
    expect(me).toHaveBeenCalledTimes(2) // первая — автоматическая при монтировании
    await act(async () => settle({ kind: 'seatNotFound' }))
  })

  it('сбой сети при пробе → не бесконечная загрузка, а сообщение и «Повторить»', async () => {
    // me() бросает исключение (fetch упал — оффлайн/DNS/сервер не поднят), а не
    // возвращает MeResult: это другой класс сбоя, чем «комната не найдена», и врать
    // пользователю про закрытую комнату нельзя.
    vi.mocked(me).mockRejectedValueOnce(new Error('network down'))
    vi.mocked(me).mockResolvedValueOnce({ kind: 'seatNotFound' })
    renderAt('ABCD')
    expect(await screen.findByText(/не удалось проверить место/i)).toBeInTheDocument()
    expect(screen.queryByText(/Комната не найдена/i)).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Повторить/i }))
    expect(me).toHaveBeenCalledTimes(2)
    expect(await screen.findByLabelText('Имя')).toBeInTheDocument()
  })
})

describe('экран комнаты — RoomBody: ветвление по stage/conn после успешной пробы', () => {
  beforeEach(() => {
    vi.mocked(me).mockResolvedValue({ kind: 'seat', seat: 0 })
  })

  it('stage lobby → рендерит Lobby (состав комнаты)', async () => {
    setGameState({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Аня' }],
        view: null,
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByTestId('players')).toHaveTextContent('Аня')
  })

  it('stage playing → рендерит стол', async () => {
    setGameState({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'playing',
        host: 0,
        seats: [{ seat: 0, name: 'Аня' }],
        view: buildSeatView(),
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByTestId('action-bar')).toBeInTheDocument()
  })

  it('stage finished → стол остаётся и поверх него баннер итога партии', async () => {
    setGameState({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'finished',
        host: 0,
        seats: [
          { seat: 0, name: 'Аня' },
          { seat: 1, name: 'Боря' },
        ],
        view: buildSeatView({ finish: [1, 0] }),
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByTestId('action-bar')).toBeInTheDocument()
    const banner = await screen.findByText(/Партия окончена/i)
    expect(banner).toBeInTheDocument()
    const order = screen.getAllByRole('listitem').map((li) => li.textContent)
    expect(order).toEqual(['Боря', 'Аня'])
  })

  it('conn lost → терминальный экран «Место потеряно», а не стол/лобби', async () => {
    setGameState({
      conn: 'lost',
      lastError: { code: 'seatLost', message: 'место потеряно' },
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Аня' }],
        view: null,
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByText(/Место потеряно/i)).toBeInTheDocument()
    expect(screen.queryByTestId('players')).not.toBeInTheDocument()
  })

  it('conn connecting → баннер подключения поверх лобби', async () => {
    setGameState({
      conn: 'connecting',
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Аня' }],
        view: null,
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByRole('status')).toHaveTextContent(/Подключение/i)
  })

  it('conn reconnecting → баннер переподключения поверх лобби', async () => {
    setGameState({
      conn: 'reconnecting',
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Аня' }],
        view: null,
        legal: [],
      },
    })
    renderAt('ABCD')
    expect(await screen.findByRole('status')).toHaveTextContent(/переподключаемся/i)
  })
})
