import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { useStore } from 'zustand'
import { createGameStore, type GameState, type GameStore } from './game'
import { createWsTransport } from '../transport/ws'

// Стор живёт ровно столько, сколько открыта комната: код известен только на маршруте,
// а транспорт держит сокет, который обязан закрыться при уходе (синглтона больше нет).
// Экспортирован ради тестов экранов (Table.test.tsx и т.п.): подставляют двойник
// стора через <GameContext.Provider>, не поднимая настоящий транспорт.
export const GameContext = createContext<GameStore | null>(null)

export function GameProvider({ code, children }: { code: string; children: ReactNode }) {
  // Транспорт createWsTransport(code) одноразовый: его close()/teardown необратимо
  // глушит инстанс (stopped больше не сбрасывается — transport/ws.ts), а createGameStore
  // синхронно подписывается на транспорт внутри себя. Из-за этого создавать их в
  // useMemo/рендере нельзя: React StrictMode в деве прогоняет монтирование эффектов как
  // setup → cleanup → setup, и если бы транспорт создавался в рендере (переживая этот
  // цикл как один и тот же объект), самый первый cleanup убил бы его навсегда — второй
  // setup не пересоздаёт то, что уже создано в рендере, так что живое соединение
  // осталось бы мёртвым. Поэтому создание — целиком внутри эффекта: каждый setup
  // получает свежий транспорт, и после двойного цикла выживает именно последний.
  const [store, setStore] = useState<GameStore | null>(null)
  useEffect(() => {
    const transport = createWsTransport(code)
    const s = createGameStore(transport)
    // Стор zustand — сам вызываемая функция (хук): setStore(s) без обёртки React
    // принял бы её за функциональный апдейтер (prev => next) и вызвал бы её вместо
    // сохранения — оборачиваем в лямбду, чтобы сохранить именно значение.
    setStore(() => s)
    return () => transport.close()
  }, [code])
  if (!store) return null
  return <GameContext.Provider value={store}>{children}</GameContext.Provider>
}

export function useGame<T>(selector: (s: GameState) => T): T {
  const store = useContext(GameContext)
  if (!store) throw new Error('useGame вне GameProvider')
  return useStore(store, selector)
}
