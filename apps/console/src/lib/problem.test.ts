import { HttpStatusError } from '@tigerbeetle-platform/api-client'
import { describe, expect, it } from 'vite-plus/test'

import { toProblem } from './problem'

describe('toProblem', () => {
  it('describes a non-problem status and an unreachable API', () => {
    expect(toProblem(new HttpStatusError(502, 'Bad Gateway', undefined))).toMatchObject({ status: 502, title: 'Unexpected response' })
    expect(toProblem(new TypeError('Failed to fetch'))).toMatchObject({ title: 'API unreachable' })
    expect(toProblem(new DOMException('signal timed out', 'TimeoutError'))).toMatchObject({ title: 'API unreachable' })
  })
})
