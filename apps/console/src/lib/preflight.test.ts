import { describe, expect, it } from 'vite-plus/test'

import { type AccountState, simulate } from './preflight'

const world: AccountState = { balance: '-500000', exists: true, incoming: '0', limit: 'none', reserved: '0' }
const wallet: AccountState = { balance: '100000', exists: true, incoming: '0', limit: 'debits_must_not_exceed_credits', reserved: '40000' }
const fresh: AccountState = { balance: '0', exists: false, incoming: '0', limit: 'debits_must_not_exceed_credits', reserved: '0' }

function states(map: Record<string, AccountState>) {
  return (address: string) => map[address]
}

describe('simulate', () => {
  it('lets a later posting spend what an earlier one credited', () => {
    const result = simulate(
      [
        { amount: '125000', asset: 'EUR/2', destination: 'users:1:wallet', source: 'world' },
        { amount: '180000', asset: 'EUR/2', destination: 'merchants:42', source: 'users:1:wallet' }
      ],
      states({ 'merchants:42': fresh, 'users:1:wallet': wallet, world })
    )
    expect(result.failures).toEqual([])
    // world has no limit; the wallet can spend 100000 - 40000 reserved + 125000 credited.
    expect(result.spendable).toEqual([undefined, '185000'])
    expect(result.effects).toEqual([
      { address: 'world', after: '-625000', asset: 'EUR/2', before: '-500000', delta: '-125000', exists: true },
      { address: 'users:1:wallet', after: '45000', asset: 'EUR/2', before: '100000', delta: '-55000', exists: true },
      { address: 'merchants:42', after: '180000', asset: 'EUR/2', before: '0', delta: '180000', exists: false }
    ])
  })

  it('reports the posting that takes a limited account below zero, counting reserved funds', () => {
    const result = simulate(
      [{ amount: '60001', asset: 'EUR/2', destination: 'merchants:42', source: 'users:1:wallet' }],
      states({ 'merchants:42': fresh, 'users:1:wallet': wallet })
    )
    expect(result.failures).toEqual([{ address: 'users:1:wallet', asset: 'EUR/2', index: 0, reason: 'exceeds-credits', shortBy: '1' }])
  })

  it('checks credits_must_not_exceed_debits on the destination', () => {
    const capped: AccountState = { balance: '-1000', exists: true, incoming: '200', limit: 'credits_must_not_exceed_debits', reserved: '0' }
    const result = simulate([{ amount: '900', asset: 'EUR/2', destination: 'loans:1', source: 'world' }], states({ 'loans:1': capped, world }))
    expect(result.failures).toEqual([{ address: 'loans:1', asset: 'EUR/2', index: 0, reason: 'exceeds-debits', shortBy: '100' }])
  })

  it('skips unknown accounts and unfinished postings', () => {
    const result = simulate(
      [
        { amount: '', asset: 'EUR/2', destination: 'a', source: 'world' },
        { amount: '10', asset: 'EUR/2', destination: 'b', source: 'unknown' }
      ],
      states({ world })
    )
    expect(result.failures).toEqual([])
    expect(result.spendable).toEqual([undefined, undefined])
    expect(result.effects).toEqual([])
  })
})
