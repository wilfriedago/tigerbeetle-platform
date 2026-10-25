import { useMemo } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type AssetSummary, type Hold, type Transaction } from '@tigerbeetle-platform/api-client'
import { LockIcon, PlusIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount, useAmountText } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { MetadataTags } from '~/components/ledger/metadata'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Stat, StatGroup } from '~/components/ledger/stat'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { holdQueries } from '~/features/holds/api'
import { ledgerQueries } from '~/features/ledgers/api'
import { transactionQueries } from '~/features/transactions/api'
import { PostingsSummary } from '~/features/transactions/components/postings-summary'

import { shortId } from '~/lib/ids'
import { assetCode } from '~/lib/money'
import { formatDate, formatRelative } from '~/lib/time'

// docs/console.md, Overview: money issued per asset, accounts, open holds and declined writes,
// the latest transactions, and what needs attention soon.

const route = getRouteApi('/_console/ledgers/$ledger/')

const column = columnHelper<Transaction>()

function recentColumns(ledger: string) {
  return [
    column.accessor('id', {
      cell: (info) => (
        <Link
          to="/ledgers/$ledger/transactions/$txId"
          params={{ ledger, txId: info.getValue() }}
          className="font-mono outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
        >
          {shortId(info.getValue())}
        </Link>
      ),
      header: 'Transaction'
    }),
    column.accessor('postings', {
      cell: (info) => <PostingsSummary postings={info.getValue()} />,
      header: 'Postings',
      meta: { className: 'w-full' }
    }),
    column.accessor('committedAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Committed' }),
    column.display({
      cell: ({ row }) => {
        const [first] = row.original.postings
        return first ? <Amount value={first.amount} asset={first.asset} /> : null
      },
      header: 'Amount',
      id: 'amount',
      meta: { align: 'right' }
    })
  ]
}

function IssuedStat({ asset }: { asset: AssetSummary }) {
  const text = useAmountText(asset.issued, asset.asset)
  return <Stat label={`Money issued, ${asset.asset}`} value={text} unit={assetCode(asset.asset)} sub="minus the balance of world" />
}

function ReservedLine({ amounts }: { amounts: readonly { asset: string; amount: string }[] }) {
  if (amounts.length === 0) {
    return <>nothing reserved</>
  }
  return (
    <>
      {amounts.slice(0, 2).map((item, index) => (
        <span key={item.asset}>
          {index > 0 ? ', ' : null}
          <ReservedAmount asset={item.asset} amount={item.amount} />
        </span>
      ))}{' '}
      reserved
    </>
  )
}

function ReservedAmount({ asset, amount }: { asset: string; amount: string }) {
  return (
    <>
      {useAmountText(amount, asset)} {assetCode(asset)}
    </>
  )
}

/** Open holds that expire, soonest first. A hold without a timeout never shows here. */
function expiringSoon(holds: readonly Hold[]): Hold[] {
  return holds
    .filter((hold) => hold.expiresAt)
    .toSorted((a, b) => (a.expiresAt ?? '').localeCompare(b.expiresAt ?? ''))
    .slice(0, 3)
}

