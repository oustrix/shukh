import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SeatMenu } from './SeatMenu'
import type { Action } from '../../contract/types'

describe('меню соперника', () => {
  it('скрывает адресные вопросы, которых нет в legal', () => {
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={vi.fn()} onClose={vi.fn()} />)
    expect(screen.queryByRole('button', { name: /Сколько карт/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Есть Запад/i })).not.toBeInTheDocument()
  })

  it('показывает вопрос о картах, когда он легален (R-6)', async () => {
    const onAction = vi.fn()
    const legal: Action[] = [{ type: 'askCount', target: 1 }]
    render(<SeatMenu seat={1} name="Боря" you={0} legal={legal} onAction={onAction} onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /Сколько карт/i }))
    expect(onAction).toHaveBeenCalledWith({ type: 'askCount', target: 1 })
  })

  it('субъективные ШУХи доступны всегда — движок их в legal не кладёт', async () => {
    const onAction = vi.fn()
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={onAction} onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /завис/i }))
    expect(onAction).toHaveBeenCalledWith({ type: 'claimSubjective', claimant: 0, target: 1, code: 6 })
  })

  it('Esc закрывает меню', async () => {
    const onClose = vi.fn()
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={vi.fn()} onClose={onClose} />)
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })
})
