import { ApiError, HttpStatusError, problemTypeOf } from '@tigerbeetle-platform/api-client'
import { describe, expect, it } from 'vite-plus/test'

import { keyIsSpent, newIdempotencyKey } from './idempotency'

function problem(status: number, reason: string) {
  return new ApiError({ status, title: reason, type: problemTypeOf(reason) }, 'req')
}

describe('newIdempotencyKey', () => {
  it('is idem_ and 32 hex characters split like a UUID', () => {
    expect(newIdempotencyKey()).toMatch(/^idem_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/)
    expect(newIdempotencyKey()).not.toBe(newIdempotencyKey())
  })
})

describe('keyIsSpent', () => {
  it('is true for a final answer stored under the key', () => {
    expect(keyIsSpent(problem(422, 'exceeds-credits'))).toBe(true)
    expect(keyIsSpent(problem(409, 'hold-already-resolved'))).toBe(true)
  })

  it('is false when retrying with the same key is the safe move', () => {
    expect(keyIsSpent(problem(409, 'request-in-flight'))).toBe(false)
    expect(keyIsSpent(problem(503, 'write-pending'))).toBe(false)
    expect(keyIsSpent(problem(500, 'internal'))).toBe(false)
    expect(keyIsSpent(new HttpStatusError(502, 'Bad Gateway', undefined))).toBe(false)
    expect(keyIsSpent(new TypeError('Failed to fetch'))).toBe(false)
  })
})
