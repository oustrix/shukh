import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError, joinRoom, me } from '../../net/rooms'
import { STORED_NAME_KEY } from '../../routes'
import { GameProvider, useGame } from '../../store/GameProvider'
import { selectConn, selectLastError, selectSeats, selectStage, selectView } from '../../store/game'
import { Button } from '../kit/Button'
import { Lobby } from './Lobby'
import { Table } from './Table'
import styles from './Screens.module.css'

type Probe = 'checking' | 'seated' | 'needsName' | 'roomNotFound'

const joinErrorText: Record<string, string> = {
  full: 'Комната заполнена',
  duplicate: 'Такое имя уже занято',
  roomNotFound: 'Комната не найдена',
  unknown: 'Не удалось занять место',
}

export function Room() {
  const { code = '' } = useParams()
  const [probe, setProbe] = useState<Probe>('checking')
  const [name, setName] = useState(() => localStorage.getItem(STORED_NAME_KEY) ?? '')
  const [joinError, setJoinError] = useState<string | null>(null)

  const check = useCallback(async () => {
    const res = await me(code)
    setProbe(res.kind === 'seat' ? 'seated' : res.kind === 'roomNotFound' ? 'roomNotFound' : 'needsName')
  }, [code])

  useEffect(() => {
    void check()
  }, [check])

  if (probe === 'checking') return <div className={styles.centered}>Проверяем место…</div>
  if (probe === 'roomNotFound') {
    return (
      <div className={styles.centered}>
        <h2>Комната не найдена</h2>
        <p>Код {code} никому не принадлежит — возможно, комната уже закрылась.</p>
      </div>
    )
  }
  if (probe === 'needsName') {
    return (
      <form
        className={styles.centered}
        onSubmit={(e) => {
          e.preventDefault()
          setJoinError(null)
          localStorage.setItem(STORED_NAME_KEY, name.trim())
          void joinRoom(code, name.trim())
            .then(() => setProbe('seated'))
            .catch((err) => setJoinError(joinErrorText[err instanceof ApiError ? err.code : 'unknown']))
        }}
      >
        <h2>Комната {code}</h2>
        <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
        <Button type="submit" disabled={name.trim() === ''}>
          Занять место
        </Button>
        {joinError && <p role="alert">{joinError}</p>}
      </form>
    )
  }
  return (
    <GameProvider code={code}>
      <RoomBody />
    </GameProvider>
  )
}

// Стадия приходит с сервера и правит экраном (W3-1): нажатый хостом «Начать» переводит
// в стол у всех, а переподключение возвращает туда, где партия сейчас.
function RoomBody() {
  const stage = useGame(selectStage)
  const conn = useGame(selectConn)
  const lastError = useGame(selectLastError)

  if (conn === 'lost') {
    return (
      <div className={styles.centered}>
        <h2>Место потеряно</h2>
        <p>{lastError?.code === 'roomNotFound' ? 'Комната закрылась.' : 'Место освободилось, пока вас не было.'}</p>
        <Button onClick={() => window.location.reload()}>Войти заново</Button>
      </div>
    )
  }
  return (
    <>
      {conn !== 'open' && (
        <div className={styles.connBanner} role="status">
          {conn === 'connecting' ? 'Подключение…' : 'Связь потеряна, переподключаемся…'}
        </div>
      )}
      {stage === null && <div className={styles.centered}>Загрузка комнаты…</div>}
      {stage === 'lobby' && <Lobby />}
      {(stage === 'playing' || stage === 'finished') && <Table />}
      {stage === 'finished' && <FinishBanner />}
    </>
  )
}

// Итог партии (R-10.1): порядок выхода публичен, новая партия в этой итерации не
// запускается — серия R-10.2 отложена дорожной картой.
function FinishBanner() {
  const view = useGame(selectView)
  const seats = useGame(selectSeats)
  const nameOf = (seat: number) => seats.find((s) => s.seat === seat)?.name ?? `Игрок ${seat}`
  return (
    <div className={styles.finishBanner} role="status">
      <h3>Партия окончена</h3>
      <ol>
        {(view?.finish ?? []).map((seat) => (
          <li key={seat}>{nameOf(seat)}</li>
        ))}
      </ol>
      <Button onClick={() => window.location.assign('/')}>Выйти</Button>
    </div>
  )
}
