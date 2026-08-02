import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { decodeServerMsg, encodeAction, WireError } from './wire'
import type { Action } from './types'

// __dirname в ESM-модулях Vitest не определён — путь берём от import.meta.url.
// Литерал вынесен в переменную: инлайновый new URL('...', import.meta.url) Vite
// статически переписывает в asset-URL дев-сервера (jsdom-окружение — «клиентский»
// режим трансформации), из-за чего fileURLToPath падает с "must be of scheme file".
const wireRelPath = '../../../server/testdata/wire/'
const wireDir = fileURLToPath(new URL(wireRelPath, import.meta.url))
const fixture = (name: string) => JSON.parse(readFileSync(`${wireDir}${name}.json`, 'utf8'))

describe('decodeServerMsg на golden-фикстурах сервера (W3-3)', () => {
  it('лобби: view отсутствует, места и хост на месте', () => {
    const d = decodeServerMsg(fixture('lobby'))
    expect(d.kind).toBe('update')
    if (d.kind !== 'update') return
    expect(d.snapshot.stage).toBe('lobby')
    expect(d.snapshot.view).toBeNull()
    expect(d.snapshot.you).toBe(1)
    expect(d.snapshot.host).toBe(0)
    expect(d.snapshot.seats).toEqual([
      { seat: 0, name: 'Вера' },
      { seat: 1, name: 'Боря' },
    ])
  })

  it('партия: рука, кон и легальные ходы декодируются', () => {
    const d = decodeServerMsg(fixture('playing'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.stage).toBe('playing')
    expect(d.snapshot.view?.hand).toHaveLength(2)
    expect(d.snapshot.view?.table[0]).toEqual({ card: { rank: 7, suit: '♣' }, by: 0 })
    expect(d.snapshot.legal).toContainEqual({ type: 'takeBottomAndPass' })
    expect(d.events[0]).toEqual({ type: 'cardPlayed', seat: 0, card: { rank: 7, suit: '♣' } })
  })

  it('открытый разбор: VoteView и дедлайн доезжают', () => {
    const d = decodeServerMsg(fixture('vote_open'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.view?.vote).toEqual({ claimant: 0, target: 1, code: 6, voted: [0] })
    expect(d.snapshot.voteDeadline).toBe(1754130000000)
  })

  it('все 12 действий сервера декодируются без потерь', () => {
    const d = decodeServerMsg(fixture('all_actions'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.legal).toHaveLength(12)
    expect(new Set(d.snapshot.legal.map((a) => a.type)).size).toBe(12)
  })

  it('все 17 событий сервера декодируются без потерь', () => {
    const d = decodeServerMsg(fixture('all_events'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.events).toHaveLength(17)
    expect(new Set(d.events.map((e) => e.type)).size).toBe(17)
  })
})

describe('decodeServerMsg — конверты и защита', () => {
  it('ack и error', () => {
    expect(decodeServerMsg({ type: 'ack', reqId: 'r1' })).toEqual({ kind: 'ack', reqId: 'r1' })
    expect(
      decodeServerMsg({ type: 'error', reqId: 'r2', code: 'notYours', message: 'nope' }),
    ).toEqual({
      kind: 'error',
      reqId: 'r2',
      error: { code: 'notYours', message: 'nope' },
    })
  })

  it('неизвестный тип события — громкая ошибка, а не тихий пропуск (§5)', () => {
    const bad = { ...fixture('playing'), events: [{ type: 'somethingNew', seat: 0 }] }
    expect(() => decodeServerMsg(bad)).toThrow(WireError)
  })

  it('код ШУХа вне множества допустимых — тоже громкая ошибка, а не as-каст мимо', () => {
    const bad = fixture('vote_open')
    bad.view.vote.code = 7 // 7 не входит в ShukhCode — намеренно пропущенное число
    expect(() => decodeServerMsg(bad)).toThrow(WireError)
  })

  it('неизвестный конверт — тоже ошибка', () => {
    expect(() => decodeServerMsg({ type: 'gossip' })).toThrow(WireError)
  })
})

describe('encodeAction', () => {
  it('голос кодируется строкой, понятной серверу', () => {
    const a: Action = { type: 'vote', vote: 'againstShukh' }
    expect(encodeAction(a)).toEqual({ type: 'vote', vote: 'againstShukh' })
  })
})
