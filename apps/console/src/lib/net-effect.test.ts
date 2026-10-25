import { describe, expect, it } from 'vite-plus/test'

import { netEffect } from './net-effect'

describe('netEffect', () => {
  it('sums debits and credits per account and asset, in chain order', () => {
    const rows = netEffect([
      { amount: '125000', asset: 'EUR/2', destination: 'users:1234:wallet', source: 'world' },
      { amount: '120000', asset: 'EUR/2', destination: 'merchants:42:main', source: 'users:1234:wallet' },
      { amount: '5000', asset: 'EUR/2', destination: 'fees:card', source: 'users:1234:wallet' }
    ])
    expect(rows).toEqual([
      { address: 'world', asset: 'EUR/2', credited: '0', debited: '125000', net: '-125000' },
      { address: 'users:1234:wallet', asset: 'EUR/2', credited: '125000', debited: '125000', net: '0' },
      { address: 'merchants:42:main', asset: 'EUR/2', credited: '120000', debited: '0', net: '120000' },
      { address: 'fees:card', asset: 'EUR/2', credited: '5000', debited: '0', net: '5000' }
    ])
  })

  it('keeps assets apart and stays exact above 2^53', () => {
    const big = '340282366920938463463374607431768211455'
    const rows = netEffect([
      { amount: big, asset: 'XOF/0', destination: 'a', source: 'world' },
      { amount: '1', asset: 'EUR/2', destination: 'a', source: 'world' }
    ])
    expect(rows.map((row) => [row.address, row.asset, row.net])).toEqual([
      ['world', 'XOF/0', `-${big}`],
      ['a', 'XOF/0', big],
      ['world', 'EUR/2', '-1'],
      ['a', 'EUR/2', '1']
    ])
  })
})
