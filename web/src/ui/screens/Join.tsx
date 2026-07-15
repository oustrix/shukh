import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { roomPath } from '../../routes'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Join() {
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const navigate = useNavigate()
  const canJoin = name.trim() !== '' && code.trim() !== ''
  return (
    <form
      className={styles.centered}
      onSubmit={(e) => {
        e.preventDefault()
        navigate(roomPath(code.trim()))
      }}
    >
      <h1>Шух</h1>
      <input
        aria-label="Имя"
        placeholder="Имя"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <input
        aria-label="Код комнаты"
        placeholder="Код комнаты"
        value={code}
        onChange={(e) => setCode(e.target.value)}
      />
      <Button type="submit" disabled={!canJoin}>
        Войти
      </Button>
    </form>
  )
}
