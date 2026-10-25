import { shortId } from '~/lib/ids'
import { cn } from '~/lib/utils'

import { CopyButton } from './copy-button'

// 20 Money and identity, ID: shortened in the middle in mono, the full 32 hex characters one click
// away. `full` shows the complete value (record headers).
interface IdTextProps {
  value: string
  full?: boolean
  copy?: boolean
  label?: string
  className?: string
}

export function IdText({ value, full = false, copy = true, label = 'Copy ID', className }: IdTextProps) {
  return (
    <span className={cn('inline-flex items-center gap-1 font-mono text-small text-muted-foreground', className)}>
      <span title={full ? undefined : value} className="whitespace-nowrap">
        {full ? value : shortId(value)}
      </span>
      {copy ? <CopyButton value={value} label={label} /> : null}
    </span>
  )
}
