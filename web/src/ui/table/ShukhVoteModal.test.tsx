import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ShukhVoteModal } from './ShukhVoteModal'
import type { Action, VoteView } from '../../contract/types'

const vote: VoteView = { claimant: 0, target: 1, code: 6, voted: [0] }
const nameOf = (s: number) => ['Вера', 'Боря', 'Гена'][s] ?? `Игрок ${s}`

describe('модалка голосования R-8.6', () => {
  it('показывает предмет разбора и КТО проголосовал, но не КАК (§8.4)', () => {
    render(<ShukhVoteModal vote={vote} deadline={null} legal={[]} nameOf={nameOf} onVote={vi.fn()} />)
    expect(screen.getByText(/Боря/)).toBeInTheDocument()
    expect(screen.getByText(/Ш-6/)).toBeInTheDocument()
    expect(screen.getByTestId('voted-0')).toHaveTextContent('Вера')
    expect(screen.queryByText(/за|против/i)).not.toBeInTheDocument()
  })

  it('кнопки голоса появляются только когда голос легален', async () => {
    const onVote = vi.fn()
    const { rerender } = render(
      <ShukhVoteModal vote={vote} deadline={null} legal={[]} nameOf={nameOf} onVote={onVote} />,
    )
    expect(screen.queryByRole('button', { name: /За ШУХ/i })).not.toBeInTheDocument()

    const legal: Action[] = [
      { type: 'vote', vote: 'forShukh' },
      { type: 'vote', vote: 'againstShukh' },
    ]
    rerender(<ShukhVoteModal vote={vote} deadline={null} legal={legal} nameOf={nameOf} onVote={onVote} />)
    await userEvent.click(screen.getByRole('button', { name: /Против ШУХа/i }))
    expect(onVote).toHaveBeenCalledWith('againstShukh')
  })

  it('исход берётся из события voteResolved', () => {
    render(
      <ShukhVoteModal
        vote={vote}
        deadline={null}
        legal={[]}
        nameOf={nameOf}
        onVote={vi.fn()}
        outcome={{ code: 8, overturned: true }}
      />,
    )
    expect(screen.getByTestId('vote-outcome')).toHaveTextContent(/отклонён/i)
  })
})
