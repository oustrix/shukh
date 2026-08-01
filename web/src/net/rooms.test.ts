import { createRoom, joinRoom, me, ApiError } from './rooms'

function mockFetch(status: number, body: unknown) {
  const spy = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response)
  vi.stubGlobal('fetch', spy)
  return spy
}

afterEach(() => vi.unstubAllGlobals())

describe('net/rooms', () => {
  it('createRoom шлёт имя и конфиг и ВСЕГДА с credentials (кука комнаты)', async () => {
    const spy = mockFetch(200, { code: 'ABCD' })
    const res = await createRoom('Вера', { deckSize: 36, mode: 'middle' })
    expect(res).toEqual({ code: 'ABCD' })
    const [url, init] = spy.mock.calls[0]
    expect(String(url)).toMatch(/\/api\/rooms$/)
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(JSON.parse(init.body)).toEqual({ name: 'Вера', config: { deckSize: 36, mode: 'middle' } })
  })

  it('joinRoom возвращает место', async () => {
    mockFetch(200, { seat: 2 })
    await expect(joinRoom('ABCD', 'Боря')).resolves.toEqual({ seat: 2 })
  })

  it('joinRoom → 409 превращается в типизированную ошибку', async () => {
    mockFetch(409, { error: 'full' })
    await expect(joinRoom('ABCD', 'Боря')).rejects.toMatchObject({ code: 'full' })
    await expect(joinRoom('ABCD', 'Боря')).rejects.toBeInstanceOf(ApiError)
  })

  it('me различает три исхода пробы', async () => {
    mockFetch(200, { seat: 1 })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'seat', seat: 1 })
    mockFetch(401, { error: 'seatNotFound' })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'seatNotFound' })
    mockFetch(404, { error: 'roomNotFound' })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'roomNotFound' })
  })
})
