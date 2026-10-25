import { cn } from '~/lib/utils'

import { StatusBadge } from './status-badge'

// 21 Ledger patterns, System and rules: Health/Up and Health/Down for the top bar, Dependency Row
// for the System page, Rule Summary for account rules.

export function HealthDot({ up, label, className }: { up: boolean | undefined; label: string; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-caption text-muted-foreground', className)}>
      <span aria-hidden="true" className={cn('size-2 rounded-full', up === undefined ? 'bg-border-strong' : up ? 'bg-success' : 'bg-danger')} />
      {label}
    </span>
  )
}

interface DependencyRowProps {
  icon: React.ElementType
  name: string
  detail?: React.ReactNode
  latencyMs?: number
  up: boolean
  reason?: string
}

export function DependencyRow({ icon: Icon, name, detail, latencyMs, up, reason }: DependencyRowProps) {
  return (
    <div className="flex items-center gap-4 rounded-md border border-border bg-surface px-4 py-3.5">
      <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-surface-sunken">
        <Icon aria-hidden="true" className="size-4 text-muted-foreground" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body font-medium text-foreground">{name}</span>
        {detail ? <span className="truncate font-mono text-[11px] text-muted-foreground">{detail}</span> : null}
      </div>
      {latencyMs === undefined ? null : <span className="font-mono text-small text-muted-foreground">{latencyMs} ms</span>}
      <StatusBadge status={up ? 'up' : 'down'}>{up ? 'up' : `down${reason ? `, ${reason}` : ''}`}</StatusBadge>
    </div>
  )
}
