import { useEffect } from 'react'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { ApiError, type CreateTransactionRequest, type Transaction } from '@tigerbeetle-platform/api-client'
import { ArrowDownToLineIcon, CheckIcon, InfoIcon, KeyRoundIcon, LoaderCircleIcon, LockOpenIcon, PencilIcon, XIcon } from 'lucide-react'

import { Address } from '~/components/ledger/address'
import { AmountDisplay, useFormattedAmount } from '~/components/ledger/amount'
import { BalanceCard } from '~/components/ledger/balance'
import { CopyButton } from '~/components/ledger/copy-button'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { IdText } from '~/components/ledger/id-text'
import { PostingChain, type StepState } from '~/components/ledger/posting-chain'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { BusinessTime, Timestamp } from '~/components/ledger/timestamp'
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card'

import { accountQueries } from '~/features/accounts/api'
import { requestQueries } from '~/features/requests/api'
import { storedProblem } from '~/features/requests/stored-problem'
import { systemQueries } from '~/features/system/api'

import { shortId } from '~/lib/ids'
import { assetScale, formatAmount, formatSignedAmount, sumAmounts } from '~/lib/money'
import { type Effect, type Simulation } from '~/lib/preflight'
import { toProblem } from '~/lib/problem'
import { formatClock, formatTime } from '~/lib/time'
import { useNow } from '~/lib/use-now'
import { cn } from '~/lib/utils'

import { transactionQueries } from '../api'

// The three answers of a commit (docs/console.md, Submission states, ADR-0007): committed, declined
// with the failing posting, or pending while the cluster has not answered. A pending write keeps
// going without the page; the write request record says how it ends.

export interface LogEntry {
  at: Date
  state: 'ok' | 'warn' | 'pending' | 'fail'
  text: string
}

export type Outcome =
  | { kind: 'sending' }
  | { kind: 'committed'; transaction: Transaction }
  | { kind: 'pending'; error: unknown }
  | { kind: 'declined'; error: unknown }

export interface Submission {
  body: CreateTransactionRequest
  key: string
  startedAt: Date
  /** When the answer arrived. */
  endedAt?: Date
  checksPassed: number
  /** The preview at commit time: balances before, and what each source could spend. */
  simulation: Simulation
  log: LogEntry[]
  outcome: Outcome
}

/** Whether an error leaves the write unresolved rather than answered (ADR-0007). */
export function isUnresolved(error: unknown): boolean {
  if (!(error instanceof ApiError)) {
    return true
  }
  return error.status >= 500 || error.type.endsWith('/request-in-flight') || error.type.endsWith('/write-pending')
}

function problemExtensions(error: unknown): { posting: number | undefined; results: string[] } {
  const { extensions } = toProblem(error)
  return {
    posting: typeof extensions.posting === 'number' ? extensions.posting : undefined,
    results: Array.isArray(extensions.results) ? extensions.results.map(String) : []
  }
}

function totalOf(body: CreateTransactionRequest): { amount: string; asset: string } | undefined {
  const assets = new Set(body.postings.map((posting) => posting.asset))
  const [asset] = assets
  if (assets.size !== 1 || !asset) {
    return undefined
  }
  return { amount: sumAmounts(body.postings.map((posting) => posting.amount)), asset }
}

type Tone = 'accent' | 'success' | 'danger'

const BANNER: Record<Tone, string> = {
  accent: 'bg-primary-subtle',
  danger: 'bg-danger-bg',
  success: 'bg-success-bg'
}

const BADGE_ICON: Record<Tone, string> = {
  accent: 'bg-primary text-primary-foreground',
  danger: 'bg-danger text-danger-on',
  success: 'bg-success text-white'
}

type ProgressState = 'done' | 'current' | 'failed' | 'todo' | 'skipped'

function ProgressDot({ state }: { state: ProgressState }) {
  if (state === 'done') {
    return (
      <span className="flex size-4 items-center justify-center rounded-full bg-success text-white">
        <CheckIcon className="size-3" strokeWidth={3} />
      </span>
    )
  }
  if (state === 'failed') {
    return (
      <span className="flex size-4 items-center justify-center rounded-full bg-danger text-danger-on">
        <XIcon className="size-3" strokeWidth={3} />
      </span>
    )
  }
  if (state === 'current') {
    return <LoaderCircleIcon aria-hidden="true" className="size-4 animate-spin text-primary-text" />
  }
  return <span className="size-4 rounded-full border-2 border-border-strong bg-surface" />
}

