import { describe, expect, it } from 'vite-plus/test'

import { emptyPosting, postingReady, toRequest } from './draft'

describe('postingReady', () => {
  it('needs two different accounts and an amount', () => {
    expect(postingReady({ ...emptyPosting('EUR/2', 'world', 'users:1'), amount: '1' })).toBe(true)
    expect(postingReady({ ...emptyPosting('EUR/2', 'world', 'world'), amount: '1' })).toBe(false)
    expect(postingReady(emptyPosting('EUR/2', 'world', 'users:1'))).toBe(false)
  })
})

describe('toRequest', () => {
  it('sends integers in the smallest unit and leaves out empty details', () => {
    const body = toRequest({
      businessTime: '',
      eventType: 'default',
      metadata: [{ key: '', value: 'ignored' }],
      postings: [{ amount: '1,250.00', asset: 'EUR/2', destination: 'users:1234:wallet', source: 'world' }],
      reference: '  '
    })
    expect(body).toEqual({ postings: [{ amount: '125000', asset: 'EUR/2', destination: 'users:1234:wallet', source: 'world' }] })
  })

  it('keeps the details that are set', () => {
    const body = toRequest({
      businessTime: '2026-10-24',
      eventType: 'deposit',
      metadata: [{ key: 'order_id', value: 'ord_8814' }],
      postings: [{ amount: '250000', asset: 'XOF', destination: 'users:1', source: 'world' }],
      reference: 'inv-2026-0043'
    })
    expect(body).toEqual({
      businessTime: '2026-10-24T00:00:00Z',
      eventType: 'deposit',
      metadata: { order_id: 'ord_8814' },
      postings: [{ amount: '250000', asset: 'XOF/0', destination: 'users:1', source: 'world' }],
      reference: 'inv-2026-0043'
    })
  })
})
