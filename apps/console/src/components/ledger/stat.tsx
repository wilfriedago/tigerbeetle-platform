import { cn } from '~/lib/utils'

// 15 Data display, Stat: overview figures in a row of cells divided by hairlines. The value never
// animates. 12 px medium label, 24 px semibold value with an optional mono unit, 12 px sub line.

export function StatGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      className={cn(
        'grid overflow-hidden rounded-lg border border-border bg-surface sm:grid-flow-col sm:auto-cols-fr [&>*]:border-b [&>*]:border-border sm:[&>*]:border-r sm:[&>*]:border-b-0 [&>*:last-child]:border-0',
        className
      )}
      {...props}
    />
  )
}

interface StatProps {
  label: React.ReactNode
  value: React.ReactNode
  unit?: React.ReactNode
  sub?: React.ReactNode
  className?: string
}

export function Stat({ label, value, unit, sub, className }: StatProps) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-1.5 p-5', className)}>
      <div className="text-caption font-medium text-muted-foreground">{label}</div>
      <div className="flex items-baseline gap-1.5">
        <span className="truncate text-page-title tabular-nums">{value}</span>
        {unit ? <span className="font-mono text-small text-muted-foreground">{unit}</span> : null}
      </div>
      {sub ? <div className="truncate text-caption text-subtle-foreground">{sub}</div> : null}
    </div>
  )
}
