import { describe, expect, it } from 'vite-plus/test'

import { compareAmounts, formatAmount, formatAsset, formatSignedAmount, parseAsset, parseMajorUnits, percentOf, sumAmounts } from './money'

describe('parseAsset', () => {
  it.each([
    ['EUR/2', { code: 'EUR', scale: 2 }],
    ['XOF/0', { code: 'XOF', scale: 0 }],
    ['EUR', { code: 'EUR', scale: 0 }],
    ['ETH/18', { code: 'ETH', scale: 18 }]
  ])('parses %s', (input, want) => {
    expect(parseAsset(input)).toEqual(want)
  })

  it.each(['eur/2', 'E/2', 'ETH/19', 'EUR/-1', ''])('rejects %s', (input) => {
    expect(parseAsset(input)).toBeUndefined()
  })

  it('renders the canonical form with the scale', () => {
    expect(formatAsset({ code: 'EUR', scale: 0 })).toBe('EUR/0')
  })
})

describe('formatAmount', () => {
  it.each([
    ['125000', 2, '1,250.00'],
    ['1250', 2, '12.50'],
    ['5', 2, '0.05'],
    ['0', 2, '0.00'],
    ['250000', 0, '250,000'],
    ['-481230000', 2, '-4,812,300.00'],
    ['100000000', 2, '1,000,000.00'],
    ['340282366920938463463374607431768211455', 0, '340,282,366,920,938,463,463,374,607,431,768,211,455'],
    ['1', 18, '0.000000000000000001']
  ])('formats %s with scale %i as %s', (raw, scale, want) => {
    expect(formatAmount(raw, scale)).toBe(want)
  })

  it('can skip grouping', () => {
    expect(formatAmount('125000', 2, { grouping: false })).toBe('1250.00')
  })

  it('leaves invalid input untouched', () => {
    expect(formatAmount('12.5', 2)).toBe('12.5')
  })
})

describe('parseMajorUnits', () => {
  it.each([
    ['1,250.00', 2, '125000'],
    ['1250', 2, '125000'],
    ['12.5', 2, '1250'],
    ['.05', 2, '5'],
    ['0', 2, '0'],
    ['250,000', 0, '250000'],
    ['340282366920938463463374607431768211455', 0, '340282366920938463463374607431768211455']
  ])('reads %s with scale %i as %s', (input, scale, raw) => {
    expect(parseMajorUnits(input, scale)).toEqual({ ok: true, raw })
  })

  it.each([
    ['', 2],
    ['-5', 2],
    ['1.234', 2],
    ['1.5', 0],
    ['1,25.00', 2],
    ['abc', 2],
    ['340282366920938463463374607431768211456', 0]
  ])('rejects %s with scale %i', (input, scale) => {
    expect(parseMajorUnits(input, scale).ok).toBe(false)
  })
})

describe('arithmetic', () => {
  it('sums with bigint beyond 2^53', () => {
    expect(sumAmounts(['9007199254740993', '1'])).toBe('9007199254740994')
  })

  it('compares', () => {
    expect(compareAmounts('10', '9')).toBe(1)
    expect(compareAmounts('-10', '9')).toBe(-1)
    expect(compareAmounts('7', '7')).toBe(0)
  })

  it('computes bar percentages without numbers overflowing', () => {
    expect(percentOf('240000', '340000')).toBeCloseTo(70.58, 1)
    expect(percentOf('0', '0')).toBe(0)
    expect(percentOf('5', '1')).toBe(100)
  })

  it('signs positive effects', () => {
    expect(formatSignedAmount('120000', 2)).toBe('+1,200.00')
    expect(formatSignedAmount('-125000', 2)).toBe('-1,250.00')
    expect(formatSignedAmount('0', 2)).toBe('0.00')
  })
})
