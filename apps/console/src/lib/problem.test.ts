import { ApiError, HttpStatusError, problemTypeOf } from '@tigerbeetle-platform/api-client'
import { describe, expect, it } from 'vite-plus/test'

import { toProblem } from './problem'

describe('toProblem', () => {
  it('keeps the members of a problem, its extensions and the Retry-After of the answer', () => {
    const error = new ApiError(
      { detail: 'still running', results: ['ok'], status: 409, title: 'Request in flight', type: problemTypeOf('request-in-flight') },
      'req-1',
      2
    )
    expect(toProblem(error)).toEqual({
      detail: 'still running',
      extensions: { results: ['ok'] },
      requestId: 'req-1',
      retryAfter: 2,
      status: 409,
      title: 'Request in flight',
      type: problemTypeOf('request-in-flight')
    })
  })

  it('has no Retry-After when the answer had none', () => {
    expect(toProblem(new ApiError({ status: 404, title: 'Not found', type: problemTypeOf('not-found') }))).not.toHaveProperty('retryAfter')
  })

  it('describes a non-problem status and an unreachable API', () => {
    expect(toProblem(new HttpStatusError(502, 'Bad Gateway', undefined))).toMatchObject({ status: 502, title: 'Unexpected response' })
    expect(toProblem(new TypeError('Failed to fetch'))).toMatchObject({ title: 'API unreachable' })
    expect(toProblem(new DOMException('signal timed out', 'TimeoutError'))).toMatchObject({ title: 'API unreachable' })
  })
})
