import { useEffect, useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type AggregateBalance } from '@tigerbeetle-platform/api-client'
import { InfoIcon, NetworkIcon } from 'lucide-react'

import { Page, PageHeader } from '~/components/layout/page'
import { Amount, AmountDisplay } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { BalanceBar } from '~/components/ledger/balance'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { InputGroup, InputGroupAddon, InputGroupInput } from '~/components/ui/input-group'
import { Spinner } from '~/components/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'

import { accountQueries } from '~/features/accounts/api'

import { formatTime } from '~/lib/time'

import { getRecentPrefixes, rememberPrefix } from './recent-prefixes'

// docs/console.md, Balances: TigerBeetle has no prefix queries, so Postgres resolves the prefix to
// account IDs and the API reads them in chunks. The answer says when it was read and in how many
// chunks, because chunks are not one consistent snapshot.

const route = getRouteApi('/_console/ledgers/$ledger/balances')

const PREFIX_PATTERN = /^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*:?$/

/** "users" and "users:" name the same whole-segment prefix; the display form ends with a colon. */
function displayPrefix(prefix: string): string {
  return prefix.endsWith(':') ? prefix : `${prefix}:`
}

export function BalancesPage() {
  const { ledger } = route.useParams()
  const { prefix } = route.useSearch()
  const navigate = route.useNavigate()
  const [draft, setDraft] = useState({ source: prefix, text: prefix ? displayPrefix(prefix) : '' })
  const text = draft.source === prefix ? draft.text : prefix ? displayPrefix(prefix) : ''
  // Read on each render: a few bytes of localStorage, and the effect below may have just written it.
  const recent = getRecentPrefixes(ledger)
  const valid = PREFIX_PATTERN.test(text.trim())
  const sum = useQuery({ ...accountQueries.aggregate(ledger, prefix ?? ''), enabled: Boolean(prefix) })

  useEffect(() => {
    if (prefix && sum.data) {
      rememberPrefix(ledger, displayPrefix(prefix))
    }
  }, [ledger, prefix, sum.data])

  return (
    <Page>
      <PageHeader title="Balances" description="Sum the balances of every account under an address prefix, per asset." />
      <form
        className="flex flex-wrap items-center gap-3"
        onSubmit={(event) => {
          event.preventDefault()
          if (valid) {
            void navigate({ search: { prefix: text.trim() } })
          }
        }}
      >
        <InputGroup className="h-10 w-full sm:w-[420px]">
          <InputGroupAddon>
            <NetworkIcon aria-hidden="true" />
          </InputGroupAddon>
          <InputGroupInput
            aria-label="Address prefix"
            placeholder="users:"
            value={text}
            spellCheck={false}
            autoComplete="off"
            onChange={(event) => setDraft({ source: prefix, text: event.target.value })}
            aria-invalid={(text !== '' && !valid) || undefined}
            className="font-mono text-body placeholder:font-sans"
          />
        </InputGroup>
        <Button type="submit" size="lg" disabled={!valid} loading={sum.isFetching}>
          Sum balances
        </Button>
        {recent.length > 0 ? (
          <div className="ml-auto flex flex-wrap items-center gap-2">
            <span className="text-caption text-subtle-foreground">Recent</span>
            {recent.map((item) => (
              <Link
                key={item}
                to="/ledgers/$ledger/balances"
                params={{ ledger }}
                search={{ prefix: item }}
                className="rounded-sm border border-border bg-surface-sunken px-1.5 font-mono text-caption hover:bg-surface-hover"
              >
                {item}
              </Link>
            ))}
          </div>
        ) : null}
      </form>

      {prefix === undefined ? (
        <p className="text-small text-muted-foreground">
          Type a prefix on whole segments: <span className="font-mono">users:</span> sums every account under users, and{' '}
          <span className="font-mono">users:1</span> does not include <span className="font-mono">users:12</span>.
        </p>
      ) : null}
      {sum.error ? <ProblemPanel error={sum.error} onRetry={() => void sum.refetch()} /> : null}
      {sum.data ? (
        <>
          <Alert>
            <InfoIcon />
            <AlertTitle>
              Summed at {formatTime(sum.data.asOf)} from {sum.data.accounts.toLocaleString('en-US')}{' '}
              {sum.data.accounts === 1 ? 'account' : 'accounts'} in {sum.data.chunks} {sum.data.chunks === 1 ? 'chunk' : 'chunks'}
            </AlertTitle>
            <AlertDescription>
              TigerBeetle has no prefix queries, so Postgres resolves the prefix and the API reads the accounts in chunks of up to 8,189.{' '}
              {sum.data.chunks > 1 ? 'The total is not one consistent snapshot across chunks.' : 'One chunk is one consistent read.'}
            </AlertDescription>
          </Alert>
          {sum.data.assets.length === 0 ? (
            <p className="text-small text-muted-foreground">
              No account under <span className="font-mono">{displayPrefix(sum.data.prefix)}</span>.
            </p>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {sum.data.assets.map((asset) => (
                <AssetTotal key={asset.asset} total={asset} />
              ))}
            </div>
          )}
          <NextSegments ledger={ledger} prefix={sum.data.prefix} assets={sum.data.assets.map((asset) => asset.asset)} />
        </>
      ) : null}
      {prefix && sum.isPending ? (
        <p className="flex items-center gap-2 text-small text-muted-foreground">
          <Spinner /> Reading every account under <span className="font-mono">{displayPrefix(prefix)}</span>
        </p>
      ) : null}
    </Page>
  )
}

