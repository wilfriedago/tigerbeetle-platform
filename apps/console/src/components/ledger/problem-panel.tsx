import { CircleXIcon, RotateCwIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'

import { toProblem } from '~/lib/problem'
import { cn } from '~/lib/utils'

import { CopyButton } from './copy-button'

// 12 Feedback, Problem Panel: renders any application/problem+json answer. Title and status,
// the problem type in mono, the detail, when to retry, and a footer with the copyable request ID.
interface ProblemPanelProps {
  error: unknown
  /** Overrides the problem title, for example "Transaction declined". */
  title?: string
  onRetry?: () => void
  className?: string
  children?: React.ReactNode
}

export function ProblemPanel({ error, title, onRetry, className, children }: ProblemPanelProps) {
  const problem = toProblem(error)
  return (
    <div role="alert" className={cn('flex flex-col gap-2.5 rounded-md border border-danger-border bg-danger-bg p-4', className)}>
      <div className="flex items-center gap-2">
        <CircleXIcon aria-hidden="true" className="size-4 shrink-0 text-danger-fg" />
        <span className="flex-1 text-body font-semibold text-foreground">{title ?? problem.title}</span>
        {problem.status ? <span className="font-mono text-caption text-danger-fg">{problem.status}</span> : null}
      </div>
      {problem.type && problem.type !== 'about:blank' ? <div className="font-mono text-caption text-muted-foreground">{problem.type}</div> : null}
      {problem.detail ? <p className="text-small leading-[1.45] text-foreground">{problem.detail}</p> : null}
      {problem.retryAfter ? <p className="text-caption text-muted-foreground">The API asks to retry after {problem.retryAfter} s.</p> : null}
      {children}
      {problem.requestId || onRetry ? (
        <div className="flex items-center gap-2 border-t border-danger-border pt-2">
          {problem.requestId ? (
            <>
              <span className="text-caption text-muted-foreground">Request ID</span>
              <span className="flex-1 truncate font-mono text-caption text-foreground">{problem.requestId}</span>
              <CopyButton value={problem.requestId} label="Copy request ID" />
            </>
          ) : (
            <span className="flex-1" />
          )}
          {onRetry ? (
            <Button variant="secondary" size="sm" onClick={onRetry}>
              <RotateCwIcon />
              Retry
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
