import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import styles from './Notice.module.css'

// Сколько уведомление держится на экране: успеть прочитать фразу, но не мозолить глаза.
export const NOTICE_MS = 4000

// Показывать уведомление умеет один узел на всё дерево комнаты — иначе два источника
// (ошибка сервера из RoomBody и отброшенная при обрыве отправка из Table) рисовали бы
// два наложенных друг на друга блока в одной точке экрана.
const NotifyContext = createContext<(text: string) => void>(() => {
  // Вне <NoticeArea> уведомлять некому. Молча глотаем, а не бросаем: уведомление —
  // вспомогательная обратная связь, ронять из-за него экран нельзя.
})

export function useNotify(): (text: string) => void {
  return useContext(NotifyContext)
}

export function NoticeArea({ children }: { children: ReactNode }) {
  // id, а не только текст: одна и та же фраза подряд (два отброшенных действия при
  // обрыве) обязана всплыть заново и заново завести таймер — по тексту это неотличимо
  // от «то же самое уведомление всё ещё висит».
  const [notice, setNotice] = useState<{ id: number; text: string } | null>(null)
  const seq = useRef(0)
  const notify = useCallback((text: string) => {
    seq.current += 1
    setNotice({ id: seq.current, text })
  }, [])

  useEffect(() => {
    if (!notice) return
    const id = setTimeout(() => setNotice(null), NOTICE_MS)
    return () => clearTimeout(id)
  }, [notice])

  return (
    <NotifyContext.Provider value={notify}>
      {children}
      {notice && (
        <div className={styles.notice}>
          <div className={styles.body} role="status" data-testid="notice">
            {notice.text}
          </div>
        </div>
      )}
    </NotifyContext.Provider>
  )
}
