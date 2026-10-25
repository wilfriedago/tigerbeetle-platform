import { type Posting } from '@tigerbeetle-platform/api-client'
import { ArrowLeftRightIcon, ArrowRightIcon, CircleCheckIcon, CircleDashedIcon, CircleDotIcon } from 'lucide-react'

import { Address } from '~/components/ledger/address'
import { useFormattedAmount } from '~/components/ledger/amount'
import { Button } from '~/components/ui/button'

import { assetCode, assetScale, formatAmount, formatSignedAmount } from '~/lib/money'
import { type Effect } from '~/lib/preflight'
import { cn } from '~/lib/utils'

import { type TransactionDraft } from './draft'

// The summary column of New transaction: the postings so far, the details, and their effect on
// balances. It stays beside every step, so the effect of each edit is visible where it is made.

function StepIcon({ state }: { state: 'done' | 'current' | 'todo' }) {
  if (state === 'done') {
    return <CircleCheckIcon aria-hidden="true" className="size-4 text-success-fg" />
  }
  return state === 'current' ? (
    <CircleDotIcon aria-hidden="true" className="size-4 text-primary-text" />
  ) : (
    <CircleDashedIcon aria-hidden="true" className="size-4 text-subtle-foreground" />
  )
}

function Section({
  title,
  state,
  aside,
  children
}: {
  title: string
  state: 'done' | 'current' | 'todo'
  aside?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="flex flex-col gap-3 border-b border-border-subtle p-4 last:border-b-0">
      <div className="flex items-center gap-2">
        <StepIcon state={state} />
        <h2 className="flex-1 text-small font-semibold">{title}</h2>
        {aside}
      </div>
      {children}
    </section>
  )
}

function PostingLine({ posting, index, ready }: { posting: Posting; index: number; ready: boolean }) {
  const amount = useFormattedAmount(posting.amount || '0', posting.asset)
  return (
    <li className="flex gap-2 text-small">
      <span className="w-3 shrink-0 font-mono text-caption text-subtle-foreground">{index + 1}</span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="flex flex-wrap items-center gap-1.5">
          {posting.source ? <Address value={posting.source} /> : <span className="text-warning-fg">source not set</span>}
          <ArrowRightIcon aria-label="to" className="size-3 text-subtle-foreground" />
          {posting.destination ? <Address value={posting.destination} /> : <span className="text-warning-fg">destination not set</span>}
        </span>
        <span className={cn('font-mono text-caption', ready ? 'text-muted-foreground' : 'text-subtle-foreground')}>
          {posting.amount ? amount : 'amount not set'}
        </span>
      </span>
    </li>
  )
}

function EffectLine({ effect }: { effect: Effect }) {
  const scale = assetScale(effect.asset)
  return (
    <li className="flex flex-col gap-0.5">
      <div className="flex items-baseline justify-between gap-2">
        <Address value={effect.address} />
        <span className="font-mono text-small font-medium tabular-nums">{formatSignedAmount(effect.delta, scale)}</span>
      </div>
      <div className="flex items-center gap-1.5 font-mono text-[11px] text-subtle-foreground tabular-nums">
        {effect.exists ? formatAmount(effect.before, scale) : 'new'}
        <ArrowRightIcon aria-hidden="true" className="size-3" />
        {formatAmount(effect.after, scale)} {assetCode(effect.asset)}
      </div>
    </li>
  )
}

interface SummaryPanelProps {
  step: number
  postings: readonly Posting[]
  ready: readonly boolean[]
  draft: TransactionDraft
  eventTypeLabel: string
  effects: readonly Effect[]
  onEdit: (step: number) => void
  committed?: boolean
}

export function SummaryPanel({ step, postings, ready, draft, eventTypeLabel, effects, onEdit, committed = false }: SummaryPanelProps) {
  const readyCount = ready.filter(Boolean).length
  const metadataKeys = draft.metadata.map((row) => row.key.trim()).filter(Boolean)
  const notSet = <span className="text-subtle-foreground">Not set</span>
  return (
    <aside aria-label="Summary" className="flex flex-col self-start overflow-hidden rounded-lg border border-border bg-surface xl:sticky xl:top-6">
      <Section
        title="Postings"
        state={step > 0 ? 'done' : 'current'}
        aside={
          step > 0 ? (
            <Button variant="link" size="sm" onClick={() => onEdit(0)}>
              Edit
            </Button>
          ) : (
            <span className="text-caption text-primary-text">
              {readyCount} of {postings.length} ready
            </span>
          )
        }
      >
        <ol className="flex flex-col gap-2.5">
          {postings.map((posting, index) => (
            // oxlint-disable-next-line react/no-array-index-key -- postings are positional
            <PostingLine key={index} posting={posting} index={index} ready={ready[index] ?? false} />
          ))}
        </ol>
      </Section>
      <Section
        title="Details"
        state={step > 1 ? 'done' : step === 1 ? 'current' : 'todo'}
        aside={
          step > 1 ? (
            <Button variant="link" size="sm" onClick={() => onEdit(1)}>
              Edit
            </Button>
          ) : (
            <span className={cn('text-caption', step === 1 ? 'text-primary-text' : 'text-subtle-foreground')}>
              {step === 1 ? 'In progress' : 'Step 2'}
            </span>
          )
        }
      >
        <dl className="grid grid-cols-[104px_1fr] gap-x-3 gap-y-2 text-caption">
          <dt className="text-muted-foreground">Reference</dt>
          <dd className="truncate font-mono">{draft.reference.trim() || notSet}</dd>
          <dt className="text-muted-foreground">Event type</dt>
          <dd className="truncate font-mono">{eventTypeLabel}</dd>
          <dt className="text-muted-foreground">Business time</dt>
          <dd className="font-mono">{draft.businessTime || notSet}</dd>
          <dt className="text-muted-foreground">Metadata</dt>
          <dd className="truncate font-mono">{metadataKeys.length > 0 ? metadataKeys.join(', ') : notSet}</dd>
        </dl>
      </Section>
      <Section
        title="Balance effect"
        state={committed ? 'done' : 'todo'}
        aside={<span className="text-caption text-subtle-foreground">{step === 2 ? 'On commit' : 'Preview'}</span>}
      >
        {effects.length > 0 ? (
          <ul className="flex flex-col gap-3">
            {effects.map((effect) => (
              <EffectLine key={`${effect.asset}-${effect.address}`} effect={effect} />
            ))}
          </ul>
        ) : (
          <p className="flex items-center gap-2 text-caption text-muted-foreground">
            <ArrowLeftRightIcon aria-hidden="true" className="size-3.5" />
            Pick accounts and amounts to see how balances move.
          </p>
        )}
      </Section>
    </aside>
  )
}
