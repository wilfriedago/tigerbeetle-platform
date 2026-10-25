import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'

import { type AssetSummary } from '@tigerbeetle-platform/api-client'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, PageHeader } from '~/components/layout/page'
import { Amount } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'

import { ledgerQueries } from '~/features/ledgers/api'

// docs/console.md, Assets: read-only. An asset is CODE/SCALE and gets its own TigerBeetle ledger
// number in this ledger the first time a write uses it (ADR-0005).

const route = getRouteApi('/_console/ledgers/$ledger/assets')

const column = columnHelper<AssetSummary>()

const columns = [
  column.accessor('asset', { cell: (info) => <AssetBadge asset={info.getValue()} />, header: 'Asset' }),
  column.accessor('tbLedger', {
    cell: (info) => <span className="font-mono">{info.getValue()}</span>,
    header: 'TigerBeetle ledger',
    meta: { align: 'right' }
  }),
  column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'First used' }),
  column.accessor('accounts', {
    cell: (info) => <span className="font-mono tabular-nums">{info.getValue().toLocaleString('en-US')}</span>,
    header: 'Accounts',
    meta: { align: 'right' }
  }),
  column.accessor('issued', {
    cell: ({ row }) => <Amount value={row.original.issued} asset={row.original.asset} />,
    header: 'Money issued',
    meta: { align: 'right', className: 'w-full' }
  })
]

export function AssetsPage() {
  const { ledger } = route.useParams()
  const assets = useQuery(ledgerQueries.assets(ledger))
  return (
    <Page>
      <PageHeader title="Assets" description="Every asset used in this ledger, with the TigerBeetle ledger number that keeps its accounts apart." />
      {assets.error ? <ProblemPanel error={assets.error} onRetry={() => void assets.refetch()} /> : null}
      <DataTable
        label="Assets"
        columns={columns}
        data={assets.data?.data}
        getRowId={(asset) => asset.asset}
        loading={assets.isPending}
        empty="No assets yet. An asset is registered by the first write that uses it."
      />
      <Hint>A scale never changes: EUR/2 and EUR/4 are two assets with their own accounts. Money issued is minus the balance of world.</Hint>
    </Page>
  )
}
