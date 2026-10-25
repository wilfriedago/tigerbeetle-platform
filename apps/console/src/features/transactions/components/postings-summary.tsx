import { ArrowRightIcon } from 'lucide-react'

import { Address } from '~/components/ledger/address'

import { cn } from '~/lib/utils'

/** "world -> users:1234:wallet +2": the first posting of a chain and how many follow it. */
export function PostingsSummary({ postings, className }: { postings: readonly { source: string; destination: string }[]; className?: string }) {
  const [first] = postings
  if (!first) {
    return null
  }
  const more = postings.length - 1
  return (
    <span className={cn('inline-flex min-w-0 items-center gap-2', className)}>
      <Address value={first.source} />
      <ArrowRightIcon aria-label="to" className="size-3.5 shrink-0 text-subtle-foreground" />
      <Address value={first.destination} />
      {more > 0 ? <span className="font-mono text-[11px] text-subtle-foreground">+{more}</span> : null}
    </span>
  )
}
