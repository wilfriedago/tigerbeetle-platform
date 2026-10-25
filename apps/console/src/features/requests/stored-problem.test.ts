import { type WriteRequestDetail } from '@tigerbeetle-platform/api-client'
import { describe, expect, it } from 'vite-plus/test'

import { storedProblem } from './stored-problem'

const base: WriteRequestDetail = {
  attempts: 1,
  bodyHash: 'ab',
  createdAt: '2026-10-25T13:26:11Z',
  key: 'idem_1',
  operation: 'transaction',
  request: {},
  status: 'committed',
  targetId: '01a1733d7c6cd6cbdfb52320fabc4032',
  transferIds: []
}

describe('storedProblem', () => {
  it('is undefined for committed and pending records', () => {
    expect(storedProblem(base)).toBeUndefined()
    expect(storedProblem({ ...base, status: 'pending' })).toBeUndefined()
  })

  it('rebuilds the problem type and title from the stored domain problem', () => {
    const error = storedProblem({
      ...base,
      requestId: 'req_1',
      response: {
        detail: 'Posting 2 would take users:1 below zero.',
        posting: 1,
        reason: 'exceeds-credits',
        results: ['linked_event_failed', 'exceeds_credits']
      },
      responseStatus: 422,
      status: 'declined'
    })
    expect(error?.status).toBe(422)
    expect(error?.type).toBe('/problems/exceeds-credits')
    expect(error?.problem.title).toBe('Transaction declined')
    expect(error?.problem.detail).toBe('Posting 2 would take users:1 below zero.')
    expect(error?.problem.posting).toBe(1)
    expect(error?.requestId).toBe('req_1')
  })
})
