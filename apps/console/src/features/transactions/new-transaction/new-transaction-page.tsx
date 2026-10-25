import { useCallback, useState } from 'react'

import { useStore } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { ChevronLeftIcon, ChevronRightIcon, InfoIcon, PlusIcon, RefreshCwIcon } from 'lucide-react'

import { useAppForm } from '~/components/form/form'
import { Page, PageHeader } from '~/components/layout/page'
import { Stepper } from '~/components/ledger/stepper'
import { Button } from '~/components/ui/button'

import { useAccountStates } from '~/features/accounts/use-account-states'
import { ledgerQueries } from '~/features/ledgers/api'
import { settingsQueries } from '~/features/settings/api'

import { apiPath } from '~/lib/curl'
import { keyIsSpent, useIdempotencyKey } from '~/lib/idempotency'
import { shortId } from '~/lib/ids'
import { simulate } from '~/lib/preflight'
import { toProblem } from '~/lib/problem'
import { cn } from '~/lib/utils'

import { useCreateTransaction } from '../api'
import { DetailsStep } from './details-step'
import { emptyPosting, postingReady, previewPostings, toRequest } from './draft'
import { transactionFormOptions } from './form-options'
import { PostingsStep } from './postings-step'
import { isUnresolved, type LogEntry, type Outcome, ResultView, type Submission } from './result-view'
import { preflightChecks, ReviewStep } from './review-step'
import { SummaryPanel } from './summary-panel'

// docs/console.md, New transaction: Postings, Details, Review. Nothing is sent before Commit; the
// summary column shows the postings and their effect on balances at every step. The form survives
// a declined commit, so "Edit and retry" starts from what was sent, with a new key.

const route = getRouteApi('/_console/ledgers/$ledger/transactions/new')

const STEPS = ['Postings', 'Details', 'Review'] as const

/** Event handlers read the clock; keeping the call here keeps the component body pure. */
function now(): Date {
  return new Date()
}

export function NewTransactionPage() {
  const search = route.useSearch()
  // A new prefill ("Fund an account", "New deposit") is a new form.
  return <NewTransactionForm key={`${search.source ?? ''}|${search.destination ?? ''}|${search.asset ?? ''}`} />
}

