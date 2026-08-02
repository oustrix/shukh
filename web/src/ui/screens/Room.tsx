import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError, joinRoom, me } from '../../net/rooms'
import { STORED_NAME_KEY } from '../../routes'
import { GameProvider, useGame } from '../../store/GameProvider'
import { selectConn, selectLastError, selectSeats, selectStage, selectView } from '../../store/game'
import { Button } from '../kit/Button'
import { NoticeArea, useNotify } from '../kit/Notice'
import { Lobby } from './Lobby'
import { Table } from './Table'
import styles from './Screens.module.css'

type Probe = 'checking' | 'seated' | 'needsName' | 'roomNotFound' | 'networkError'

const joinErrorText: Record<string, string> = {
  full: 'Комната заполнена',
  duplicate: 'Такое имя уже занято',
  roomNotFound: 'Комната не найдена',
  unknown: 'Не удалось занять место',
}

// Отказы сервера по сокету (§9). Сырые коды игроку не показываем — фразы по смыслу
// правил, а не буквальный перевод кода. Больнее всего это бьёт по субъективным ШУХам:
// движок сознательно не кладёт claimSubjective в legal, законность проверяется на
// сабмите, и ошибка сервера — ЕДИНСТВЕННЫЙ канал обратной связи. Без неё нажатие
// «ШУХ: завис» неотличимо от проглоченного клика.
const serverErrorText: Record<string, string> = {
  illegalAction: 'Так сейчас нельзя — стол не принял это по правилам',
  notYours: 'Сейчас не ваш ход',
  notPlaying: 'Партия ещё не идёт',
  notHost: 'Настройки и старт партии — у хоста',
  notLobby: 'Партия уже началась — в лобби не вернуться',
  tooFewPlayers: 'Для партии нужно хотя бы два игрока',
}

// Уведомляет об отказах сервера. Ничего не рисует сам: единственная поверхность
// уведомлений — NoticeArea, чтобы два источника не накладывались друг на друга.
function ServerErrorNotice() {
  const lastError = useGame(selectLastError)
  const notify = useNotify()
  // Реагируем на ПОЯВЛЕНИЕ ошибки, а не на её наличие: lastError живёт в сторе до
  // следующей смены статуса, и по «наличию» уведомление всплывало бы на каждый
  // ре-рендер. Кодек создаёт новый объект на каждый кадр error — сравнения по
  // ссылке достаточно, и повтор того же кода честно считается новой ошибкой.
  const seen = useRef(lastError)
  useEffect(() => {
    if (lastError === seen.current) return
    seen.current = lastError
    if (!lastError) return
    notify(serverErrorText[lastError.code] ?? 'Сервер отклонил действие')
  }, [lastError, notify])
  return null
}

export function Room() {
  const { code = '' } = useParams()
  const [probe, setProbe] = useState<Probe>('checking')
  const [name, setName] = useState(() => localStorage.getItem(STORED_NAME_KEY) ?? '')
  const [joinError, setJoinError] = useState<string | null>(null)
  // Запрос в полёте держим в ref, а не в state: два клика в один тик увидели бы
  // одно и то же (устаревшее) значение state, а disabled на кнопке применится лишь
  // после ре-рендера. Ref срабатывает сразу — это и есть настоящая защёлка.
  const inFlight = useRef(false)
  const [busy, setBusy] = useState(false)
  const startRequest = () => {
    if (inFlight.current) return false
    inFlight.current = true
    setBusy(true)
    return true
  }
  const endRequest = () => {
    inFlight.current = false
    setBusy(false)
  }

  const check = useCallback(async () => {
    if (inFlight.current) return
    inFlight.current = true
    setBusy(true)
    setProbe('checking')
    try {
      const res = await me(code)
      setProbe(res.kind === 'seat' ? 'seated' : res.kind === 'roomNotFound' ? 'roomNotFound' : 'needsName')
    } catch {
      // fetch сам упал (оффлайн/DNS/сервер не поднят) — это не «комната не найдена»,
      // сервер вообще не ответил. Даём выход вместо вечной «Проверяем место…».
      setProbe('networkError')
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }, [code])

  useEffect(() => {
    void check()
  }, [check])

  if (probe === 'checking') return <div className={styles.centered}>Проверяем место…</div>
  if (probe === 'networkError') {
    return (
      <div className={styles.centered}>
        <h2>Не удалось проверить место</h2>
        <p>Сервер не ответил — проверьте соединение и попробуйте ещё раз.</p>
        <Button onClick={() => void check()} disabled={busy}>
          Повторить
        </Button>
      </div>
    )
  }
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
          // Room.Join на сервере минтит НОВЫЙ PlayerID на каждый вызов: второй
          // параллельный запрос занял бы второе место и перезаписал куку, а первое
          // осталось бы за призраком — выселить его в MVP нечем (ни кика, ни тайм-аута
          // хода). В комнате на двоих это сразу неиграбельный стол.
          if (!startRequest()) return
          setJoinError(null)
          localStorage.setItem(STORED_NAME_KEY, name.trim())
          void joinRoom(code, name.trim())
            .then(() => setProbe('seated'))
            .catch((err) => setJoinError(joinErrorText[err instanceof ApiError ? err.code : 'unknown']))
            .finally(endRequest)
        }}
      >
        <h2>Комната {code}</h2>
        <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
        <Button type="submit" disabled={busy || name.trim() === ''}>
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
    <NoticeArea>
      <ServerErrorNotice />
      {conn !== 'open' && (
        <div className={styles.connBanner} role="status">
          {conn === 'connecting' ? 'Подключение…' : 'Связь потеряна, переподключаемся…'}
        </div>
      )}
      {stage === null && <div className={styles.centered}>Загрузка комнаты…</div>}
      {stage === 'lobby' && <Lobby />}
      {(stage === 'playing' || stage === 'finished') && <Table />}
      {stage === 'finished' && <FinishBanner />}
    </NoticeArea>
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
