import { render, screen } from '@testing-library/react'
import { ShukhVoteModal } from './ShukhVoteModal'
import type { ShukhVote } from '../../contract/types'

const nameOf = (s: number) => `Игрок ${s}`

const voting: ShukhVote = {
  claimant: 0,
  target: 1,
  code: 11,
  votes: [{ seat: 2, up: true }],
  outcome: 'upheld',
  resolved: false,
}

test('показывает цель, код и голоса во время голосования', () => {
  render(<ShukhVoteModal vote={voting} nameOf={nameOf} />)
  expect(screen.getByTestId('shukh-vote')).toHaveTextContent('Ш-11')
  expect(screen.getByTestId('shukh-vote')).toHaveTextContent('Игрок 1')
  expect(screen.getByTestId('vote-2')).toHaveTextContent('за')
  expect(screen.queryByTestId('vote-outcome')).toBeNull() // ещё не resolved
})

test('resolved=upheld показывает исход «подтверждён»', () => {
  render(<ShukhVoteModal vote={{ ...voting, resolved: true }} nameOf={nameOf} />)
  expect(screen.getByTestId('vote-outcome')).toHaveTextContent('подтверждён')
})

test('resolved=overturned показывает Ш-8 предъявившему', () => {
  render(
    <ShukhVoteModal
      vote={{ ...voting, resolved: true, outcome: 'overturned' }}
      nameOf={nameOf}
    />,
  )
  expect(screen.getByTestId('vote-outcome')).toHaveTextContent('Ш-8')
})
