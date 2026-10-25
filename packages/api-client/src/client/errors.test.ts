import { describe, expect, it } from 'vite-plus/test'

import { ApiError, parseRetryAfter, problemFrom } from './errors'

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
      type: '/problems/exceeds-credits'
    })
    expect(problemFrom(text)).toEqual({
      posting: 1,
      results: ['linked_event_failed', 'exceeds_credits'],
      status: 422,
      title: 'Declined',
      type: '/problems/exceeds-credits'
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
    const error = new ApiError({ detail: 'retry later', status: 503, title: 'Write pending', type: '/problems/write-pending' }, 'req-1', 2)
    expect(error).toBeInstanceOf(Error)
    expect(error.message).toBe('Write pending: retry later')
    expect([error.status, error.type, error.requestId, error.retryAfter]).toEqual([503, '/problems/write-pending', 'req-1', 2])
  })
})
