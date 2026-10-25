import { useMemo } from 'react'

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Transaction } from '@tigerbeetle-platform/api-client'
import { PlusIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter, SelectFilter } from '~/components/data-table/filters'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { Amount } from '~/components/ledger/amount'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { BusinessTime, Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'

import { ledgerQueries } from '~/features/ledgers/api'
import { settingsQueries } from '~/features/settings/api'

import { shortId } from '~/lib/ids'

import { transactionQueries } from './api'
import { PostingsSummary } from './components/postings-summary'
import { rangeStart, TIME_RANGE_KEYS, TIME_RANGES } from './time-range'

const route = getRouteApi('/_console/ledgers/$ledger/transactions/')

const column = columnHelper<Transaction>()

function transactionColumns(ledger: string) {
  return [
    column.accessor('id', {
      cell: ({ row }) => (
        <span className="inline-flex items-center gap-2">
          <Link
            to="/ledgers/$ledger/transactions/$txId"
            params={{ ledger, txId: row.original.id }}
            className="font-mono outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
          >
            {shortId(row.original.id)}
          </Link>
          {row.original.revertedBy ? <StatusBadge status="reverted" /> : null}
          {row.original.reverts ? <StatusBadge status="revert" /> : null}
        </span>
      ),
      header: 'Transaction'
    }),
    column.accessor('committedAt', {
      cell: ({ row }) => (
        <span className="flex flex-col">
          <Timestamp value={row.original.committedAt} />
          {row.original.businessTime ? <BusinessTime value={row.original.businessTime} className="text-caption" /> : null}
        </span>
      ),
      header: 'Committed'
    }),
    column.accessor('postings', {
      cell: (info) => <PostingsSummary postings={info.getValue()} />,
      header: 'Postings',
      meta: { className: 'w-full' }
    }),
    column.accessor('eventType', {
      cell: (info) =>
        info.getValue().name === 'default' ? (
          <span className="text-subtle-foreground">default</span>
        ) : (
          <span className="font-mono">{info.getValue().name}</span>
        ),
      header: 'Event type'
    }),
    column.accessor('reference', { cell: (info) => <span className="font-mono">{info.getValue()}</span>, header: 'Reference' }),
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

export function TransactionsPage() {
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const { cursor, range, ...filters } = search
  const params = { ...filters, after: filters.after ?? rangeStart(range), cursor, limit: 25 }
  const list = useQuery({ ...transactionQueries.list(ledger, params), placeholderData: keepPreviousData })
  const assets = useQuery(ledgerQueries.assets(ledger))
  const eventTypes = useQuery(settingsQueries.eventTypes(ledger))
  const columns = useMemo(() => transactionColumns(ledger), [ledger])
  const setFilter = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch, cursor: undefined }) })
  const pager = useCursorStack({
    cursor,
    next: list.data?.next,
    onChange: (next) => void navigate({ search: (prev) => ({ ...prev, cursor: next }) }),
    scope: JSON.stringify({ filters, range })
  })
  const filtered = Object.values(filters).some(Boolean) || range !== undefined

  return (
    <Page>
      <PageHeader
        title="Transactions"
        description={
          <>
            Every committed transaction on <span className="font-mono">{ledger}</span>, newest first.
          </>
        }
        actions={
          <Button render={<Link to="/ledgers/$ledger/transactions/new" params={{ ledger }} />}>
            <PlusIcon />
            New transaction
          </Button>
        }
      />
      <FilterBar aside={<Hint>No sort by amount: amounts live in TigerBeetle</Hint>}>
        <SearchFilter
          label="Address or prefix"
          placeholder="Address or prefix"
          value={search.address}
          onChange={(address) => setFilter({ address })}
        />
        <SelectFilter
          label="Asset"
          allLabel="All assets"
          mono
          value={search.asset}
          options={(assets.data?.data ?? []).map((asset) => ({ label: asset.asset, value: asset.asset }))}
          onChange={(asset) => setFilter({ asset })}
        />
        <SelectFilter
          label="Event type"
          allLabel="Any event type"
          mono
          value={search.eventType}
          options={(eventTypes.data?.data ?? []).map((type) => ({ label: type.name, value: type.name }))}
          onChange={(eventType) => setFilter({ eventType })}
        />
        <SelectFilter
          label="Committed"
          allLabel="Any time"
          value={range}
          options={TIME_RANGE_KEYS.map((key) => ({ label: TIME_RANGES[key].label, value: key }))}
          onChange={(next) => setFilter({ after: undefined, before: undefined, range: TIME_RANGE_KEYS.find((key) => key === next) })}
        />
        <SearchFilter
          label="Reference"
          placeholder="Reference"
          value={search.reference}
          onChange={(reference) => setFilter({ reference })}
          className="sm:w-44"
        />
      </FilterBar>
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Transactions"
        columns={columns}
        data={list.data?.data}
        getRowId={(tx) => tx.id}
        loading={list.isPending}
        onRowClick={(tx) => void navigate({ params: { ledger, txId: tx.id }, to: '/ledgers/$ledger/transactions/$txId' })}
        empty={
          filtered ? (
            'No transaction matches these filters. Declined writes are not transactions: they are listed under Requests.'
          ) : (
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
              from world to start.
            </>
          )
        }
      />
      <CursorPager hasNext={pager.hasNext} hasPrevious={pager.hasPrevious} onNext={pager.next} onPrevious={pager.previous} busy={list.isFetching} />
    </Page>
  )
}
