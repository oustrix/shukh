import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

describe('ActionBar', () => {
  it('рендерит переданные действия и гасит недоступные', async () => {
    const onPlay = vi.fn()
    render(
      <ActionBar
        actions={[
          { label: 'Сходить', enabled: true, onClick: onPlay },
          { label: 'Сбросить Запад', enabled: false, onClick: vi.fn() },
        ]}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'Сходить' }))
    expect(onPlay).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Сбросить Запад' })).toBeDisabled()
  })
})
