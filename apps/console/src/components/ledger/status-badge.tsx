import { Badge, type BadgeProps } from '~/components/ui/badge'

// 12 Feedback, Status vocabulary: one mapping for the whole console, so the same word always gets
// the same badge. Colour means state, never direction (Ledger Instrument principle 02).
const STATUS_VARIANTS = {
  // Transactions
  committed: 'neutral',
  reverted: 'muted',
  revert: 'accent',
  // Holds
  open: 'warning',
  captured: 'accent',
  voided: 'muted',
  expired: 'muted',
  // Write requests
  pending: 'warning',
  failed: 'danger',
  declined: 'danger',
  // Services
  up: 'success',
  down: 'danger',
  degraded: 'warning',
  // Previews
  'not sent': 'neutral'
} as const satisfies Record<string, NonNullable<BadgeProps['variant']>>

export type Status = keyof typeof STATUS_VARIANTS

export function StatusBadge({ status, className, children }: { status: Status; className?: string; children?: React.ReactNode }) {
  return (
    <Badge variant={STATUS_VARIANTS[status]} className={className}>
      {children ?? status}
    </Badge>
  )
}
