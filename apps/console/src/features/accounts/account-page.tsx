import { useMemo, useState } from 'react'

import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Account, type Hold, type Transaction } from '@tigerbeetle-platform/api-client'
import { ArrowLeftRightIcon, LockIcon, PencilIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { Address, AddressPrefixLinks } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { BalanceCard } from '~/components/ledger/balance'
import { BalanceHistoryChart } from '~/components/ledger/balance-history-chart'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { MetadataTags } from '~/components/ledger/metadata'
import { MetadataDialog } from '~/components/ledger/metadata-dialog'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'
import { Tabs, TabsContent, TabsCount, TabsList, TabsTrigger } from '~/components/ui/tabs'

import { holdQueries } from '~/features/holds/api'
import { transactionQueries } from '~/features/transactions/api'

import { shortId } from '~/lib/ids'
import { activeOrganization, useOrganization } from '~/lib/organization'
import { formatDate } from '~/lib/time'

import { accountQueries, useReplaceAccountMetadata } from './api'

// docs/console.md, Account: one balance card per asset, the flags frozen at creation (ADR-0006),
// this account's postings and holds, and the balance history when the account keeps it.

const route = getRouteApi('/_console/$org/ledgers/$ledger/accounts/$address')

const LIMIT_TEXT = {
  credits_must_not_exceed_debits: 'credits must not exceed debits',
  debits_must_not_exceed_credits: 'debits must not exceed credits',
  none: 'none'
} as const

type Tab = 'transactions' | 'holds' | 'history' | 'metadata'

/** One row per posting that touches the account: the other side is the counterparty. */
interface Movement {
  key: string
  transaction: Transaction
  counterparty: string
  asset: string
  debit?: string
  credit?: string
}

function movementsOf(transactions: readonly Transaction[], address: string): Movement[] {
  return transactions.flatMap((transaction) =>
    transaction.postings.flatMap((posting): Movement[] => {
      if (posting.source === address) {
        return [{ asset: posting.asset, counterparty: posting.destination, debit: posting.amount, key: posting.transferId, transaction }]
      }
      if (posting.destination === address) {
        return [{ asset: posting.asset, counterparty: posting.source, credit: posting.amount, key: posting.transferId, transaction }]
      }
      return []
    })
  )
}

const movement = columnHelper<Movement>()

function movementColumns(ledger: string) {
  const org = activeOrganization() ?? ''
  return [
    movement.display({ cell: ({ row }) => <Timestamp value={row.original.transaction.committedAt} />, header: 'Committed', id: 'committed' }),
    movement.display({
      cell: ({ row }) => (
        <Link
          to="/$org/ledgers/$ledger/transactions/$txId"
          params={{ ledger, org, txId: row.original.transaction.id }}
          className="font-mono outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
        >
          {shortId(row.original.transaction.id)}
        </Link>
      ),
      header: 'Transaction',
      id: 'transaction'
    }),
    movement.accessor('counterparty', {
      cell: (info) => <Address value={info.getValue()} ledger={ledger} />,
      header: 'Counterparty',
      meta: { className: 'w-full' }
    }),
    movement.accessor('asset', { cell: (info) => <AssetBadge asset={info.getValue()} />, header: 'Asset' }),
    movement.display({
      cell: ({ row }) =>
        row.original.debit ? <Amount value={row.original.debit} asset={row.original.asset} /> : <span className="text-subtle-foreground">-</span>,
      header: 'Debit',
      id: 'debit',
      meta: { align: 'right' }
    }),
    movement.display({
      cell: ({ row }) =>
        row.original.credit ? <Amount value={row.original.credit} asset={row.original.asset} /> : <span className="text-subtle-foreground">-</span>,
      header: 'Credit',
      id: 'credit',
      meta: { align: 'right' }
    })
  ]
}

const hold = columnHelper<Hold>()

function holdColumns(ledger: string, address: string) {
  const org = activeOrganization() ?? ''
  return [
    hold.accessor('id', {
      cell: (info) => (
        <Link to="/$org/ledgers/$ledger/holds/$holdId" params={{ holdId: info.getValue(), ledger, org }} className="font-mono hover:underline">
          {shortId(info.getValue())}
        </Link>
      ),
      header: 'Hold'
    }),
    hold.accessor('status', { cell: (info) => <StatusBadge status={info.getValue()} />, header: 'Status' }),
    hold.display({
      cell: ({ row }) => <Address value={row.original.source === address ? row.original.destination : row.original.source} ledger={ledger} />,
      header: 'Counterparty',
      id: 'counterparty',
      meta: { className: 'w-full' }
    }),
    hold.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' }),
    hold.accessor('amount', {
      cell: ({ row }) => <Amount value={row.original.amount} asset={row.original.asset} />,
      header: 'Amount',
      meta: { align: 'right' }
    })
  ]
}

export function AccountPage() {
  const org = useOrganization()
  const { ledger, address } = route.useParams()
  const { tab = 'transactions' } = route.useSearch()
  const navigate = route.useNavigate()
  const { data: account } = useSuspenseQuery(accountQueries.detail(ledger, address))
  const [editing, setEditing] = useState(false)
  const replaceMetadata = useReplaceAccountMetadata(ledger, address)
  const { createdAt } = account
  const historyAssets = account.balances.filter((balance) => balance.history).map((balance) => balance.asset)

  return (
    <Page>
      <PageHeader
        title={<AddressPrefixLinks value={address} ledger={ledger} />}
        mono
        description={
          <span className="flex flex-wrap items-center gap-2">
            <span>Created {formatDate(createdAt)} by its first transaction</span>
            <MetadataTags metadata={account.metadata} />
          </span>
        }
        actions={
          <>
            <Button variant="secondary" render={<Link to="/$org/ledgers/$ledger/holds/new" params={{ ledger, org }} search={{ source: address }} />}>
              <LockIcon />
              New hold
            </Button>
            <Button render={<Link to="/$org/ledgers/$ledger/transactions/new" params={{ ledger, org }} search={{ source: address }} />}>
              <ArrowLeftRightIcon />
              New transaction
            </Button>
          </>
        }
      />

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid gap-4 md:grid-cols-2">
          {account.balances.length === 0 ? (
            <Card className="md:col-span-2">
              <CardContent className="text-small text-muted-foreground">This address has no TigerBeetle account yet.</CardContent>
            </Card>
          ) : (
            account.balances.map((balance) => (
              <BalanceCard key={balance.asset} balance={balance} holds={account.balances.length === 1 ? account.openHolds : undefined} />
            ))
          )}
        </div>
        <FrozenCard account={account} />
      </div>

      <Tabs
        value={tab}
        onValueChange={(value: Tab) =>
          void navigate({ replace: true, search: (prev) => ({ ...prev, tab: value === 'transactions' ? undefined : value }) })
        }
      >
        <TabsList>
          <TabsTrigger value="transactions">Transactions</TabsTrigger>
          <TabsTrigger value="holds">Holds {account.openHolds > 0 ? <TabsCount>{account.openHolds} open</TabsCount> : null}</TabsTrigger>
          <TabsTrigger value="history">Balance history</TabsTrigger>
          <TabsTrigger value="metadata">Metadata</TabsTrigger>
        </TabsList>
        <TabsContent value="transactions">
          <AccountTransactions ledger={ledger} address={address} />
        </TabsContent>
        <TabsContent value="holds">
          <AccountHolds ledger={ledger} address={address} />
        </TabsContent>
        <TabsContent value="history">
          {historyAssets.length > 0 ? (
            <BalanceHistory ledger={ledger} address={address} assets={historyAssets} />
          ) : (
            <Card>
              <CardContent className="text-small text-muted-foreground">
                This account does not keep balance history. The flag is set when an account is created and never changes; set{' '}
                <Link to="/$org/ledgers/$ledger/settings/rules" params={{ ledger, org }} className="text-primary-text hover:underline">
                  history on its rule
                </Link>{' '}
                for accounts created later.
              </CardContent>
            </Card>
          )}
        </TabsContent>
        <TabsContent value="metadata">
          <Card>
            <CardHeader>
              <CardTitle>Metadata</CardTitle>
              <CardDescription>Stored in Postgres with the address. Replacing it never touches the money.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col items-start gap-3">
              <MetadataTags metadata={account.metadata} stacked empty={<p className="text-small text-muted-foreground">No metadata.</p>} />
              <Button variant="secondary" size="sm" onClick={() => setEditing(true)}>
                <PencilIcon />
                Edit metadata
              </Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
      <MetadataDialog
        open={editing}
        onOpenChange={setEditing}
        title="Edit account metadata"
        description={
          <>
            The map replaces the current metadata of <span className="font-mono">{address}</span>. Balances and flags never change.
          </>
        }
        metadata={account.metadata}
        onSave={async (metadata) => replaceMetadata.mutateAsync(metadata)}
      />
    </Page>
  )
}

function FrozenCard({ account }: { account: Account }) {
  const [first] = account.balances
  // One rule usually creates every asset account of an address; differences are listed per asset.
  const uniform = account.balances.every(
    (balance) => balance.limit === first?.limit && balance.history === first.history && balance.code === first.code && balance.rule === first.rule
  )
  return (
    <Card className="self-start">
      <CardHeader>
        <CardTitle>Frozen at creation</CardTitle>
        <CardDescription>
          Set by rule <span className="font-mono">{first?.rule ?? '*'}</span> and stored on the TigerBeetle account. They never change.
        </CardDescription>
      </CardHeader>
      <CardContent className="py-1">
        {first && uniform ? (
          <DescriptionList>
            <DescriptionItem term="Limit" termWidth="sm">
              {LIMIT_TEXT[first.limit]}
            </DescriptionItem>
            <DescriptionItem term="History" termWidth="sm">
              {first.history ? 'on' : 'off'}
            </DescriptionItem>
            <DescriptionItem term="Code" mono termWidth="sm">
              {first.code}
            </DescriptionItem>
            <DescriptionItem term="Matched rule" mono termWidth="sm">
              {first.rule}
            </DescriptionItem>
            <DescriptionItem term="Ledger numbers" termWidth="sm">
              {account.balances.map((balance) => `${balance.asset} is ${balance.tbLedger}`).join(', ')}
            </DescriptionItem>
          </DescriptionList>
        ) : (
          <DescriptionList>
            {account.balances.map((balance) => (
              <DescriptionItem key={balance.asset} term={balance.asset} termWidth="sm">
                {LIMIT_TEXT[balance.limit]}, history {balance.history ? 'on' : 'off'}, code {balance.code}, rule{' '}
                <span className="font-mono">{balance.rule}</span>, ledger {balance.tbLedger}
              </DescriptionItem>
            ))}
          </DescriptionList>
        )}
      </CardContent>
    </Card>
  )
}

function AccountTransactions({ ledger, address }: { ledger: string; address: string }) {
  const [cursor, setCursor] = useState<string | undefined>()
  const list = useQuery({ ...transactionQueries.list(ledger, { address, cursor, limit: 25 }), placeholderData: keepPreviousData })
  const rows = useMemo(() => movementsOf(list.data?.data ?? [], address), [list.data, address])
  const columns = useMemo(() => movementColumns(ledger), [ledger])
  const pager = useCursorStack({ cursor, next: list.data?.next, onChange: setCursor, scope: address })
  return (
    <div className="flex flex-col gap-4">
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Postings of this account"
        columns={columns}
        data={rows}
        getRowId={(row) => row.key}
        loading={list.isPending}
        empty="No transaction has touched this account."
      />
      <CursorPager
        info="Newest first. 25 transactions per page."
        hasNext={pager.hasNext}
        hasPrevious={pager.hasPrevious}
        onNext={pager.next}
        onPrevious={pager.previous}
        busy={list.isFetching}
      />
    </div>
  )
}

function AccountHolds({ ledger, address }: { ledger: string; address: string }) {
  const list = useQuery(holdQueries.list(ledger, { address, limit: 50 }))
  const columns = useMemo(() => holdColumns(ledger, address), [ledger, address])
  // Open holds first: they are the ones that still reserve money.
  const rows = (list.data?.data ?? []).toSorted((a, b) => Number(b.status === 'open') - Number(a.status === 'open'))
  return (
    <div className="flex flex-col gap-4">
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Holds of this account"
        columns={columns}
        data={rows}
        getRowId={(row) => row.id}
        loading={list.isPending}
        empty="No holds on this account."
      />
    </div>
  )
}

function BalanceHistory({ ledger, address, assets }: { ledger: string; address: string; assets: readonly string[] }) {
  const [asset, setAsset] = useState(assets[0] ?? '')
  const history = useQuery(accountQueries.history(ledger, address, asset))
  const points = history.data?.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>Balance in {asset}</CardTitle>
        <CardDescription>Read from TigerBeetle&apos;s account history, the last {points.length} changes.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {assets.length > 1 ? (
          <div className="flex gap-2">
            {assets.map((option) => (
              <Button
                key={option}
                size="sm"
                variant={option === asset ? 'secondary' : 'ghost'}
                aria-pressed={option === asset}
                onClick={() => setAsset(option)}
                className="font-mono"
              >
                {option}
              </Button>
            ))}
          </div>
        ) : null}
        {history.error ? <ProblemPanel error={history.error} onRetry={() => void history.refetch()} /> : null}
        {points.length > 0 ? <BalanceHistoryChart points={points} asset={asset} /> : null}
        {points.length > 0 ? (
          <Table aria-label="Balance history" className="[&_td]:h-8">
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead align="right">Balance</TableHead>
                <TableHead align="right">Reserved</TableHead>
                <TableHead align="right">Credits</TableHead>
                <TableHead align="right">Debits</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {points.map((point) => (
                <TableRow key={point.timestamp}>
                  <TableCell>
                    <Timestamp value={point.at} />
                  </TableCell>
                  <TableCell align="right">
                    <Amount value={point.balance} asset={asset} />
                  </TableCell>
                  <TableCell align="right">
                    <Amount value={point.reserved} asset={asset} />
                  </TableCell>
                  <TableCell align="right">
                    <Amount value={point.credits} asset={asset} />
                  </TableCell>
                  <TableCell align="right">
                    <Amount value={point.debits} asset={asset} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : null}
      </CardContent>
    </Card>
  )
}
