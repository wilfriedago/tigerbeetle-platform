import { describe, expect, it } from 'vite-plus/test'

import { cn, tv } from './utils'

// Each case pairs a design token with a default Tailwind class from a different group (both kept)
// or from the same group (the later one wins). cn and tv run separate merge engines, so both are
// checked against the same expectations.
const cases: [string, string, string][] = [
  ['text-small', 'text-muted-foreground', 'text-small text-muted-foreground'],
  ['text-caption', 'text-body', 'text-body'],
  ['shadow-float', 'shadow-black', 'shadow-float shadow-black'],
  ['shadow-float', 'shadow-overlay', 'shadow-overlay'],
  ['h-row', 'h-8', 'h-8'],
  ['px-3', 'px-header-row', 'px-header-row']
]

describe('cn', () => {
  it.each(cases)('merges %s with %s', (first, second, want) => {
    expect(cn(first, second)).toBe(want)
  })
})

describe('tv', () => {
  it.each(cases)('merges %s with %s', (first, second, want) => {
    expect(tv({ base: first })({ className: second })).toBe(want)
  })

  it('merges a variant over the base with the token config', () => {
    const recipe = tv({
      base: 'text-caption text-foreground',
      variants: { tone: { muted: 'text-muted-foreground' } }
    })
    expect(recipe({ tone: 'muted' })).toBe('text-caption text-muted-foreground')
  })
})
