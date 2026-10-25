import { describe, expect, it } from 'vite-plus/test'

import { rangeStart } from './time-range'

describe('rangeStart', () => {
  it('is undefined without a range', () => {
    expect(rangeStart(undefined)).toBeUndefined()
  })

  it('subtracts the range and rounds down to the minute', () => {
    const now = Date.parse('2026-10-25T14:32:47.123Z')
    expect(rangeStart('1h', now)).toBe('2026-10-25T13:32:00.000Z')
    expect(rangeStart('24h', now)).toBe('2026-10-24T14:32:00.000Z')
  })
})
