// Единый источник форм URL комнаты: паттерны маршрутов (для <Route path>) и
// билдеры путей (для navigate). Меняется структура маршрутов — правится здесь.
export const ROOM_ROUTE = '/room/:code'
export const TABLE_ROUTE = '/room/:code/table'

export const roomPath = (code: string) => `/room/${code}`
export const tablePath = (code: string) => `/room/${code}/table`
