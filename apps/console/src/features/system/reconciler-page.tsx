import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { type StuckRequest } from '@tigerbeetle-platform/api-client'
import { CircleCheckIcon, TriangleAlertIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, PageHeader } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { useOrganization } from '~/lib/organization'
import { formatRelative } from '~/lib/time'
import { useNow } from '~/lib/use-now'

import { systemQueries } from './api'

// docs/console.md, Reconciler: the process that settles a write the API could not finish
// (ADR-0007). It looks up the minted transfer IDs: all found means committed, none means it can
// submit again with the same IDs, some found and some missing needs a person.

const column = columnHelper<StuckRequest>()

function LedgerLink({ ledger }: { ledger: string }) {
  const org = useOrganization()
  return (
    <Link to="/$org/ledgers/$ledger" params={{ ledger, org }} className="font-mono hover:underline">
      {ledger}
    </Link>
  )
}

function RequestLink({ ledger, requestKey }: { ledger: string; requestKey: string }) {
  const org = useOrganization()
  return (
    <Link to="/$org/ledgers/$ledger/requests" params={{ ledger, org }} search={{ key: requestKey }} className="font-mono hover:underline">
      {requestKey}
    </Link>
  )
}

const columns = [
  column.accessor('ledger', {
    cell: (info) => <LedgerLink ledger={info.getValue()} />,
    header: 'Ledger'
  }),
  column.accessor('key', {
    cell: ({ row }) => <RequestLink ledger={row.original.ledger} requestKey={row.original.key} />,
    header: 'Idempotency-Key'
  }),
  column.accessor('operation', { header: 'Operation' }),
  column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' }),
  column.accessor('attempts', { cell: (info) => <span className="font-mono">{info.getValue()}</span>, header: 'Attempts', meta: { align: 'right' } }),
  column.accessor('note', {
    cell: (info) => <span className="whitespace-normal text-muted-foreground">{info.getValue()}</span>,
    header: 'Note',
    meta: { className: 'w-full' }
  })
]

export function ReconcilerPage() {
  const status = useQuery(systemQueries.reconciler())
  const now = useNow(1000)
  const { data } = status
  return (
    <Page>
      <PageHeader
        title="Reconciler"
        description="Settles write records that stayed pending after a crash or a timeout, so a retry always gets an answer."
      />
      {status.error ? <ProblemPanel error={status.error} onRetry={() => void status.refetch()} /> : null}
      {data && data.stuck > 0 ? (
        <Alert variant="danger">
          <TriangleAlertIcon />
          <AlertTitle>
            {data.stuck} {data.stuck === 1 ? 'write needs' : 'writes need'} a person
          </AlertTitle>
          <AlertDescription>
            Some of their transfers exist in TigerBeetle and some do not. The reconciler will not guess; compare the IDs below with the cluster.
          </AlertDescription>
        </Alert>
      ) : data ? (
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>Nothing is stuck</AlertTitle>
          <AlertDescription>
            {data.pending === 0
              ? 'No write is pending.'
              : `${data.pending} pending ${data.pending === 1 ? 'write is' : 'writes are'} younger than the grace period or being settled.`}
          </AlertDescription>
        </Alert>
      ) : null}
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <Card>
          <CardHeader>
            <CardTitle>How a pending record is settled</CardTitle>
            <CardDescription>ADR-0007. Each run looks at records older than the grace period.</CardDescription>
          </CardHeader>
          <CardContent>
            <ol className="flex list-decimal flex-col gap-2 pl-5 text-small text-muted-foreground">
              <li>Look up every transfer ID minted for the record before its first submission.</li>
              <li>All found: the chain committed. The record becomes committed with the TigerBeetle timestamp.</li>
              <li>None found: the chain never reached the cluster. Submit it again with the same IDs, so it can apply at most once.</li>
              <li>
                Some found and some missing: impossible for a linked chain unless IDs were reused elsewhere. The record is reported here and left
                alone.
              </li>
            </ol>
          </CardContent>
        </Card>
        <Card className="self-start">
          <CardContent className="py-1">
            <DescriptionList>
              <DescriptionItem term="State" termWidth="sm">
                {data ? <StatusBadge status={data.running ? 'up' : 'down'}>{data.running ? 'running' : 'stopped'}</StatusBadge> : '-'}
              </DescriptionItem>
              <DescriptionItem term="Interval" mono termWidth="sm">
                {data ? `${data.intervalSeconds} s` : '-'}
              </DescriptionItem>
              <DescriptionItem term="Grace period" mono termWidth="sm">
                {data ? `${data.gracePeriodSeconds} s` : '-'}
              </DescriptionItem>
              <DescriptionItem term="Last run" termWidth="sm">
                {data?.lastRunAt ? (
                  <>
                    <Timestamp value={data.lastRunAt} /> <span className="text-muted-foreground">({formatRelative(data.lastRunAt, now)})</span>
                  </>
                ) : (
                  'not yet'
                )}
              </DescriptionItem>
              <DescriptionItem term="Settled last run" mono termWidth="sm">
                {data?.lastSettled ?? 0}
              </DescriptionItem>
              <DescriptionItem term="Pending, all ledgers" mono termWidth="sm">
                {data?.pending ?? '-'}
              </DescriptionItem>
            </DescriptionList>
          </CardContent>
        </Card>
      </div>
      <section className="flex flex-col gap-3">
        <h2 className="text-body font-semibold">Stuck writes</h2>
        <DataTable
          label="Stuck writes"
          columns={columns}
          data={data?.stuckRequests ?? []}
          getRowId={(row) => `${row.ledger}/${row.key}`}
          loading={status.isPending}
          empty="None. Every pending record so far was settled."
        />
      </section>
    </Page>
  )
}
