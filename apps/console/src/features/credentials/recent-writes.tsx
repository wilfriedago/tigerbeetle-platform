import { useQueries } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { type WriteRequest } from '@tigerbeetle-platform/api-client'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { SectionTitle } from '~/components/layout/page'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Timestamp } from '~/components/ledger/timestamp'

import { requestQueries } from '~/features/requests/api'

import { useOrganization } from '~/lib/organization'

// .design/auth.pen, "Recent writes" on the API key and machine client pages: the newest write
// requests the credential made, read per ledger it holds a role on with the actor filter of
// listWriteRequests, then merged. A write on a ledger it lost its role on is not listed here; the
// ledger's Requests page still shows it.

const SHOWN = 5

interface Row extends WriteRequest {
  ledger: string
}

const column = columnHelper<Row>()

export function RecentWrites({ actor, ledgers, what }: { actor: string; ledgers: readonly string[]; what: string }) {
  const org = useOrganization()
  const results = useQueries({
    queries: ledgers.map((ledger) => requestQueries.list(ledger, { actor, limit: SHOWN }))
  })

  const error = results.find((r) => r.error)?.error
  const pending = results.some((r) => r.isPending)
  const rows: Row[] = results
    .flatMap((r, i) => (r.data?.data ?? []).map((w) => ({ ...w, ledger: ledgers[i] ?? '' })))
    .toSorted((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
    .slice(0, SHOWN)
  const only = ledgers.length === 1 ? ledgers[0] : undefined

  return (
    <section className="flex flex-col gap-3">
      <SectionTitle
        title="Recent writes"
        description={`Write requests this ${what} made. The ${what} is the actor on each one.`}
        actions={
          only ? (
            <Link
              to="/$org/ledgers/$ledger/requests"
              params={{ ledger: only, org }}
              search={{ actor }}
              className="text-small text-primary-text hover:underline"
            >
              All in Requests
            </Link>
          ) : null
        }
      />
      {error ? <ProblemPanel error={error} /> : null}
      <DataTable
        label="Recent writes"
        columns={[
          column.accessor('key', {
            cell: ({ row }) => (
              <Link
                to="/$org/ledgers/$ledger/requests"
                params={{ ledger: row.original.ledger, org }}
                search={{ actor, key: row.original.key }}
                className="block max-w-[260px] truncate font-mono hover:underline"
              >
                {row.original.key}
              </Link>
            ),
            header: 'Idempotency-Key',
            meta: { className: 'w-full' }
          }),
          column.accessor('ledger', {
            cell: (info) => <span className="font-mono text-muted-foreground">{info.getValue()}</span>,
            header: 'Ledger'
          }),
          column.accessor('status', {
            cell: (info) => <StatusBadge status={info.getValue()} />,
            header: 'Outcome'
          }),
          column.accessor('createdAt', {
            cell: (info) => <Timestamp value={info.getValue()} relative />,
            header: 'When'
          })
        ]}
        data={rows}
        getRowId={(r) => `${r.ledger}/${r.key}`}
        loading={pending && ledgers.length > 0}
        empty={ledgers.length === 0 ? 'No role, so no write.' : 'No write yet.'}
      />
    </section>
  )
}
