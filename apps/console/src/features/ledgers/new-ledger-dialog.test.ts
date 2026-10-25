import { describe, expect, it } from 'vite-plus/test'

import { LEDGER_NAME_PATTERN, suggestLedgerName } from './new-ledger-dialog'

describe('suggestLedgerName', () => {
  it('turns a display name into a valid ledger name', () => {
    expect(suggestLedgerName('EU Refunds')).toBe('eu-refunds')
    expect(suggestLedgerName('  Payments / Card  ')).toBe('payments-card')
    expect(LEDGER_NAME_PATTERN.test(suggestLedgerName('EU Refunds'))).toBe(true)
  })
})
