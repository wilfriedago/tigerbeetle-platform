import { describe, expect, it } from 'vite-plus/test'

import { ALIAS_PATTERN, suggestAlias } from './organizations-page'

describe('suggestAlias', () => {
  it('turns a name into a valid alias', () => {
    expect(suggestAlias('  Umbrella Finance, Inc. ')).toBe('umbrella-finance-inc')
    expect(ALIAS_PATTERN.test(suggestAlias('Umbrella Finance, Inc.'))).toBe(true)
  })
})
