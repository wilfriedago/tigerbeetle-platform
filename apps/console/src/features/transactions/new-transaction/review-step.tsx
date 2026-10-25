import { useState } from 'react'

import { type CreateTransactionRequest } from '@tigerbeetle-platform/api-client'
import { CircleAlertIcon, CircleCheckIcon, CircleXIcon, CodeIcon, LoaderCircleIcon, TerminalIcon } from 'lucide-react'

import { CodeBlock } from '~/components/ledger/code-block'
import { useCopy } from '~/components/ledger/copy-button'
import { IdempotencyKeyField } from '~/components/ledger/idempotency-key-field'
import { PostingChain } from '~/components/ledger/posting-chain'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { type AccountStates } from '~/features/accounts/use-account-states'

import { apiPath, curlCommand } from '~/lib/curl'
import { assetCode, assetScale, formatAmount } from '~/lib/money'
import { type Simulation } from '~/lib/preflight'
import { cn } from '~/lib/utils'

import { MAX_POSTINGS } from './draft'

type CheckState = 'ok' | 'warn' | 'fail' | 'pending'

export interface Check {
  state: CheckState
  text: React.ReactNode
}

const ICON = { fail: CircleXIcon, ok: CircleCheckIcon, pending: LoaderCircleIcon, warn: CircleAlertIcon } as const
const TONE = { fail: 'text-danger-fg', ok: 'text-success-fg', pending: 'animate-spin text-muted-foreground', warn: 'text-warning-fg' } as const

/** The preflight checks of docs/console.md: one asset per posting, accounts to create, the cap, balances. */
export function preflightChecks(body: CreateTransactionRequest, states: AccountStates, simulation: Simulation): Check[] {
  const assets = [...new Set(body.postings.map((posting) => posting.asset))]
  const addresses = [...new Set(body.postings.flatMap((posting) => [posting.source, posting.destination]))]
  const checks: Check[] = [
    {
      state: 'ok',
      text:
        assets.length === 1 ? (
          <>
            Every posting has one asset: <span className="font-mono">{assets[0]}</span>.
          </>
        ) : (
          <>
            Each posting moves one asset; this chain uses {assets.length}: <span className="font-mono">{assets.join(', ')}</span>.
          </>
        )
    }
  ]
  if (states.loading) {
    checks.push({ state: 'pending', text: 'Reading the accounts and their rules.' })
  } else {
    const created = body.postings.flatMap((posting) =>
      [posting.source, posting.destination]
        .filter((address) => states.stateOf(address, posting.asset)?.exists === false)
        .map((address) => `${address} (${posting.asset})`)
    )
    const unique = [...new Set(created)]
    checks.push(
      unique.length === 0
        ? { state: 'ok', text: `All ${addresses.length} accounts exist. None will be created.` }
        : {
            state: 'warn',
            text: (
              <>
                {unique.length} {unique.length === 1 ? 'account' : 'accounts'} will be created with their rule's flags, which never change:{' '}
                <span className="font-mono">{unique.join(', ')}</span>.
              </>
            )
          }
    )
  }
  checks.push({
    state: body.postings.length > MAX_POSTINGS ? 'fail' : 'ok',
    text: `${body.postings.length} of ${MAX_POSTINGS} postings.`
  })
  if (!states.loading) {
    checks.push(
      simulation.failures.length === 0
        ? { state: 'ok', text: 'Available balances cover every debit, counting earlier postings.' }
        : {
            state: 'fail',
            text: (
              <>
                {simulation.failures.map((failure) => (
                  <span key={`${failure.index}-${failure.address}`} className="block">
                    Posting {failure.index + 1} would take <span className="font-mono">{failure.address}</span>{' '}
                    {failure.reason === 'exceeds-credits' ? 'below zero' : 'above its debits'} by{' '}
                    <span className="font-mono">{`${formatAmount(failure.shortBy, assetScale(failure.asset))} ${assetCode(failure.asset)}`}</span>.
                    TigerBeetle would decline the whole chain.
                  </span>
                ))}
              </>
            )
          }
    )
  }
  return checks
}

export function CheckList({ checks }: { checks: readonly Check[] }) {
  return (
    <ul className="flex flex-col gap-2.5">
      {checks.map((check, index) => {
        const Icon = ICON[check.state]
        return (
          // Checks are a fixed sequence; their position is their identity.
          // oxlint-disable-next-line react/no-array-index-key
          <li key={index} className="flex items-start gap-2.5 text-small">
            <Icon aria-label={check.state} className={cn('mt-0.5 size-4 shrink-0', TONE[check.state])} />
            <span>{check.text}</span>
          </li>
        )
      })}
    </ul>
  )
}

interface ReviewStepProps {
  ledger: string
  body: CreateTransactionRequest
  idempotencyKey: string
  checks: readonly Check[]
}

export function ReviewStep({ ledger, body, idempotencyKey, checks }: ReviewStepProps) {
  const [showJson, setShowJson] = useState(false)
  const [copied, copy] = useCopy()
  const path = apiPath('ledgers', ledger, 'transactions')
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-card-title">Review and commit</CardTitle>
        <CardDescription className="text-small">Nothing is sent until you commit. The postings are applied as one linked chain.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 p-5">
        <PostingChain
          postings={body.postings}
          status="not sent"
          steps={body.postings.map(() => ({ state: 'pending' }))}
          summary={`${body.postings.length} ${body.postings.length === 1 ? 'posting' : 'postings'}. All are applied, or none is.`}
        />
        <Card>
          <CardHeader>
            <CardTitle>Before you commit</CardTitle>
            <CardDescription>Checked in the console. TigerBeetle has the last word.</CardDescription>
          </CardHeader>
          <CardContent>
            <CheckList checks={checks} />
          </CardContent>
        </Card>
        <div className="flex flex-col gap-3 rounded-md border border-border bg-surface-sunken p-3">
          <div className="flex flex-wrap items-center gap-3">
            <CodeIcon aria-hidden="true" className="size-4 text-muted-foreground" />
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="text-small font-medium">Request body</span>
              <span className="truncate font-mono text-caption text-muted-foreground">POST {path}</span>
            </div>
            <Button variant="secondary" size="sm" aria-expanded={showJson} onClick={() => setShowJson((value) => !value)}>
              <CodeIcon />
              {showJson ? 'Hide JSON' : 'View JSON'}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => copy(curlCommand({ body, idempotencyKey, method: 'POST', path }))}>
              <TerminalIcon />
              {copied ? 'Copied' : 'Copy as curl'}
            </Button>
          </div>
          {showJson ? <CodeBlock value={body} label="application/json" /> : null}
        </div>
        <IdempotencyKeyField
          value={idempotencyKey}
          description="Created when this form opened. Reused if you retry; renewed when you change the request."
        />
      </CardContent>
    </Card>
  )
}
