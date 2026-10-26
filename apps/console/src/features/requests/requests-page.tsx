import { useMemo } from 'react'

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Operation, problemReason, type RequestStatus, type WriteRequest, type WriteRequestDetail } from '@tigerbeetle-platform/api-client'
import { InfoIcon, XIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter, SelectFilter } from '~/components/data-table/filters'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { CodeBlock } from '~/components/ledger/code-block'
import { CopyButton } from '~/components/ledger/copy-button'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { IdText } from '~/components/ledger/id-text'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Sheet, SheetBody, SheetClose, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '~/components/ui/sheet'
import { Skeleton } from '~/components/ui/skeleton'

import { systemQueries } from '~/features/system/api'

import { shortId } from '~/lib/ids'
import { useOrganization } from '~/lib/organization'
import { formatTime } from '~/lib/time'
import { useNow } from '~/lib/use-now'

import { requestQueries } from './api'
import { storedProblem } from './stored-problem'

// docs/console.md, Requests: the idempotency records of ADR-0007, the only place where declined and
// in-flight writes are visible. A pending record older than the reconciler's grace period is
// highlighted: the API lost its answer and the reconciler is settling it.

const route = getRouteApi('/_console/$org/ledgers/$ledger/requests')

const OPERATIONS: readonly Operation[] = ['transaction', 'revert', 'hold', 'capture', 'void']
const STATUSES: readonly RequestStatus[] = ['pending', 'committed', 'declined', 'failed']

const ENDPOINT: Record<Operation, string> = {
  capture: 'POST holds/{id}/capture',
  hold: 'POST holds',
  revert: 'POST transactions/{id}/revert',
  transaction: 'POST transactions',
  void: 'POST holds/{id}/void'
}

/** Where the record's target lives: transactions and reverts create transactions, the rest act on holds. */
function TargetLink({ ledger, request }: { ledger: string; request: WriteRequest }) {
  const org = useOrganization()
  const isTransaction = request.operation === 'transaction' || request.operation === 'revert'
  const label = shortId(request.targetId)
  if (request.status !== 'committed') {
    return <span className="font-mono text-subtle-foreground">{label}</span>
  }
  return isTransaction ? (
    <Link to="/$org/ledgers/$ledger/transactions/$txId" params={{ ledger, org, txId: request.targetId }} className="font-mono hover:underline">
      {label}
    </Link>
  ) : (
    <Link to="/$org/ledgers/$ledger/holds/$holdId" params={{ holdId: request.targetId, ledger, org }} className="font-mono hover:underline">
      {label}
    </Link>
  )
}

/** Only pending rows tick, so the rest of the table renders once. */
function PendingAge({ createdAt }: { createdAt: string }) {
  const now = useNow(1000)
  return <span className="font-mono text-warning-fg">in flight, {Math.max(0, Math.round((now.getTime() - Date.parse(createdAt)) / 1000))} s</span>
}

function Outcome({ ledger, request }: { ledger: string; request: WriteRequest }) {
  if (request.status === 'pending') {
    return <PendingAge createdAt={request.createdAt} />
  }
  if (request.status === 'committed') {
    return <TargetLink ledger={ledger} request={request} />
  }
  return <span className="font-mono text-danger-fg">{problemReason(request.problemType) ?? request.status}</span>
}

const column = columnHelper<WriteRequest>()

function requestColumns(ledger: string) {
  return [
    column.accessor('key', {
      cell: (info) => (
        <Link
          from="/$org/ledgers/$ledger/requests"
          to="."
          search={(prev) => ({ ...prev, key: info.getValue() })}
          className="block max-w-[260px] truncate font-mono outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
        >
          {info.getValue()}
        </Link>
      ),
      header: 'Idempotency-Key'
    }),
    column.accessor('operation', { cell: (info) => info.getValue(), header: 'Operation' }),
    column.accessor('status', { cell: (info) => <StatusBadge status={info.getValue()} />, header: 'Status' }),
    column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' }),
    column.display({
      cell: ({ row }) => <Outcome ledger={ledger} request={row.original} />,
      header: 'Outcome',
      id: 'outcome',
      meta: { className: 'w-full' }
    }),
    column.accessor('requestId', {
      cell: (info) => <span className="font-mono text-muted-foreground">{info.getValue() ? shortId(info.getValue() ?? '') : ''}</span>,
      header: 'Request ID'
    })
  ]
}