function Progress({ steps }: { steps: readonly { title: string; note: React.ReactNode; state: ProgressState }[] }) {
  return (
    <ol className="grid gap-4 border-t border-border px-6 py-4 sm:grid-cols-4">
      {steps.map((step, index) => (
        <li key={step.title} className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <ProgressDot state={step.state} />
            {index < steps.length - 1 ? (
              <span className={cn('hidden h-0.5 flex-1 sm:block', step.state === 'done' ? 'bg-success' : 'bg-border')} />
            ) : null}
          </div>
          <span
            className={cn(
              'text-small font-medium',
              step.state === 'failed' && 'text-danger-fg',
              (step.state === 'todo' || step.state === 'skipped') && 'text-muted-foreground'
            )}
          >
            {step.title}
          </span>
          <span className="text-caption text-muted-foreground">{step.note}</span>
        </li>
      ))}
    </ol>
  )
}

interface BannerProps {
  tone: Tone
  icon: React.ReactNode
  badge: React.ReactNode
  meta: React.ReactNode
  title: string
  description: React.ReactNode
  totalLabel: string
  body: CreateTransactionRequest
  steps: readonly { title: string; note: React.ReactNode; state: ProgressState }[]
}

function Banner({ tone, icon, badge, meta, title, description, totalLabel, body, steps }: BannerProps) {
  const total = totalOf(body)
  return (
    <section className="overflow-hidden rounded-lg border border-border bg-surface" aria-live="polite">
      <div className={cn('flex flex-wrap gap-5 p-6', BANNER[tone])}>
        <span className={cn('flex size-11 shrink-0 items-center justify-center rounded-full', BADGE_ICON[tone])}>{icon}</span>
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-3 font-mono text-caption text-muted-foreground">
            {badge}
            {meta}
          </div>
          <h1 className="text-[22px] leading-tight font-semibold">{title}</h1>
          <p className="max-w-[820px] text-small text-muted-foreground">{description}</p>
        </div>
        <div className="flex flex-col items-end gap-1">
          <span className="text-caption text-muted-foreground">{totalLabel}</span>
          {total ? (
            <AmountDisplay value={total.amount} asset={total.asset} size="md" />
          ) : (
            <span className="text-page-title">{new Set(body.postings.map((p) => p.asset)).size} assets</span>
          )}
          <span className="text-caption text-subtle-foreground">
            across {body.postings.length} {body.postings.length === 1 ? 'posting' : 'postings'}
          </span>
        </div>
      </div>
      <Progress steps={steps} />
    </section>
  )
}

interface ResultViewProps {
  ledger: string
  submission: Submission
  /** Resolves a pending write once its record settles. */
  onResolved: (outcome: Outcome) => void
}

export function ResultView({ ledger, submission, onResolved }: ResultViewProps) {
  const { outcome } = submission
  if (outcome.kind === 'committed') {
    return <Committed ledger={ledger} submission={submission} transaction={outcome.transaction} />
  }
  if (outcome.kind === 'declined') {
    return <Declined ledger={ledger} submission={submission} error={outcome.error} />
  }
  return <Pending ledger={ledger} submission={submission} onResolved={onResolved} />
}