function AssetTotal({ total }: { total: AggregateBalance }) {
  return (
    <div className="flex flex-col gap-4 rounded-lg border border-border bg-surface p-5">
      <div className="flex items-center gap-2">
        <AssetBadge asset={total.asset} />
        <span className="text-caption text-muted-foreground">
          {total.accounts.toLocaleString('en-US')} {total.accounts === 1 ? 'account' : 'accounts'}
        </span>
      </div>
      <AmountDisplay value={total.balance} asset={total.asset} />
      <BalanceBar balance={total.balance} reserved={total.reserved} />
      <div className="flex flex-wrap gap-x-10 gap-y-2">
        <div className="flex flex-col gap-0.5">
          <span className="text-caption text-muted-foreground">Available</span>
          <Amount value={total.available} asset={total.asset} />
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-caption text-muted-foreground">Reserved</span>
          <Amount value={total.reserved} asset={total.asset} />
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-caption text-muted-foreground">Incoming, pending</span>
          <Amount value={total.incoming} asset={total.asset} />
        </div>
      </div>
    </div>
  )
}

/** One row per child segment, each its own prefix sum, so the reader can drill down. */
function NextSegments({ ledger, prefix, assets }: { ledger: string; prefix: string; assets: readonly string[] }) {
  const segments = useQuery(accountQueries.segments(ledger, prefix.replace(/:$/, '')))
  const children = (segments.data?.data ?? []).filter((segment) => segment.hasChildren || segment.isAccount)
  if (children.length === 0) {
    return null
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>By next segment</CardTitle>
        <CardDescription>Each row is its own prefix sum. Open one to drill down.</CardDescription>
      </CardHeader>
      <Table framed={false} aria-label="Sums by next segment">
        <TableHeader>
          <TableRow>
            <TableHead className="w-full">Prefix</TableHead>
            <TableHead align="right">Accounts</TableHead>
            {assets.map((asset) => (
              <TableHead key={asset} align="right" className="font-mono">
                {asset}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {children.slice(0, 50).map((segment) => (
            <SegmentRow key={segment.prefix} ledger={ledger} prefix={segment.prefix} accounts={segment.accounts} assets={assets} />
          ))}
        </TableBody>
      </Table>
    </Card>
  )
}

function SegmentRow({ ledger, prefix, accounts, assets }: { ledger: string; prefix: string; accounts: number; assets: readonly string[] }) {
  const sum = useQuery(accountQueries.aggregate(ledger, prefix))
  return (
    <TableRow>
      <TableCell>
        <Link to="/ledgers/$ledger/balances" params={{ ledger }} search={{ prefix }} className="font-mono hover:underline">
          {displayPrefix(prefix)}
        </Link>
      </TableCell>
      <TableCell align="right" className="font-mono tabular-nums">
        {accounts.toLocaleString('en-US')}
      </TableCell>
      {assets.map((asset) => {
        const total = sum.data?.assets.find((entry) => entry.asset === asset)
        return (
          <TableCell key={asset} align="right">
            {sum.isPending ? <Spinner /> : <Amount value={total?.balance ?? '0'} asset={asset} />}
          </TableCell>
        )
      })}
    </TableRow>
  )
}
