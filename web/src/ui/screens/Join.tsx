import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createRoom } from '../../net/rooms'
import { roomPath, STORED_NAME_KEY } from '../../routes'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Join() {
  const [name, setName] = useState(() => localStorage.getItem(STORED_NAME_KEY) ?? '')
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const trimmedName = name.trim()

  const remember = () => localStorage.setItem(STORED_NAME_KEY, trimmedName)

  return (
    <div className={styles.centered}>
      <h1>Шух</h1>
      <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
      <Button
        disabled={trimmedName === ''}
        onClick={() => {
          setError(null)
          remember()
          void createRoom(trimmedName)
            .then((r) => navigate(roomPath(r.code)))
            .catch(() => setError('Не удалось создать комнату'))
        }}
      >
        Создать комнату
      </Button>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          remember()
          navigate(roomPath(code.trim().toUpperCase()))
        }}
      >
        <input
          aria-label="Код комнаты"
          placeholder="Код комнаты"
          value={code}
          onChange={(e) => setCode(e.target.value)}
        />
        <Button type="submit" disabled={code.trim() === ''}>
          Войти по коду
        </Button>
      </form>
      {error && <p role="alert">{error}</p>}
    </div>
  )
}
