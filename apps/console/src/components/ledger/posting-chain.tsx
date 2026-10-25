import { ArrowRightIcon, Link2Icon } from 'lucide-react'

import { cn } from '~/lib/utils'

import { Address } from './address'
import { Amount } from './amount'
import { type Status, StatusBadge } from './status-badge'

// 21 Ledger patterns, Posting chain: draws TigerBeetle's linked transfers, where all postings
// succeed or none do (ADR-0007). On a decline the failing posting carries the TigerBeetle result in
// plain words; the others say they were not applied because they are linked.

export interface PostingView {
  source: string
  destination: string
  asset: string
  amount: string
}

interface PostingRowProps {
  posting: PostingView
  index?: number
  ledger?: string
  muted?: boolean
  className?: string
}

/** One posting on a line: index, source, arrow, destination, then the amount on the right. */
export function PostingRow({ posting, index, ledger, muted = false, className }: PostingRowProps) {
  return (
    <div className={cn('flex min-h-9 min-w-0 items-center gap-3', muted && 'opacity-60', className)}>
      {index === undefined ? null : <span className="w-4 shrink-0 font-mono text-caption text-subtle-foreground">{index + 1}</span>}
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2.5 gap-y-1">
        <Address value={posting.source} {...(ledger ? { ledger } : {})} />
        <ArrowRightIcon aria-label="to" className="size-3.5 shrink-0 text-subtle-foreground" />
        <Address value={posting.destination} {...(ledger ? { ledger } : {})} />
      </div>
      <Amount value={posting.amount} asset={posting.asset} className="shrink-0" />
    </div>
  )
}

export type StepState = 'applied' | 'pending' | 'failed' | 'skipped'

const DOT: Record<StepState, string> = {
  applied: 'bg-primary',
  failed: 'bg-danger',
  pending: 'bg-border-strong',
  skipped: 'bg-border-strong'
}

interface ChainStepProps {
  posting: PostingView
  index: number
  state: StepState
  note?: React.ReactNode
  first: boolean
  last: boolean
  ledger?: string
}

function ChainStep({ posting, index, state, note, first, last, ledger }: ChainStepProps) {
  return (
    <li className="flex gap-3">
      <div aria-hidden="true" className="flex w-3 shrink-0 flex-col items-center">
        <span className={cn('h-3.5 w-px', first ? 'bg-transparent' : 'bg-border-strong')} />
        <span className={cn('size-2 shrink-0 rounded-full', DOT[state])} />
        <span className={cn('w-px flex-1', last ? 'bg-transparent' : 'bg-border-strong')} />
      </div>
      <div className="flex min-w-0 flex-1 flex-col pb-1">
        <PostingRow posting={posting} index={index} muted={state === 'skipped'} {...(ledger ? { ledger } : {})} />
        {note ? (
          <div className={cn('-mt-1 pb-1 pl-7 font-mono text-[11px]', state === 'failed' ? 'text-danger-fg' : 'text-subtle-foreground')}>{note}</div>
        ) : null}
      </div>
    </li>
  )
}

interface PostingChainProps {
  postings: readonly PostingView[]
  status: Status
  /** The summary next to the title: "3 postings, all applied." */
  summary?: React.ReactNode
  /** Per-posting state and note, for declined chains. Defaults to all applied (or pending). */
  steps?: readonly { state: StepState; note?: React.ReactNode }[]
  ledger?: string
  className?: string
}

export function PostingChain({ postings, status, summary, steps, ledger, className }: PostingChainProps) {
  const defaultState: StepState = status === 'committed' ? 'applied' : 'pending'
  const declined = status === 'declined' || status === 'failed'
  return (
    <div className={cn('overflow-hidden rounded-lg border border-border bg-surface', className)}>
      <div className="flex items-center gap-2 border-b border-border-subtle px-4 py-3">
        <Link2Icon aria-hidden="true" className="size-3.5 text-muted-foreground" />
        <span className="text-small font-semibold text-foreground">Linked chain</span>
        <span className={cn('flex-1 truncate text-caption', declined ? 'text-danger-fg' : 'text-muted-foreground')}>{summary}</span>
        <StatusBadge status={status} />
      </div>
      <ol className="px-4 pt-1.5 pb-2">
        {postings.map((posting, index) => (
          <ChainStep
            // Postings are positional: the index is the leg number in the chain.
            // oxlint-disable-next-line react/no-array-index-key
            key={index}
            posting={posting}
            index={index}
            state={steps?.[index]?.state ?? defaultState}
            note={steps?.[index]?.note}
            first={index === 0}
            last={index === postings.length - 1}
            {...(ledger ? { ledger } : {})}
          />
        ))}
      </ol>
    </div>
  )
}
