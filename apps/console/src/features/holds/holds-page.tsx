import { useMemo, useState } from 'react'

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Hold, type HoldStatus } from '@tigerbeetle-platform/api-client'
import { ArrowRightIcon, LockIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter } from '~/components/data-table/filters'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Tabs, TabsCount, TabsList, TabsTrigger } from '~/components/ui/tabs'

import { shortId } from '~/lib/ids'
import { activeOrganization, useOrganization } from '~/lib/organization'

import { holdQueries } from './api'
import { CaptureDialog } from './components/capture-dialog'
import { HoldExpiry } from './components/hold-expiry'
import { VoidDialog } from './components/void-dialog'

// docs/console.md, Holds: two-phase transfers (ADR-0010) by status, with Capture and Void on the
// open ones. Counts come with every page, so the tabs always show them.

const route = getRouteApi('/_console/$org/ledgers/$ledger/holds/')

const STATUSES: readonly HoldStatus[] = ['open', 'captured', 'voided', 'expired']
const LABEL: Record<HoldStatus, string> = { captured: 'Captured', expired: 'Expired', open: 'Open', voided: 'Voided' }

const column = columnHelper<Hold>()

interface Actions {
  capture: (hold: Hold) => void
  void: (hold: Hold) => void
}

function holdColumns(ledger: string, status: HoldStatus, actions: Actions) {
  const org = activeOrganization() ?? ''
  return [
    column.accessor('id', {
      cell: (info) => (
        <Link
          to="/$org/ledgers/$ledger/holds/$holdId"
          params={{ holdId: info.getValue(), ledger, org }}
          className="font-mono outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
        >
          {shortId(info.getValue())}
        </Link>
      ),
      header: 'Hold'
    }),
    column.display({
      cell: ({ row }) => (
        <span className="inline-flex items-center gap-2">
          <Address value={row.original.source} />
          <ArrowRightIcon aria-label="to" className="size-3.5 text-subtle-foreground" />
          <Address value={row.original.destination} />
        </span>
      ),
      header: 'Source and destination',
      id: 'route',
      meta: { className: 'w-full' }
    }),
    column.accessor('amount', {
      cell: ({ row }) => <Amount value={row.original.amount} asset={row.original.asset} />,
      header: 'Amount',
      meta: { align: 'right' }
    }),
    ...(status === 'captured'
      ? [
          column.accessor('capturedAmount', {
            cell: ({ row }) => (row.original.capturedAmount ? <Amount value={row.original.capturedAmount} asset={row.original.asset} /> : null),
            header: 'Captured',
            meta: { align: 'right' }
          })
        ]
      : []),
    column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' }),
    status === 'open' || status === 'expired'
      ? column.display({ cell: ({ row }) => <HoldExpiry hold={row.original} />, header: 'Expires', id: 'expires' })
      : column.accessor('resolvedAt', { cell: (info) => (info.getValue() ? <Timestamp value={info.getValue() ?? ''} /> : null), header: 'Resolved' }),
    ...(status === 'open'
      ? [
          column.display({
            cell: ({ row }) => (
              <span className="flex justify-end gap-1">
                <Button variant="secondary" size="sm" onClick={() => actions.capture(row.original)}>
                  Capture
                </Button>
                <Button variant="ghost" size="sm" onClick={() => actions.void(row.original)}>
                  Void
                </Button>
              </span>
            ),
            header: () => <span className="sr-only">Actions</span>,
            id: 'actions',
            meta: { align: 'right' }
          })
        ]
      : [])
  ]
}

export function HoldsPage() {
  const org = useOrganization()
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const status = search.status ?? 'open'
  const list = useQuery({
    ...holdQueries.list(ledger, { address: search.address, cursor: search.cursor, limit: 25, status }),
    placeholderData: keepPreviousData
  })
  const [capturing, setCapturing] = useState<Hold | undefined>()
  const [voiding, setVoiding] = useState<Hold | undefined>()
  const columns = useMemo(() => holdColumns(ledger, status, { capture: setCapturing, void: setVoiding }), [ledger, status])
  const pager = useCursorStack({
    cursor: search.cursor,
    next: list.data?.next,
    onChange: (cursor) => void navigate({ search: (prev) => ({ ...prev, cursor }) }),
    scope: `${status}|${search.address ?? ''}`
  })
  const counts = list.data?.counts

  return (
    <Page>
      <PageHeader
        title="Holds"
        description="Pending transfers that reserve funds until they are captured, voided or expire."
        actions={
          <Button render={<Link to="/$org/ledgers/$ledger/holds/new" params={{ ledger, org }} />}>
            <LockIcon />
            New hold
          </Button>
        }
      />
      <Tabs
        value={status}
        onValueChange={(value: HoldStatus) =>
          void navigate({ search: (prev) => ({ ...prev, cursor: undefined, status: value === 'open' ? undefined : value }) })
        }
      >
        <TabsList>
          {STATUSES.map((value) => (
            <TabsTrigger key={value} value={value}>
              {LABEL[value]} {counts ? <TabsCount>{counts[value].toLocaleString('en-US')}</TabsCount> : null}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <FilterBar>
        <SearchFilter
          label="Address"
          placeholder="Source or destination address"
          value={search.address}
          onChange={(address) => void navigate({ replace: true, search: (prev) => ({ ...prev, address, cursor: undefined }) })}
          className="sm:w-72"
        />
      </FilterBar>
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label={`${LABEL[status]} holds`}
        columns={columns}
        data={list.data?.data}
        getRowId={(hold) => hold.id}
        loading={list.isPending}
        onRowClick={(hold) => void navigate({ params: { holdId: hold.id, ledger, org }, to: '/$org/ledgers/$ledger/holds/$holdId' })}
        empty={`No ${LABEL[status].toLowerCase()} holds${search.address ? ' for this address' : ''}.`}
      />
      <CursorPager hasNext={pager.hasNext} hasPrevious={pager.hasPrevious} onNext={pager.next} onPrevious={pager.previous} busy={list.isFetching} />
      <Hint>Expiry follows the cluster clock. An expired hold is released by TigerBeetle; capture or void then fails with hold-expired.</Hint>
      {capturing ? <CaptureDialog ledger={ledger} hold={capturing} open onOpenChange={(open) => !open && setCapturing(undefined)} /> : null}
      {voiding ? <VoidDialog ledger={ledger} hold={voiding} open onOpenChange={(open) => !open && setVoiding(undefined)} /> : null}
    </Page>
  )
}
