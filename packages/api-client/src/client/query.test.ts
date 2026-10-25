import { describe, expect, it } from 'vite-plus/test'

import { queryParams } from './query'

describe('queryParams', () => {
  it('drops undefined, null and empty members and keeps the rest', () => {
    expect(queryParams({ cursor: undefined, limit: 100, prefix: '', search: null, status: 'open', zero: 0 })).toEqual({
      limit: 100,
      status: 'open',
      zero: 0
    })
  })
})
