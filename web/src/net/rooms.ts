// HTTP-часть входа в комнату. Токен места живёт в HttpOnly-куке (L2-6), поэтому
// каждый запрос идёт с credentials:'include' — иначе кука не уедет на другой origin.

export interface RoomConfig {
  deckSize: 36 | 52
  mode: 'guard' | 'middle' | 'culture'
}

export type MeResult = { kind: 'seat'; seat: number } | { kind: 'seatNotFound' } | { kind: 'roomNotFound' }

export type ApiErrorCode = 'full' | 'duplicate' | 'roomNotFound' | 'unknown'

export class ApiError extends Error {
  readonly code: ApiErrorCode
  constructor(code: ApiErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

// Сервер живёт на отдельном origin (W3-4): в деве адрес приходит из .env.development,
// в проде — из переменной сборки; иначе считаем, что API на том же хосте.
export function apiOrigin(): string {
  const fromEnv = import.meta.env.VITE_API_ORIGIN
  return typeof fromEnv === 'string' && fromEnv !== '' ? fromEnv : window.location.origin
}

async function postJSON(path: string, body: unknown): Promise<Response> {
  return fetch(`${apiOrigin()}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

function errorCode(payload: unknown): ApiErrorCode {
  const raw = typeof payload === 'object' && payload !== null ? (payload as { error?: unknown }).error : undefined
  return raw === 'full' || raw === 'duplicate' || raw === 'roomNotFound' ? raw : 'unknown'
}

export async function createRoom(name: string, config?: RoomConfig): Promise<{ code: string }> {
  const resp = await postJSON('/api/rooms', config ? { name, config } : { name })
  const payload = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new ApiError(errorCode(payload), 'не удалось создать комнату')
  return { code: String((payload as { code?: unknown }).code ?? '') }
}

export async function joinRoom(code: string, name: string): Promise<{ seat: number }> {
  const resp = await postJSON(`/api/rooms/${encodeURIComponent(code)}/join`, { name })
  const payload = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new ApiError(errorCode(payload), 'не удалось занять место')
  return { seat: Number((payload as { seat?: unknown }).seat ?? 0) }
}

// Проба места: браузерный WebSocket не показывает статус неудавшегося рукопожатия,
// поэтому «нет места» и «сервер лежит» различаем этим запросом (§7.7).
//
// Исходов ровно три: 200 {seat}, 401 seatNotFound, 404 roomNotFound. Любой другой
// статус (500/502/503, страница прокси) — НЕ «место потеряно», а транзиентный сбой,
// и он обязан реджектиться: seatNotFound терминален (conn=lost, §8), так что
// пятисотка посреди grace-периода иначе выкинула бы игрока с живого места.
export async function me(code: string): Promise<MeResult> {
  const resp = await fetch(`${apiOrigin()}/api/rooms/${encodeURIComponent(code)}/me`, {
    credentials: 'include',
  })
  if (resp.status === 404) return { kind: 'roomNotFound' }
  if (resp.status === 401) return { kind: 'seatNotFound' }
  if (!resp.ok) throw new ApiError('unknown', 'проба места не удалась')
  const payload = await resp.json().catch(() => ({}))
  return { kind: 'seat', seat: Number((payload as { seat?: unknown }).seat ?? 0) }
}
