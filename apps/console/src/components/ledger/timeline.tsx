import { cn } from '~/lib/utils'

// 21 Ledger patterns, Hold timeline: a hold resolves exactly once, captured (fully or partially),
// voided, or expired by the cluster clock (ADR-0010).

export type TimelineTone = 'warning' | 'accent' | 'muted' | 'danger' | 'success'

const TONE: Record<TimelineTone, string> = {
  accent: 'bg-primary',
  danger: 'bg-danger',
  muted: 'bg-border-strong',
  success: 'bg-success',
  warning: 'bg-warning'
}

export function Timeline({ className, ...props }: React.ComponentProps<'ol'>) {
  return <ol className={cn('flex flex-col', className)} {...props} />
}

interface TimelineItemProps {
  title: React.ReactNode
  meta?: React.ReactNode
  aside?: React.ReactNode
  tone: TimelineTone
  /** An unfinished step (waiting for capture) shows a hollow dot. */
  hollow?: boolean
  last?: boolean
}

export function TimelineItem({ title, meta, aside, tone, hollow = false, last = false }: TimelineItemProps) {
  return (
    <li className="flex gap-3">
      <div aria-hidden="true" className="flex w-3 shrink-0 flex-col items-center gap-1 pt-1">
        <span className={cn('size-2.5 shrink-0 rounded-full', hollow ? 'border border-border-strong bg-surface' : TONE[tone])} />
        {last ? null : <span className="w-px flex-1 bg-border-strong" />}
      </div>
      <div className={cn('flex min-w-0 flex-1 flex-col gap-0.5', last ? 'pb-0' : 'pb-4')}>
        <div className="flex items-center gap-2">
          <span className="flex-1 text-small font-medium text-foreground">{title}</span>
          {aside}
        </div>
        {meta ? <div className="font-mono text-[11px] text-muted-foreground">{meta}</div> : null}
      </div>
    </li>
  )
}