export function RequestsPage() {
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const now = useNow(5000)
  const reconciler = useQuery(systemQueries.reconciler())
  const grace = (reconciler.data?.gracePeriodSeconds ?? 30) * 1000
  const list = useQuery({
    ...requestQueries.list(ledger, {
      actor: search.actor,
      cursor: search.cursor,
      limit: 25,
      operation: search.operation,
      search: search.search,
      status: search.status
    }),
    placeholderData: keepPreviousData
  })
  const columns = useMemo(() => requestColumns(ledger), [ledger])
  const pager = useCursorStack({
    cursor: search.cursor,
    next: list.data?.next,
    onChange: (cursor) => void navigate({ search: (prev) => ({ ...prev, cursor }) }),
    scope: `${search.status ?? ''}|${search.operation ?? ''}|${search.search ?? ''}|${search.actor ?? ''}`
  })
  const setFilter = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch, cursor: undefined }) })

  return (
    <Page>
      <PageHeader title="Write requests" description="Every write and its Idempotency-Key. Declined and in-flight writes only appear here." />
      <FilterBar>
        <SearchFilter
          label="Idempotency-Key or request ID"
          placeholder="Idempotency-Key or request ID"
          value={search.search}
          onChange={(value) => setFilter({ search: value })}
          className="sm:w-72"
        />
        <SelectFilter
          label="Status"
          allLabel="All statuses"
          value={search.status}
          options={STATUSES.map((status) => ({ label: status, value: status }))}
          onChange={(value) => setFilter({ status: STATUSES.find((status) => status === value) })}
        />
        <SelectFilter
          label="Operation"
          allLabel="All operations"
          value={search.operation}
          options={OPERATIONS.map((operation) => ({ label: operation, value: operation }))}
          onChange={(value) => setFilter({ operation: OPERATIONS.find((operation) => operation === value) })}
        />
        {search.actor ? (
          <Button variant="secondary" size="sm" onClick={() => setFilter({ actor: undefined })} aria-label="Show the writes of every actor">
            Actor <span className="font-mono">{search.actor}</span>
            <XIcon />
          </Button>
        ) : null}
      </FilterBar>
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Write requests"
        columns={columns}
        data={list.data?.data}
        getRowId={(request) => request.key}
        loading={list.isPending}
        rowState={(request) =>
          request.key === search.key
            ? 'selected'
            : request.status === 'pending' && now.getTime() - Date.parse(request.createdAt) > grace
              ? 'warning'
              : undefined
        }
        onRowClick={(request) => void navigate({ search: (prev) => ({ ...prev, key: request.key }) })}
        empty="No write matches these filters."
      />
      <CursorPager hasNext={pager.hasNext} hasPrevious={pager.hasPrevious} onNext={pager.next} onPrevious={pager.previous} busy={list.isFetching} />
      <Sheet open={search.key !== undefined} onOpenChange={(open) => !open && void navigate({ search: (prev) => ({ ...prev, key: undefined }) })}>
        <SheetContent className="sm:max-w-[480px]">{search.key ? <RequestDrawer ledger={ledger} requestKey={search.key} /> : null}</SheetContent>
      </Sheet>
    </Page>
  )
}

function RequestDrawer({ ledger, requestKey }: { ledger: string; requestKey: string }) {
  const record = useQuery(requestQueries.detail(ledger, requestKey))
  return (
    <>
      <SheetHeader>
        <SheetTitle>Write request</SheetTitle>
        <SheetDescription className="flex items-center gap-1 font-mono text-caption">
          <span className="truncate">{requestKey}</span>
          <CopyButton value={requestKey} label="Copy Idempotency-Key" />
        </SheetDescription>
      </SheetHeader>
      <SheetBody className="flex flex-col gap-5 *:shrink-0">
        {record.error ? <ProblemPanel error={record.error} onRetry={() => void record.refetch()} /> : null}
        {record.isPending ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="w-1/2" />
            <Skeleton className="w-3/4" />
            <Skeleton className="w-2/3" />
          </div>
        ) : null}
        {record.data ? <RecordDetail ledger={ledger} record={record.data} /> : null}
      </SheetBody>
      <SheetFooter>
        <SheetClose render={<Button variant="secondary" />}>Close</SheetClose>
      </SheetFooter>
    </>
  )
}

function RecordDetail({ ledger, record }: { ledger: string; record: WriteRequestDetail }) {
  const problem = storedProblem(record)
  const duration = record.resolvedAt ? Date.parse(record.resolvedAt) - Date.parse(record.createdAt) : undefined
  const final = record.status === 'declined' || record.status === 'failed'
  return (
    <>
      <div className="flex items-center gap-2 text-caption text-muted-foreground">
        <StatusBadge status={record.status} />
        {record.status === 'pending' ? 'Not settled yet. A retry with this key waits for it.' : 'Stored outcome. Retrying this key replays it.'}
      </div>
      <DescriptionList>
        <DescriptionItem term="Operation" mono>
          {ENDPOINT[record.operation]}
        </DescriptionItem>
        <DescriptionItem term="Created">
          <Timestamp value={record.createdAt} />
        </DescriptionItem>
        {record.resolvedAt ? (
          <DescriptionItem term="Resolved">
            {formatTime(record.resolvedAt)} {duration === undefined ? null : <span className="text-muted-foreground">({duration} ms)</span>}
          </DescriptionItem>
        ) : null}
        <DescriptionItem term="Body hash" mono>
          <span title={record.bodyHash}>sha256:{shortId(record.bodyHash)}</span>
        </DescriptionItem>
        <DescriptionItem term={record.operation === 'transaction' || record.operation === 'revert' ? 'Transaction ID' : 'Hold or transfer ID'}>
          <span className="inline-flex items-center gap-2">
            <TargetLink ledger={ledger} request={record} />
            {final ? <span className="text-caption text-muted-foreground">(burned)</span> : null}
          </span>
        </DescriptionItem>
        <DescriptionItem term="Leg IDs">
          <span className="flex flex-col gap-1">
            <span className="text-muted-foreground">{record.transferIds.length} minted before submit</span>
            {record.transferIds.map((id) => (
              <IdText key={id} value={id} label="Copy transfer ID" />
            ))}
          </span>
        </DescriptionItem>
        <DescriptionItem term="Attempts" mono>
          {record.attempts}
        </DescriptionItem>
        {record.requestId ? (
          <DescriptionItem term="Request ID" mono>
            {record.requestId}
          </DescriptionItem>
        ) : null}
      </DescriptionList>
      {problem ? <ProblemPanel error={problem} /> : null}
      {final ? (
        <Alert>
          <InfoIcon />
          <AlertTitle>Why the key cannot be reused</AlertTitle>
          <AlertDescription>
            The transfer IDs minted for this key are burned in TigerBeetle. Send the corrected request with a new key.
          </AlertDescription>
        </Alert>
      ) : null}
      <CodeBlock value={record.request} label="Stored request" maxHeight={260} />
      {record.response ? <CodeBlock value={record.response} label={`Stored response ${record.responseStatus ?? ''}`} maxHeight={260} /> : null}
    </>
  )
}
