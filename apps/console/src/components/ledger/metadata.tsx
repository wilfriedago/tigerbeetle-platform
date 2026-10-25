import { cn } from '~/lib/utils'

// 12 Feedback, Tag: a key/value chip in mono, the key muted. Used for metadata everywhere.
export function Tag({ name, value, className }: { name: string; value?: string; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex h-5 max-w-full items-center gap-1 rounded-sm border border-border bg-surface-sunken px-1.5 font-mono text-caption leading-none whitespace-nowrap',
        className
      )}
    >
      <span className="text-muted-foreground">{name}</span>
      {value === undefined ? null : <span className="truncate text-foreground">{value}</span>}
    </span>
  )
}

interface MetadataTagsProps {
  metadata: Record<string, string> | undefined
  /** Lay the chips out in a column (cards) instead of a wrapping row (headers, tables). */
  stacked?: boolean
  /** Show at most this many chips and a "+N" count for the rest. */
  limit?: number
  empty?: React.ReactNode
  className?: string
}

export function MetadataTags({ metadata, stacked = false, limit, empty = null, className }: MetadataTagsProps) {
  const entries = Object.entries(metadata ?? {}).toSorted(([a], [b]) => a.localeCompare(b))
  if (entries.length === 0) {
    return <>{empty}</>
  }
  const shown = limit === undefined ? entries : entries.slice(0, limit)
  const hidden = entries.length - shown.length
  return (
    <span className={cn('flex gap-1.5', stacked ? 'flex-col items-start' : 'flex-wrap items-center', className)}>
      {shown.map(([key, value]) => (
        <Tag key={key} name={key} value={value} />
      ))}
      {hidden > 0 ? <span className="font-mono text-[11px] text-subtle-foreground">+{hidden}</span> : null}
    </span>
  )
}
