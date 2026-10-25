import { describe, expect, it } from 'vite-plus/test'

import { randomRequestId, requestHeaders } from './headers'

describe('randomRequestId', () => {
  it('is 32 lowercase hex characters, new each time', () => {
    expect(randomRequestId()).toMatch(/^[0-9a-f]{32}$/)
    expect(randomRequestId()).not.toBe(randomRequestId())
  })
})

describe('requestHeaders', () => {
  it('keeps the headers of the call and adds the request ID and the accepted media types', () => {
    const headers = requestHeaders({ 'Content-Type': 'application/json', 'Idempotency-Key': 'idem_1' }, 'req-1')
    expect(Object.fromEntries(headers)).toEqual({
      accept: 'application/json, application/problem+json',
      'content-type': 'application/json',
      'idempotency-key': 'idem_1',
      'x-request-id': 'req-1'
    })
  })

  it('keeps a request ID the caller chose', () => {
    expect(requestHeaders({ 'X-Request-Id': 'mine' }, 'req-1').get('X-Request-Id')).toBe('mine')
  })
})
