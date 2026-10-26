import { describe, expect, it } from 'vite-plus/test'

import { can, canCreateLedgers, type Session } from './session'

function session(role: string | undefined, ledgers: Record<string, string> = {}): Session {
  const base: Session = { ledgers, name: 'Ada', organizations: [{ alias: 'acme', name: 'Acme' }], staff: false, subject: 'ada' }
  return role ? { ...base, role } : base
}

describe('can', () => {
  it('lets organization admins and owners do everything', () => {
    expect(can(session('owner'), 'ledger.configure', 'payments')).toBe(true)
    expect(can(session('admin'), 'money.write', 'anything')).toBe(true)
    expect(canCreateLedgers(session('admin'))).toBe(true)
  })

  it('follows the ledger role of a member (ADR-0028)', () => {
    const viewer = session('member', { payments: 'viewer' })
    expect(can(viewer, 'ledger.read', 'payments')).toBe(true)
    expect(can(viewer, 'money.write', 'payments')).toBe(false)
    const operator = session('member', { payments: 'operator' })
    expect(can(operator, 'money.write', 'payments')).toBe(true)
    expect(can(operator, 'ledger.configure', 'payments')).toBe(false)
    expect(can(session('member', { payments: 'admin' }), 'ledger.configure', 'payments')).toBe(true)
    expect(canCreateLedgers(viewer)).toBe(false)
  })

  it('allows nothing on a ledger without a role, or without a session', () => {
    expect(can(session('member'), 'ledger.read', 'payroll')).toBe(false)
    expect(can(undefined, 'ledger.read', 'payments')).toBe(false)
  })
})
