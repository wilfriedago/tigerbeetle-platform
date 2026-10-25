import { describe, expect, it } from 'vite-plus/test'

import { addressError, amountError } from './validation'

describe('validation', () => {
  it('checks addresses against the API pattern', () => {
    expect(addressError('', 'source')).toBe('Pick the source account.')
    expect(addressError('users:1234:wallet', 'source')).toBeUndefined()
    expect(addressError('users::1', 'source')).toMatch(/segments/)
    expect(addressError('users:1 2', 'source')).toMatch(/segments/)
  })

  it('checks amounts with the asset scale and refuses zero', () => {
    expect(amountError('12.50', 'EUR/2')).toBeUndefined()
    expect(amountError('12.505', 'EUR/2')).toMatch(/2 decimal places/)
    expect(amountError('0.00', 'EUR/2')).toMatch(/greater than zero/)
    expect(amountError('', 'EUR/2')).toBe('Enter an amount.')
  })
})
