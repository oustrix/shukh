import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Lobby } from './Lobby'
import { useGame } from '../../store/GameProvider'
import { NoticeArea } from '../kit/Notice'
import type { GameState } from '../../store/game'

// Лобби читает стор только через useGame — подменяем его, чтобы не поднимать сокет.
vi.mock('../../store/GameProvider', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../store/GameProvider')>()
  return { ...actual, useGame: vi.fn() }
})

function mockGame(state: Partial<GameState>, command = vi.fn()) {
  const full: GameState = {
    snapshot: {
      roomCode: 'ABCD',
      you: 1,
      stage: 'lobby',
      host: 0,
      seats: [
        { seat: 0, name: 'Вера' },
        { seat: 1, name: 'Боря' },
      ],
      view: null,
      legal: [],
    },
    events: [],
    conn: 'open',
    lastError: null,
    play: vi.fn(),
    command,
    ...state,
  }
  vi.mocked(useGame).mockImplementation((selector) => selector(full))
  return command
}

afterEach(() => vi.resetAllMocks())

describe('лобби', () => {
  it('показывает состав и код комнаты', () => {
    mockGame({})
    render(<Lobby />)
    expect(screen.getByText('ABCD')).toBeInTheDocument()
    expect(screen.getByText('Вера')).toBeInTheDocument()
    expect(screen.getByText('Боря')).toBeInTheDocument()
  })

  it('не-хост не получает кнопку «Начать» (host=0, you=1)', () => {
    mockGame({})
    render(<Lobby />)
    expect(screen.queryByRole('button', { name: /Начать/i })).not.toBeInTheDocument()
  })

  it('хост начинает партию командой start', async () => {
    const command = mockGame({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [
          { seat: 0, name: 'Вера' },
          { seat: 1, name: 'Боря' },
        ],
        view: null,
        legal: [],
      },
    })
    render(<Lobby />)
    await userEvent.click(screen.getByRole('button', { name: /Начать/i }))
    expect(command).toHaveBeenCalledWith({ type: 'start' })
  })

  it('хост меняет колоду — уходит setConfig', async () => {
    const command = mockGame({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Вера' }],
        view: null,
        legal: [],
      },
    })
    render(<Lobby />)
    await userEvent.selectOptions(screen.getByLabelText('Колода'), '52')
    expect(command).toHaveBeenCalledWith({ type: 'setConfig', config: { deckSize: 52, mode: 'middle' } })
  })

  // Раньше блокировка с уведомлением жила локально в столе, и лобби её не унаследовало:
  // хост жал «Начать» в обрыве и не получал вообще ничего. Теперь механизм общий (§8/W3-5).
  it('в обрыве связи «Начать» заблокирована, а клик по настройке уведомляет', async () => {
    const command = mockGame({
      conn: 'reconnecting',
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [
          { seat: 0, name: 'Вера' },
          { seat: 1, name: 'Боря' },
        ],
        view: null,
        legal: [],
      },
    })
    render(
      <NoticeArea>
        <Lobby />
      </NoticeArea>,
    )
    expect(screen.getByRole('button', { name: /Начать/i })).toBeDisabled()

    await userEvent.selectOptions(screen.getByLabelText('Колода'), '52')
    expect(command).not.toHaveBeenCalled()
    expect(await screen.findByTestId('notice')).toHaveTextContent(/Связь потеряна/i)
  })
})
