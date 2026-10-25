import { type Hold } from '@tigerbeetle-platform/api-client'
import { ClockIcon, InfinityIcon } from 'lucide-react'

import { formatRelative } from '~/lib/time'
import { useNow } from '~/lib/use-now'
import { cn } from '~/lib/utils'

/** Under this many minutes the countdown turns to the warning tone. */
const SOON_MS = 15 * 60 * 1000

/** "in 12 min", "no timeout" or the resolution time: expiry follows the cluster clock (ADR-0010). */
export function HoldExpiry({ hold }: { hold: Hold }) {
  const now = useNow(15_000)
  if (!hold.expiresAt) {
    return (
      <span className="inline-flex items-center gap-1.5 text-small text-muted-foreground">
        <InfinityIcon aria-hidden="true" className="size-3.5" />
        no timeout
      </span>
    )
  }
  const left = Date.parse(hold.expiresAt) - now.getTime()
  const soon = hold.status === 'open' && left < SOON_MS
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-small', soon ? 'text-warning-fg' : 'text-muted-foreground')}>
      <ClockIcon aria-hidden="true" className="size-3.5" />
      {hold.status === 'open' ? formatRelative(hold.expiresAt, now) : `expired ${formatRelative(hold.expiresAt, now)}`}
    </span>
  )
}
