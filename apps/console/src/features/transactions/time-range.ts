/**
 * Committed-time presets for the transaction filters. The URL keeps the preset, not a timestamp,
 * so a shared "last 24 hours" link means the last 24 hours when it is opened. The start is
 * rounded down to the minute: the query key then stays stable while the minute lasts.
 */
export const TIME_RANGES = {
  '1h': { label: 'Last hour', ms: 60 * 60 * 1000 },
  '24h': { label: 'Last 24 hours', ms: 24 * 60 * 60 * 1000 },
  '7d': { label: 'Last 7 days', ms: 7 * 24 * 60 * 60 * 1000 },
  '30d': { label: 'Last 30 days', ms: 30 * 24 * 60 * 60 * 1000 }
} as const

export type TimeRange = keyof typeof TIME_RANGES

export const TIME_RANGE_KEYS = ['1h', '24h', '7d', '30d'] as const satisfies readonly TimeRange[]

export function rangeStart(range: TimeRange | undefined, now = Date.now()): string | undefined {
  if (!range) {
    return undefined
  }
  const start = now - TIME_RANGES[range].ms
  return new Date(start - (start % 60_000)).toISOString()
}
