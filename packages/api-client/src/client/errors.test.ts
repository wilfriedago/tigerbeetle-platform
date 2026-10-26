import { describe, expect, it } from 'vite-plus/test'

import { ApiError, parseRetryAfter, PROBLEM_TYPE_BASE, problemFrom, problemReason, problemTypeOf } from './errors'

describe('parseRetryAfter', () => {
  const now = Date.parse('2026-10-25T12:00:00Z')

  it.each([
    ['5', 5],
    [' 0 ', 0],
    ['Sun, 25 Oct 2026 12:00:30 GMT', 30],
    ['Sun, 25 Oct 2026 11:59:00 GMT', 0],
    ['Sunday, 25-Oct-26 12:00:10 GMT', 10],
    ['Sun Oct 25 12:00:20 2026', 20],
    [null, undefined],
    ['', undefined],
    ['soon', undefined],
    ['-1', undefined]
  ])('reads %j as %j seconds', (value, seconds) => {
    expect(parseRetryAfter(value, now)).toBe(seconds)
  })
})

describe('problemFrom', () => {
  it('reads a problem and keeps its extension members', () => {
    const text = JSON.stringify({
      posting: 1,
      results: ['linked_event_failed', 'exceeds_credits'],
      status: 422,
      title: 'Declined',
      type: problemTypeOf('exceeds-credits')
    })
    expect(problemFrom(text)).toEqual({
      posting: 1,
      results: ['linked_event_failed', 'exceeds_credits'],
      status: 422,
      title: 'Declined',
      type: problemTypeOf('exceeds-credits')
    })
  })

  it('reads a missing type as about:blank (RFC 9457)', () => {
    expect(problemFrom('{"status":404,"title":"Not found"}')).toEqual({ status: 404, title: 'Not found', type: 'about:blank' })
  })

  it.each(['', 'not json', '[]', 'null', '{"title":"no status"}', '{"status":500}'])('is undefined for %j', (text) => {
    expect(problemFrom(text)).toBeUndefined()
  })
})

describe('ApiError', () => {
  it('carries the problem, its status and type, and the request ID', () => {
    const error = new ApiError({ detail: 'retry later', status: 503, title: 'Write pending', type: problemTypeOf('write-pending') }, 'req-1', 2)
    expect(error).toBeInstanceOf(Error)
    expect(error.message).toBe('Write pending: retry later')
    expect([error.status, error.type, error.requestId, error.retryAfter]).toEqual([503, problemTypeOf('write-pending'), 'req-1', 2])
  })
})

describe('problem types', () => {
  it('turns a reason into its type and back', () => {
    expect(problemTypeOf('exceeds-credits')).toBe('https://github.com/wilfriedago/tigerbeetle-platform/blob/develop/docs/problems.md#exceeds-credits')
    expect(problemReason(problemTypeOf('exceeds-credits'))).toBe('exceeds-credits')
  })

  it.each(['about:blank', '/problems/exceeds-credits', PROBLEM_TYPE_BASE, undefined])('has no reason for %j', (type) => {
    expect(problemReason(type)).toBeUndefined()
  })

  it('gives an ApiError the reason of its type', () => {
    expect(new ApiError({ status: 409, title: 'Request in flight', type: problemTypeOf('request-in-flight') }).reason).toBe('request-in-flight')
    expect(new ApiError({ status: 404, title: 'Not Found', type: 'about:blank' }).reason).toBeUndefined()
  })
})
