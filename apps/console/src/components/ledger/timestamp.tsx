import { CalendarIcon } from 'lucide-react'

import { usePreferences } from '~/lib/preferences'
import { formatDate, formatDateTimeFull, formatRelative, formatTimestamp } from '~/lib/time'
import { cn } from '~/lib/utils'

// 20 Money and identity, Timestamp and Business Time. The TigerBeetle timestamp is the committed
// time, the only time the ledger knows; a client business time is labelled apart with a calendar
// icon (ADR-0004).

interface TimestampProps {
  value: string
  relative?: boolean
  className?: string
}

export function Timestamp({ value, relative = false, className }: TimestampProps) {
  const { timeZone } = usePreferences()
  return (
    <span className={cn('inline-flex items-baseline gap-1.5 whitespace-nowrap', className)}>
      <time dateTime={value} title={formatDateTimeFull(value, timeZone)}>
        {formatTimestamp(value, timeZone)}
      </time>
      {relative ? <span className="text-caption text-subtle-foreground">{formatRelative(value)}</span> : null}
    </span>
  )
}

export function BusinessTime({ value, className }: { value: string; className?: string }) {
  const { timeZone } = usePreferences()
  return (
    <span
      className={cn('inline-flex items-center gap-1.5 whitespace-nowrap text-muted-foreground', className)}
      title="Business time, set by the client"
    >
      <CalendarIcon aria-hidden="true" className="size-3.5" />
      <time dateTime={value}>{formatDate(value, timeZone)}</time>
    </span>
  )
}