function Pending({ ledger, submission, onResolved }: ResultViewProps) {
  const sending = submission.outcome.kind === 'sending'
  const error = submission.outcome.kind === 'pending' ? submission.outcome.error : undefined
  const request = useQuery({ ...requestQueries.detail(ledger, submission.key), enabled: !sending })
  const transaction = useQuery({ ...transactionQueries.detail(ledger, request.data?.targetId ?? ''), enabled: request.data?.status === 'committed' })
  const reconciler = useQuery(systemQueries.reconciler())
  const recorded = request.data !== undefined
  const grace = reconciler.data?.gracePeriodSeconds

  const pending = submission.outcome.kind === 'pending'
  const settledTransaction = transaction.data
  const record = request.data
  // The record settled while the page was open: hand the answer to the page.
  useEffect(() => {
    if (!pending) {
      return
    }
    if (settledTransaction) {
      onResolved({ kind: 'committed', transaction: settledTransaction })
    } else if (record && (record.status === 'declined' || record.status === 'failed')) {
      onResolved({ error: storedProblem(record) ?? new Error(`The write ended ${record.status}.`), kind: 'declined' })
    }
  }, [pending, settledTransaction, record, onResolved])

  const now = useNow()
  const elapsed = Math.max(0, Math.round((now.getTime() - submission.startedAt.getTime()) / 1000))
  return (
    <div className="flex flex-col gap-6">
      <Banner
        tone="accent"
        icon={<LoaderCircleIcon className="size-5 animate-spin" />}
        badge={<StatusBadge status="pending" />}
        meta={
          <>
            {request.data?.requestId ? <span>{request.data.requestId}</span> : null}
            <span>started {formatTime(submission.startedAt)}</span>
          </>
        }
        title={sending ? 'Sending to the API' : 'Waiting for TigerBeetle'}
        description={
          sending
            ? 'The console sent the request with its Idempotency-Key. TigerBeetle applies every posting together or none of them.'
            : `The API recorded your write and sent the chain to the cluster. TigerBeetle applies all ${submission.body.postings.length} postings together or none of them, so no balance has moved yet.`
        }
        totalLabel="Moving"
        body={submission.body}
        steps={[
          { note: `${submission.checksPassed} preflight checks passed`, state: 'done', title: 'Checked in the console' },
          {
            note: recorded ? `Key ${submission.key.slice(0, 13)} stored as pending` : sending ? 'Sending' : 'No answer yet',
            state: recorded ? 'done' : 'current',
            title: 'Recorded by the API'
          },
          {
            note: recorded ? `Attempt ${request.data?.attempts ?? 1}, same transfer IDs` : 'Not sent yet',
            state: recorded ? 'current' : 'todo',
            title: 'Applying in TigerBeetle'
          },
          { note: 'Balances update once every posting is applied', state: 'todo', title: 'Committed' }
        ]}
      />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <PostingChain
            postings={submission.body.postings}
            status="pending"
            summary={`${submission.body.postings.length} postings, waiting to be applied together.`}
            steps={submission.body.postings.map(() => ({ state: 'pending' as StepState }))}
          />
          <ActivityLog entries={submission.log} />
          {error && !recorded && request.isError ? (
            <ProblemPanel error={error} title="The API has no record of this write">
              <p className="text-caption text-muted-foreground">Nothing was written. Sending it again with the same key is safe.</p>
            </ProblemPanel>
          ) : null}
        </div>
        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Write request</CardTitle>
            </CardHeader>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="Idempotency-Key" mono termWidth="sm">
                  <span className="flex items-center justify-between gap-2">
                    <span className="truncate">{submission.key}</span>
                    <CopyButton value={submission.key} label="Copy Idempotency-Key" />
                  </span>
                </DescriptionItem>
                {request.data ? (
                  <DescriptionItem term="Transaction ID" mono termWidth="sm">
                    <IdText value={request.data.targetId} className="text-foreground" />
                  </DescriptionItem>
                ) : null}
                <DescriptionItem term="Record" termWidth="sm">
                  {request.data ? <StatusBadge status={request.data.status} /> : <span className="text-muted-foreground">not stored yet</span>}
                </DescriptionItem>
                <DescriptionItem term="Elapsed" termWidth="sm">
                  {elapsed} s{grace ? ` of a ${grace} s grace period` : ''}
                </DescriptionItem>
              </DescriptionList>
            </CardContent>
          </Card>
          <div className="flex gap-3 rounded-lg border border-primary-border bg-primary-subtle p-4">
            <InfoIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-primary-text" />
            <div className="flex flex-col gap-1">
              <span className="text-small font-semibold">Safe to leave this page</span>
              <p className="text-small text-muted-foreground">
                The write keeps going without you. If this page closes, the reconciler settles it{grace ? ` after ${grace} s` : ''}. Find it under{' '}
                <Link
                  to="/ledgers/$ledger/requests"
                  params={{ ledger }}
                  search={{ key: submission.key }}
                  className="text-primary-text hover:underline"
                >
                  Requests
                </Link>
                .
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function ActivityLog({ entries }: { entries: readonly LogEntry[] }) {
  const icon = {
    fail: <XIcon className="size-3.5 text-danger-fg" />,
    ok: <CheckIcon className="size-3.5 text-success-fg" />,
    pending: <LoaderCircleIcon className="size-3.5 animate-spin text-primary-text" />,
    warn: <InfoIcon className="size-3.5 text-warning-fg" />
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Activity</CardTitle>
        <div className="text-caption text-muted-foreground">What the console and the API did, newest last.</div>
      </CardHeader>
      <ol className="flex flex-col gap-1.5 bg-surface-sunken px-4 py-3 font-mono text-caption">
        {entries.map((entry, index) => (
          // oxlint-disable-next-line react/no-array-index-key -- the log only grows
          <li key={index} className="flex items-center gap-3">
            <span className="text-subtle-foreground tabular-nums">{formatClock(entry.at)}</span>
            {icon[entry.state]}
            <span className="min-w-0 break-words">{entry.text}</span>
          </li>
        ))}
      </ol>
    </Card>
  )
}

function Committed({ ledger, submission, transaction }: { ledger: string; submission: Submission; transaction: Transaction }) {
  return (
    <div className="flex flex-col gap-6">
      <Banner
        tone="success"
        icon={<CheckIcon className="size-5" strokeWidth={3} />}
        badge={<StatusBadge status="committed" />}
        meta={
          <>
            <span>tx {shortId(transaction.id)}</span>
            <span>committed {formatClock(new Date(transaction.committedAt))}</span>
          </>
        }
        title="Transaction committed"
        description={`TigerBeetle applied all ${transaction.postings.length} postings as one linked chain. The balances below are read back from the cluster, not computed by the console.`}
        totalLabel="Moved"
        body={submission.body}
        steps={[
          { note: `${submission.checksPassed} preflight checks passed`, state: 'done', title: 'Checked in the console' },
          { note: `Key ${submission.key.slice(0, 13)} stored`, state: 'done', title: 'Recorded by the API' },
          { note: `${transaction.postings.length} transfers created`, state: 'done', title: 'Applied in TigerBeetle' },
          { note: `Record committed at ${formatClock(new Date(transaction.committedAt))}`, state: 'done', title: 'Committed' }
        ]}
      />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <PostingChain
            postings={transaction.postings}
            status="committed"
            ledger={ledger}
            summary={`${transaction.postings.length} postings, all applied.`}
          />
          <section className="flex flex-col gap-3">
            <h2 className="flex items-baseline gap-2 text-body font-semibold">
              Balances now <span className="text-caption font-normal text-muted-foreground">Read from TigerBeetle after the commit</span>
            </h2>
            <div className="grid gap-3 md:grid-cols-2">
              {submission.simulation.effects.map((effect) => (
                <BalanceNow key={`${effect.asset}-${effect.address}`} ledger={ledger} effect={effect} />
              ))}
            </div>
          </section>
        </div>
        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Transaction</CardTitle>
            </CardHeader>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="ID" termWidth="sm">
                  <IdText value={transaction.id} className="text-foreground" />
                </DescriptionItem>
                <DescriptionItem term="Committed" termWidth="sm">
                  <Timestamp value={transaction.committedAt} />
                </DescriptionItem>
                {transaction.businessTime ? (
                  <DescriptionItem term="Business time" termWidth="sm">
                    <BusinessTime value={transaction.businessTime} />
                  </DescriptionItem>
                ) : null}
                {transaction.reference ? (
                  <DescriptionItem term="Reference" mono termWidth="sm">
                    {transaction.reference}
                  </DescriptionItem>
                ) : null}
                <DescriptionItem term="Event type" mono termWidth="sm">
                  {transaction.eventType.name} (code {transaction.eventType.code})
                </DescriptionItem>
                <DescriptionItem term="Idempotency-Key" mono termWidth="sm">
                  <span className="break-all">{transaction.idempotencyKey}</span>
                </DescriptionItem>
              </DescriptionList>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Need to undo it?</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3 text-small text-muted-foreground">
              <p>
                A revert is a new transaction with every posting reversed. TigerBeetle can decline it like any other, and there is no force option.
              </p>
              <Link to="/ledgers/$ledger/transactions/$txId" params={{ ledger, txId: transaction.id }} className="text-primary-text hover:underline">
                Open the transaction to revert it
              </Link>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

function BalanceNow({ ledger, effect }: { ledger: string; effect: Effect }) {
  const account = useQuery(accountQueries.detail(ledger, effect.address))
  const now = account.data?.balances.find((balance) => balance.asset === effect.asset)
  const scale = assetScale(effect.asset)
  const delta = now ? (BigInt(now.balance) - BigInt(effect.before)).toString() : effect.delta
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center justify-between gap-2">
        <Address value={effect.address} ledger={ledger} />
        <span
          className={cn(
            'rounded-sm px-1.5 font-mono text-caption tabular-nums',
            delta.startsWith('-')
              ? 'bg-quiet-bg text-muted-foreground'
              : delta === '0'
                ? 'bg-quiet-bg text-muted-foreground'
                : 'bg-success-bg text-success-fg'
          )}
        >
          {formatSignedAmount(delta, scale)}
        </span>
      </div>
      {now ? <AmountDisplay value={now.balance} asset={effect.asset} size="md" /> : <span className="h-8 animate-pulse rounded bg-surface-sunken" />}
      <span className="font-mono text-caption text-subtle-foreground">
        {effect.exists ? `was ${formatAmount(effect.before, scale)}` : 'new account'}
      </span>
    </div>
  )
}

function Declined({ ledger, submission, error }: { ledger: string; submission: Submission; error: unknown }) {
  const problem = toProblem(error)
  const { posting: failedIndex, results } = problemExtensions(error)
  const failed = failedIndex === undefined ? undefined : submission.body.postings[failedIndex]
  const reason = results[failedIndex ?? -1] ?? problem.type?.split('/').pop() ?? 'declined'
  const tbDeclined = failedIndex !== undefined
  const steps = submission.body.postings.map((_, index) => {
    if (failedIndex === undefined) {
      return { state: 'skipped' as StepState }
    }
    if (index === failedIndex) {
      const posting = submission.body.postings[index]
      let why = ''
      if (posting && reason.includes('exceeds_credits')) {
        why = `: ${posting.source} would go below zero in ${posting.asset}`
      } else if (posting && reason.includes('exceeds_debits')) {
        why = `: ${posting.destination} would take more credits than debits in ${posting.asset}`
      }
      return { note: `${reason}${why}`, state: 'failed' as StepState }
    }
    return { note: `not applied: ${results[index] ?? 'linked_event_failed'}`, state: 'skipped' as StepState }
  })
  return (
    <div className="flex flex-col gap-6">
      <Banner
        tone="danger"
        icon={<XIcon className="size-5" strokeWidth={3} />}
        badge={<StatusBadge status="declined" />}
        meta={
          <>
            {problem.requestId ? <span>{problem.requestId}</span> : null}
            {submission.endedAt ? <span>declined {formatClock(submission.endedAt)}</span> : null}
          </>
        }
        title={tbDeclined ? 'TigerBeetle declined the transaction' : problem.title}
        description={
          tbDeclined
            ? `${problem.detail ?? `Posting ${(failedIndex ?? 0) + 1} failed.`} One failure stops the whole chain, so no balance moved.`
            : (problem.detail ?? 'The API refused the request. Nothing was applied.')
        }
        totalLabel="Not moved"
        body={submission.body}
        steps={[
          { note: `${submission.checksPassed} preflight checks passed`, state: 'done', title: 'Checked in the console' },
          {
            note: `Key ${submission.key.slice(0, 13)} stored`,
            state: tbDeclined ? 'done' : 'failed',
            title: tbDeclined ? 'Recorded by the API' : 'Refused by the API'
          },
          {
            note: tbDeclined ? `${reason} on posting ${(failedIndex ?? 0) + 1}` : 'Not sent',
            state: tbDeclined ? 'failed' : 'skipped',
            title: 'Declined by TigerBeetle'
          },
          { note: 'Skipped. The chain was rolled back as a whole', state: 'skipped', title: 'Committed' }
        ]}
      />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <PostingChain
            postings={submission.body.postings}
            status="declined"
            ledger={ledger}
            steps={steps}
            summary={failedIndex === undefined ? 'Nothing applied.' : `Nothing applied. Posting ${failedIndex + 1} failed.`}
          />
          {!tbDeclined ? <ProblemPanel error={error} /> : null}
          {failed && failedIndex !== undefined && reason.includes('exceeds_credits') ? (
            <WaysForward ledger={ledger} submission={submission} index={failedIndex} address={failed.source} asset={failed.asset} />
          ) : null}
        </div>
        <div className="flex flex-col gap-6">
          {failed ? (
            <FailedAccount ledger={ledger} address={reason.includes('exceeds_debits') ? failed.destination : failed.source} asset={failed.asset} />
          ) : null}
          <Card>
            <CardHeader>
              <CardTitle>This key is spent</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <KeyRoundIcon aria-hidden="true" className="size-4 text-muted-foreground" />
                <span className="flex-1 truncate font-mono text-small">{submission.key}</span>
                <StatusBadge status="declined" />
              </div>
              <p className="text-small text-muted-foreground">
                The answer is stored under this key, so sending it again only replays the decline. Retrying sends a new key; the console makes one for
                you.
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

function FailedAccount({ ledger, address, asset }: { ledger: string; address: string; asset: string }) {
  const account = useQuery(accountQueries.detail(ledger, address))
  const balance = account.data?.balances.find((entry) => entry.asset === asset)
  if (!balance) {
    return null
  }
  return (
    <div className="flex flex-col gap-2">
      <Address value={address} ledger={ledger} />
      <BalanceCard balance={balance} holds={account.data?.openHolds} />
    </div>
  )
}

function WaysForward({
  ledger,
  submission,
  index,
  address,
  asset
}: {
  ledger: string
  submission: Submission
  index: number
  address: string
  asset: string
}) {
  const account = useQuery(accountQueries.detail(ledger, address))
  const balance = account.data?.balances.find((entry) => entry.asset === asset)
  const spendable = submission.simulation.spendable[index]
  const failure = submission.simulation.failures.find((entry) => entry.index === index)
  const spendText = useFormattedAmount(spendable ?? '0', asset)
  const shortText = useFormattedAmount(failure?.shortBy ?? '0', asset)
  const reservedText = useFormattedAmount(balance?.reserved ?? '0', asset)
  const holds = account.data?.openHolds ?? 0
  return (
    <section className="flex flex-col gap-3">
      <h2 className="flex items-baseline gap-2 text-body font-semibold">
        Ways forward <span className="text-caption font-normal text-muted-foreground">Pick one, then retry with a new key</span>
      </h2>
      <div className="grid gap-3 md:grid-cols-3">
        <WayCard icon={<PencilIcon />} title={`Lower posting ${index + 1}`}>
          <p>
            <span className="font-mono">{address}</span>{' '}
            {spendable === undefined ? 'cannot cover it' : <>can spend {spendText} at that point of the chain</>}.
          </p>
          <span className="text-primary-text">Use Edit and retry below.</span>
        </WayCard>
        <WayCard icon={<ArrowDownToLineIcon />} title="Fund the account first">
          <p>
            Add {failure ? <>at least {shortText}</> : 'funds'} to <span className="font-mono">{address}</span>, then retry.
          </p>
          <Link
            to="/ledgers/$ledger/transactions/new"
            params={{ ledger }}
            search={{ asset, destination: address, source: 'world' }}
            className="text-primary-text hover:underline"
          >
            New deposit
          </Link>
        </WayCard>
        {holds > 0 ? (
          <WayCard icon={<LockOpenIcon />} title="Release a hold">
            <p>
              {holds} open {holds === 1 ? 'hold reserves' : 'holds reserve'} {reservedText} on this account.
            </p>
            <Link to="/ledgers/$ledger/holds" params={{ ledger }} search={{ address, status: 'open' }} className="text-primary-text hover:underline">
              Open holds
            </Link>
          </WayCard>
        ) : null}
      </div>
    </section>
  )
}

function WayCard({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface p-4 text-small text-muted-foreground">
      <span className="flex size-7 items-center justify-center rounded-md border border-border bg-surface-sunken text-muted-foreground [&_svg]:size-3.5">
        {icon}
      </span>
      <span className="font-medium text-foreground">{title}</span>
      {children}
    </div>
  )
}
