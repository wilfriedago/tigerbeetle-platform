/**
 * Time display. The TigerBeetle timestamp (committed time) is the authoritative time of every
 * transfer; a client business time is always labelled separately (ADR-0004). Times show in the
 * zone chosen in Preferences, local by default.
 */

export type TimeZoneMode = 'local' | 'utc'

const formatters = new Map<string, Intl.DateTimeFormat>()

function formatter(key: string, options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  let f = formatters.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', options)
    formatters.set(key, f)
  }
  return f
}

function zoneOptions(mode: TimeZoneMode): Intl.DateTimeFormatOptions {
  return mode === 'utc' ? { timeZone: 'UTC' } : {}
}

function toDate(value: string | Date): Date | undefined {
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? undefined : date
}

function yearOf(date: Date, mode: TimeZoneMode): number {
  return mode === 'utc' ? date.getUTCFullYear() : date.getFullYear()
}

/** "Oct 24, 14:32:08", with the year when it is not the current one. */
export function formatTimestamp(value: string | Date, mode: TimeZoneMode = 'local', now = new Date()): string {
  const date = toDate(value)
  if (!date) {
    return String(value)
  }
  const withYear = yearOf(date, mode) !== yearOf(now, mode)
  return formatter(`ts-${mode}-${withYear}`, {
    ...zoneOptions(mode),
    day: 'numeric',
    hour: '2-digit',
    hourCycle: 'h23',
    minute: '2-digit',
    month: 'short',
    second: '2-digit',
    ...(withYear ? { year: 'numeric' } : {})
  }).format(date)
}

/** "14:32:08", for lists that are already scoped to recent activity. */
export function formatTime(value: string | Date, mode: TimeZoneMode = 'local'): string {
  const date = toDate(value)
  if (!date) {
    return String(value)
  }
  return formatter(`time-${mode}`, { ...zoneOptions(mode), hour: '2-digit', hourCycle: 'h23', minute: '2-digit', second: '2-digit' }).format(date)
}

/** "Oct 24, 2026, 14:32:08 UTC", the full form for description lists. */
export function formatDateTimeFull(value: string | Date, mode: TimeZoneMode = 'local'): string {
  const date = toDate(value)
  if (!date) {
    return String(value)
  }
  return formatter(`full-${mode}`, {
    ...zoneOptions(mode),
    day: 'numeric',
    hour: '2-digit',
    hourCycle: 'h23',
    minute: '2-digit',
    month: 'short',
    second: '2-digit',
    timeZoneName: 'short',
    year: 'numeric'
  }).format(date)
}

/** "Oct 23, 2026". */
export function formatDate(value: string | Date, mode: TimeZoneMode = 'local'): string {
  const date = toDate(value)
  if (!date) {
    return String(value)
  }
  return formatter(`date-${mode}`, { ...zoneOptions(mode), day: 'numeric', month: 'short', year: 'numeric' }).format(date)
}

const RELATIVE_UNITS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ['day', 86_400],
  ['hour', 3600],
  ['minute', 60],
  ['second', 1]
]

const SHORT_UNITS: Partial<Record<Intl.RelativeTimeFormatUnit, string>> = { day: 'd', hour: 'h', minute: 'min', second: 's' }

/** "2 min ago", "in 38 min", "in 2 h". */
export function formatRelative(value: string | Date, now = new Date()): string {
  const date = toDate(value)
  if (!date) {
    return String(value)
  }
  const seconds = Math.round((date.getTime() - now.getTime()) / 1000)
  const abs = Math.abs(seconds)
  if (abs < 5) {
    return 'just now'
  }
  for (const [unit, size] of RELATIVE_UNITS) {
    if (abs >= size || unit === 'second') {
      const count = Math.floor(abs / size)
      const text = `${count} ${SHORT_UNITS[unit] ?? unit}`
      return seconds < 0 ? `${text} ago` : `in ${text}`
    }
  }
  return ''
}