function NewTransactionForm() {
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const [step, setStep] = useState(0)
  const [submission, setSubmission] = useState<Submission | null>(null)
  const idempotency = useIdempotencyKey()
  const create = useCreateTransaction(ledger)
  const assets = useQuery(ledgerQueries.assets(ledger))
  const eventTypes = useQuery(settingsQueries.eventTypes(ledger))

  const form = useAppForm({
    ...transactionFormOptions,
    defaultValues: {
      ...transactionFormOptions.defaultValues,
      postings: [emptyPosting(search.asset ?? '', search.source ?? '', search.destination ?? '')]
    }
  })
  const draft = useStore(form.store, (state) => state.values)

  const postings = previewPostings(draft.postings)
  const ready = draft.postings.map(postingReady)
  const states = useAccountStates(
    ledger,
    postings.flatMap((posting) => [posting.source, posting.destination])
  )
  const simulation = simulate(postings, states.stateOf)
  const body = toRequest(draft)
  const checks = preflightChecks(body, states, simulation)
  const eventType = eventTypes.data?.data.find((type) => type.name === draft.eventType)
  const eventTypeLabel = eventType ? `${eventType.name} (code ${eventType.code})` : draft.eventType

  const fieldsOf = (index: number) =>
    index === 0
      ? draft.postings.flatMap(
          (_, i) => [`postings[${i}].source`, `postings[${i}].destination`, `postings[${i}].asset`, `postings[${i}].amount`] as const
        )
      : (['reference', 'eventType', 'businessTime', 'metadata'] as const)

  const goTo = async (target: number) => {
    if (target <= step) {
      setStep(target)
      return
    }
    const fields = fieldsOf(step)
    await Promise.all(fields.map(async (name) => form.validateField(name, 'submit')))
    for (const name of fields) {
      form.setFieldMeta(name, (meta) => ({ ...meta, isTouched: true }))
    }
    if (fields.every((name) => (form.getFieldMeta(name)?.errors.length ?? 0) === 0)) {
      setStep(target)
    }
  }

  const commit = async () => {
    const key = idempotency.keyFor(body)
    const startedAt = now()
    const path = apiPath('ledgers', ledger, 'transactions')
    const log: LogEntry[] = [
      { at: startedAt, state: 'ok', text: `Preflight: ${checks.filter((check) => check.state === 'ok').length} of ${checks.length} checks passed` },
      { at: startedAt, state: 'ok', text: `POST ${path}  Idempotency-Key ${key}` }
    ]
    const base = { body, checksPassed: checks.filter((check) => check.state !== 'fail').length, key, simulation, startedAt }
    setSubmission({ ...base, log, outcome: { kind: 'sending' } })
    try {
      const transaction = await create.mutateAsync({ body, idempotencyKey: key })
      const at = now()
      setSubmission({
        ...base,
        endedAt: at,
        log: [...log, { at, state: 'ok', text: `201 Created: transaction ${shortId(transaction.id)}` }],
        outcome: { kind: 'committed', transaction }
      })
    } catch (error) {
      const at = now()
      const problem = toProblem(error)
      const answer: LogEntry = { at, state: isUnresolved(error) ? 'warn' : 'fail', text: `${problem.status ?? 'No answer'}: ${problem.title}` }
      if (isUnresolved(error)) {
        setSubmission({
          ...base,
          log: [...log, answer, { at, state: 'pending', text: 'Waiting for the write request to settle' }],
          outcome: { error, kind: 'pending' }
        })
        return
      }
      if (keyIsSpent(error)) {
        idempotency.renew()
      }
      setSubmission({ ...base, endedAt: at, log: [...log, answer], outcome: { error, kind: 'declined' } })
    }
  }

  const { renew } = idempotency
  const onResolved = useCallback(
    (outcome: Outcome) => {
      if (outcome.kind === 'declined') {
        renew()
      }
      setSubmission((current) => (current ? { ...current, endedAt: now(), outcome } : current))
    },
    [renew]
  )

  const editAndRetry = () => {
    setSubmission(null)
    setStep(0)
  }
  const startOver = () => {
    form.reset()
    idempotency.renew()
    setSubmission(null)
    setStep(0)
  }

  if (submission) {
    const outcome = submission.outcome.kind
    return (
      <Page>
        <ResultView ledger={ledger} submission={submission} onResolved={onResolved} />
        <ActionBar
          start={
            outcome === 'declined' ? (
              <Button variant="ghost" onClick={startOver}>
                Discard
              </Button>
            ) : outcome === 'committed' ? (
              <Button
                variant="ghost"
                onClick={() => {
                  idempotency.renew()
                  editAndRetry()
                }}
              >
                Start from this one
              </Button>
            ) : null
          }
          hint={
            outcome === 'declined'
              ? 'Retry opens the form with your postings and a new key.'
              : outcome === 'committed'
                ? 'Sending the same key again replays this result.'
                : 'Safe to leave: the write keeps going.'
          }
        >
          <Button variant="secondary" render={<Link to="/ledgers/$ledger/transactions" params={{ ledger }} />}>
            Go to transactions
          </Button>
          {submission.outcome.kind === 'committed' ? (
            <Button
              variant="secondary"
              render={<Link to="/ledgers/$ledger/transactions/$txId" params={{ ledger, txId: submission.outcome.transaction.id }} />}
            >
              View transaction
            </Button>
          ) : null}
          {outcome === 'declined' ? (
            <Button onClick={editAndRetry}>
              <RefreshCwIcon />
              Edit and retry
            </Button>
          ) : (
            <Button onClick={startOver} disabled={outcome === 'sending'}>
              <PlusIcon />
              New transaction
            </Button>
          )}
        </ActionBar>
      </Page>
    )
  }

  const failing = checks.some((check) => check.state === 'fail')
  return (
    <Page>
      <PageHeader title="New transaction" description="One transaction is one linked chain: every posting is applied, or none is." />
      <Stepper steps={STEPS} current={step} onSelect={(index) => void goTo(index)} />
      <form
        noValidate
        className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]"
        onSubmit={(event) => {
          event.preventDefault()
          event.stopPropagation()
          if (step < 2) {
            void goTo(step + 1)
          } else {
            void commit()
          }
        }}
      >
        <div className="min-w-0">
          {step === 0 ? (
            <PostingsStep
              form={form}
              ledger={ledger}
              assets={assets.data?.data.map((asset) => asset.asset) ?? []}
              states={states}
              simulation={simulation}
            />
          ) : null}
          {step === 1 ? <DetailsStep form={form} eventTypes={eventTypes.data?.data ?? []} /> : null}
          {step === 2 ? <ReviewStep ledger={ledger} body={body} idempotencyKey={idempotency.key} checks={checks} /> : null}
        </div>
        <SummaryPanel
          step={step}
          postings={postings}
          ready={ready}
          draft={draft}
          eventTypeLabel={eventTypeLabel}
          effects={simulation.effects}
          onEdit={setStep}
        />
        <ActionBar
          className="xl:col-span-2"
          start={
            <Button variant="ghost" render={<Link to="/ledgers/$ledger/transactions" params={{ ledger }} />}>
              Cancel
            </Button>
          }
          hint={
            step === 0
              ? `Postings ${ready.filter(Boolean).length} of ${ready.length} complete`
              : step === 1
                ? 'Changes are kept when you go back to postings.'
                : failing
                  ? 'The preview expects a decline. TigerBeetle decides; nothing is applied if it declines.'
                  : 'Nothing is applied if TigerBeetle declines it.'
          }
        >
          {step > 0 ? (
            <Button type="button" variant="secondary" onClick={() => setStep(step - 1)}>
              <ChevronLeftIcon />
              Back to {STEPS[step - 1]?.toLowerCase()}
            </Button>
          ) : null}
          <Button type="submit" loading={create.isPending}>
            {step === 2 ? (
              'Commit transaction'
            ) : (
              <>
                Continue to {STEPS[step + 1]?.toLowerCase()}
                <ChevronRightIcon />
              </>
            )}
          </Button>
        </ActionBar>
      </form>
    </Page>
  )
}

/** The bar at the bottom of a multi-step form: a quiet action on the left, the next step on the right. */
function ActionBar({
  start,
  hint,
  children,
  className
}: {
  start?: React.ReactNode
  hint?: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        'sticky bottom-0 z-10 -mx-4 -mb-16 flex flex-wrap items-center gap-3 border-t border-border bg-background/95 px-4 py-3 backdrop-blur sm:-mx-8 sm:px-8',
        className
      )}
    >
      {start}
      <span className="ml-auto hidden items-center gap-1.5 text-caption text-muted-foreground md:flex">
        <InfoIcon aria-hidden="true" className="size-3.5" />
        {hint}
      </span>
      <div className="flex flex-wrap items-center gap-2 max-md:ml-auto">{children}</div>
    </div>
  )
}