export function OverviewPage() {
  const { ledger } = route.useParams()
  const navigate = route.useNavigate()
  const summary = useQuery(ledgerQueries.detail(ledger))
  const stats = useQuery(ledgerQueries.stats(ledger))
  const recent = useQuery(transactionQueries.list(ledger, { limit: 10 }))
  const holds = useQuery(holdQueries.list(ledger, { limit: 100, status: 'open' }))
  const declined = stats.data?.declined
  const topReason = declined?.reasons[0]
  const expiring = expiringSoon(holds.data?.data ?? [])
  const columns = useMemo(() => recentColumns(ledger), [ledger])

  return (
    <Page>
      <PageHeader
        mono
        title={ledger}
        titleAddon={
          summary.data ? <span className="text-small text-muted-foreground">Ledger, created {formatDate(summary.data.createdAt)}</span> : null
        }
        description={<MetadataTags metadata={summary.data?.metadata} />}
        actions={
          <>
            <Button variant="secondary" render={<Link to="/ledgers/$ledger/holds/new" params={{ ledger }} />}>
              <LockIcon />
              New hold
            </Button>
            <Button render={<Link to="/ledgers/$ledger/transactions/new" params={{ ledger }} />}>
              <PlusIcon />
              New transaction
            </Button>
          </>
        }
      />

      {stats.error ? <ProblemPanel error={stats.error} onRetry={() => void stats.refetch()} /> : null}
      {stats.data ? (
        <StatGroup>
          {stats.data.assets.slice(0, 2).map((asset) => (
            <IssuedStat key={asset.asset} asset={asset} />
          ))}
          <Stat
            label="Accounts"
            value={stats.data.accounts.toLocaleString('en-US')}
            sub={`${stats.data.accountsToday.toLocaleString('en-US')} created today`}
          />
          <Stat label="Open holds" value={stats.data.holds.open.toLocaleString('en-US')} sub={<ReservedLine amounts={stats.data.holds.reserved} />} />
          <Stat
            label="Declined writes, 24 h"
            value={stats.data.declined.total.toLocaleString('en-US')}
            sub={topReason ? `${topReason.count.toLocaleString('en-US')} ${topReason.reason}` : 'none'}
          />
        </StatGroup>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <Card className="self-start">
          <CardHeader>
            <CardTitle>Recent transactions</CardTitle>
            <CardDescription>Newest first, committed time from TigerBeetle</CardDescription>
            <CardAction>
              <Button variant="link" size="sm" render={<Link to="/ledgers/$ledger/transactions" params={{ ledger }} />}>
                View all
              </Button>
            </CardAction>
          </CardHeader>
          {recent.error ? (
            <CardContent>
              <ProblemPanel error={recent.error} onRetry={() => void recent.refetch()} />
            </CardContent>
          ) : (
            <div className="[&_[data-slot=table-container]]:rounded-none [&_[data-slot=table-container]]:border-0">
              <DataTable
                label="Recent transactions"
                columns={columns}
                data={recent.data?.data}
                getRowId={(tx) => tx.id}
                loading={recent.isPending}
                onRowClick={(tx) => void navigate({ params: { ledger, txId: tx.id }, to: '/ledgers/$ledger/transactions/$txId' })}
                empty={
                  <>
                    No transactions yet.{' '}
                    <Link
                      to="/ledgers/$ledger/transactions/new"
                      params={{ ledger }}
                      search={{ source: 'world' }}
                      className="text-primary-text hover:underline"
                    >
                      Fund an account
                    </Link>{' '}
                    to start.
                  </>
                }
              />
            </div>
          )}
        </Card>

        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Declined writes, last 24 h</CardTitle>
              <CardDescription>Kept under their Idempotency-Key</CardDescription>
              <CardAction>
                <Button variant="link" size="sm" render={<Link to="/ledgers/$ledger/requests" params={{ ledger }} search={{ status: 'declined' }} />}>
                  Requests
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              {declined && declined.reasons.length > 0 ? (
                declined.reasons.slice(0, 5).map((reason) => (
                  <div key={reason.reason} className="flex flex-col gap-1.5">
                    <div className="flex items-baseline justify-between gap-2 font-mono text-caption">
                      <span className="truncate">{reason.reason}</span>
                      <span className="text-muted-foreground tabular-nums">{reason.count.toLocaleString('en-US')}</span>
                    </div>
                    <div className="h-1 overflow-hidden rounded-full bg-bar-track" aria-hidden="true">
                      <div
                        className="h-full rounded-full bg-danger"
                        style={{ width: `${Math.max(4, (reason.count / Math.max(1, declined.total)) * 100)}%` }}
                      />
                    </div>
                  </div>
                ))
              ) : (
                <p className="text-small text-muted-foreground">No declined writes in the last 24 hours.</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Holds expiring soon</CardTitle>
              <CardDescription>Released by the cluster at timeout</CardDescription>
              <CardAction>
                <Button variant="link" size="sm" render={<Link to="/ledgers/$ledger/holds" params={{ ledger }} />}>
                  Holds
                </Button>
              </CardAction>
            </CardHeader>
            {expiring.length > 0 ? (
              <ul className="divide-y divide-border-subtle">
                {expiring.map((hold) => (
                  <li key={hold.id}>
                    <Link
                      to="/ledgers/$ledger/holds/$holdId"
                      params={{ holdId: hold.id, ledger }}
                      className="flex items-center gap-3 px-4 py-3 outline-none hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring focus-visible:ring-inset"
                    >
                      <LockIcon aria-hidden="true" className="size-4 shrink-0 text-warning-fg" />
                      <span className="flex min-w-0 flex-1 flex-col">
                        <Address value={hold.source} className="truncate" />
                        <span className="text-caption text-warning-fg">expires {formatRelative(hold.expiresAt ?? '')}</span>
                      </span>
                      <Amount value={hold.amount} asset={hold.asset} />
                    </Link>
                  </li>
                ))}
              </ul>
            ) : (
              <CardContent>
                <p className="text-small text-muted-foreground">No open hold has a timeout.</p>
              </CardContent>
            )}
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Assets in use</CardTitle>
              <CardAction>
                <Button variant="link" size="sm" render={<Link to="/ledgers/$ledger/assets" params={{ ledger }} />}>
                  Assets
                </Button>
              </CardAction>
            </CardHeader>
            {stats.data && stats.data.assets.length > 0 ? (
              <ul className="divide-y divide-border-subtle">
                {stats.data.assets.map((asset) => (
                  <li key={asset.asset} className="flex items-center gap-3 px-4 py-2.5">
                    <AssetBadge asset={asset.asset} />
                    <span className="font-mono text-small text-muted-foreground">ledger {asset.tbLedger}</span>
                    <span className="ml-auto text-caption text-muted-foreground tabular-nums">{asset.accounts.toLocaleString('en-US')} accounts</span>
                  </li>
                ))}
              </ul>
            ) : (
              <CardContent>
                <p className="text-small text-muted-foreground">An asset appears with its first transaction.</p>
              </CardContent>
            )}
          </Card>
        </div>
      </div>
    </Page>
  )
}
