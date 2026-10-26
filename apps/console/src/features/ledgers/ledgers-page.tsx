import { useState } from 'react'

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Ledger } from '@tigerbeetle-platform/api-client'
import { LibraryIcon, PlusIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { CodeBlock } from '~/components/ledger/code-block'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { Hint } from '~/components/ledger/hint'
import { MetadataTags } from '~/components/ledger/metadata'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

import { canCreateLedgers } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'
import { curlCommand } from '~/lib/curl'
import { useOrganization } from '~/lib/organization'

import { ledgerQueries } from './api'
import { NewLedgerDialog } from './new-ledger-dialog'

const route = getRouteApi('/_console/$org/ledgers/')

const column = columnHelper<Ledger>()

/** A ledger's name, linking to its overview in the current organization. */
function LedgerNameLink({ name }: { name: string }) {
  const org = useOrganization()
  return (
    <Link
      to="/$org/ledgers/$ledger"
      params={{ ledger: name, org }}
      className="font-mono text-foreground outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring"
    >
      {name}
    </Link>
  )
}

const columns = [
  column.accessor('name', {
    cell: (info) => <LedgerNameLink name={info.getValue()} />,
    header: 'Name'
  }),
  column.accessor('assets', {
    cell: (info) => (
      <span className="flex gap-1">
        {info.getValue().map((asset) => (
          <AssetBadge key={asset} asset={asset} />
        ))}
      </span>
    ),
    header: 'Assets'
  }),
  column.accessor('accounts', {
    cell: (info) => <span className="font-mono tabular-nums">{info.getValue().toLocaleString('en-US')}</span>,
    header: 'Accounts',
    meta: { align: 'right' }
  }),
  column.accessor('createdAt', {
    cell: (info) => <Timestamp value={info.getValue()} />,
    header: 'Created'
  }),
  column.accessor('metadata', {
    cell: (info) => <MetadataTags metadata={info.getValue()} limit={3} />,
    header: 'Metadata',
    meta: { className: 'w-full' }
  })
]

export function LedgersPage() {
  const org = useOrganization()
  const { cursor } = route.useSearch()
  const navigate = route.useNavigate()
  const [creating, setCreating] = useState(false)
  const session = useSession()
  const mayCreate = canCreateLedgers(session)
  const example = curlCommand({ body: { metadata: { region: 'eu-west' } }, method: 'POST', organization: org, path: '/v1/ledgers/eu-payments' })
  const { data, error, isFetching, isPending, refetch } = useQuery({ ...ledgerQueries.list(cursor), placeholderData: keepPreviousData })
  const pager = useCursorStack({ cursor, next: data?.next, onChange: (next) => void navigate({ search: { cursor: next } }), scope: 'ledgers' })
  const empty = data && data.data.length === 0 && !cursor
  return (
    <Page>
      <PageHeader
        title="Ledgers"
        description="Each ledger is a namespace with its own accounts, assets and rules. Money never crosses ledgers."
        actions={
          mayCreate ? (
            <Button onClick={() => setCreating(true)}>
              <PlusIcon />
              New ledger
            </Button>
          ) : null
        }
      />
      {error ? <ProblemPanel error={error} onRetry={() => void refetch()} /> : null}
      {empty ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia>
              <LibraryIcon />
            </EmptyMedia>
            <EmptyTitle>No ledgers yet</EmptyTitle>
            <EmptyDescription>
              {mayCreate
                ? 'A ledger holds accounts, assets and rules. Create one here or with the API.'
                : 'A ledger holds accounts, assets and rules. Ask an admin of this organization to create one, or to give you a role on one.'}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent className="w-full max-w-[560px] flex-col items-stretch">
            {mayCreate ? (
              <>
                <Button className="self-center" onClick={() => setCreating(true)}>
                  <PlusIcon />
                  New ledger
                </Button>
                <CodeBlock value={example} label="curl" className="text-left" />
              </>
            ) : null}
          </EmptyContent>
        </Empty>
      ) : (
        <>
          <DataTable
            label="Ledgers"
            columns={columns}
            data={data?.data}
            getRowId={(ledger) => ledger.name}
            loading={isPending}
            onRowClick={(ledger) => void navigate({ params: { ledger: ledger.name, org }, to: '/$org/ledgers/$ledger' })}
          />
          <CursorPager
            info="Sorted by name. 100 per page."
            hasNext={pager.hasNext}
            hasPrevious={pager.hasPrevious}
            onNext={pager.next}
            onPrevious={pager.previous}
            busy={isFetching}
          />
        </>
      )}
      <Hint>Ledgers cannot be deleted: TigerBeetle accounts and transfers are permanent.</Hint>
      <NewLedgerDialog open={creating} onOpenChange={setCreating} />
    </Page>
  )
}
