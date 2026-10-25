import { useMemo, useState } from 'react'

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type AccountSummary, type Segment } from '@tigerbeetle-platform/api-client'
import { ArrowDownToLineIcon, ChevronDownIcon, ChevronRightIcon, FolderIcon, GlobeIcon, ListIcon, NetworkIcon, WalletIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter } from '~/components/data-table/filters'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { Hint } from '~/components/ledger/hint'
import { MetadataTags } from '~/components/ledger/metadata'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Spinner } from '~/components/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '~/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip'

import { cn } from '~/lib/utils'

import { accountQueries } from './api'

// docs/console.md, Accounts: a list with balances from one batched lookup per page, and the chart
// of accounts by segment where a balance is a prefix sum computed on demand. Accounts are created
// by their first transaction (ADR-0006), so the page offers "Fund an account" instead of "Create".

const route = getRouteApi('/_console/ledgers/$ledger/accounts/')

const LIMIT_TEXT = {
  credits_must_not_exceed_debits: 'credits cannot exceed debits',
  debits_must_not_exceed_credits: 'cannot go negative',
  none: 'no limit'
} as const

const column = columnHelper<AccountSummary>()

function accountColumns(ledger: string) {
  return [
    column.accessor('address', { cell: (info) => <Address value={info.getValue()} ledger={ledger} />, header: 'Address' }),
    column.display({
      cell: ({ row }) => (
        <span className="flex flex-col gap-1.5 py-1.5">
          {row.original.balances.map((balance) => (
            <AssetBadge key={balance.asset} asset={balance.asset} />
          ))}
        </span>
      ),
      header: 'Assets',
      id: 'assets'
    }),
    column.display({
      cell: ({ row }) => (
        <span className="flex flex-col items-end gap-1.5 py-1.5">
          {row.original.balances.map((balance) => (
            <Amount key={balance.asset} value={balance.balance} asset={balance.asset} />
          ))}
        </span>
      ),
      header: () => (
        <Tooltip>
          <TooltipTrigger render={<span className="cursor-help underline decoration-dotted underline-offset-4" />}>Balance</TooltipTrigger>
          <TooltipContent>Balances live in TigerBeetle, so the list cannot sort by them.</TooltipContent>
        </Tooltip>
      ),
      id: 'balance',
      meta: { align: 'right' }
    }),
    column.display({
      cell: ({ row }) => (
        <span className="flex flex-col items-end gap-1.5 py-1.5">
          {row.original.balances.map((balance) =>
            balance.reserved === '0' ? (
              <span key={balance.asset} className="font-mono text-small text-subtle-foreground">
                -
              </span>
            ) : (
              <Amount key={balance.asset} value={balance.reserved} asset={balance.asset} />
            )
          )}
        </span>
      ),
      header: 'Reserved',
      id: 'reserved',
      meta: { align: 'right' }
    }),
    column.display({
      cell: ({ row }) => (
        <span className="flex flex-col gap-1.5 py-1.5 text-small text-muted-foreground">
          {row.original.balances.map((balance) => (
            <span key={balance.asset}>{LIMIT_TEXT[balance.limit]}</span>
          ))}
        </span>
      ),
      header: 'Limit',
      id: 'limit'
    }),
    column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' }),
    column.accessor('metadata', {
      cell: (info) => <MetadataTags metadata={info.getValue()} limit={2} />,
      header: 'Metadata',
      meta: { className: 'w-full' }
    })
  ]
}

export function AccountsPage() {
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const view = search.view ?? 'list'
  return (
    <Page>
      <PageHeader
        title="Accounts"
        description="Accounts are created by their first transaction. The tree groups them by address segment."
        actions={
          <Button variant="secondary" render={<Link to="/ledgers/$ledger/transactions/new" params={{ ledger }} search={{ source: 'world' }} />}>
            <ArrowDownToLineIcon />
            Fund an account
          </Button>
        }
      />
      <FilterBar
        aside={
          <ToggleGroup
            aria-label="View"
            value={[view]}
            onValueChange={(next) => {
              const [value] = next
              if (value === 'list' || value === 'tree') {
                void navigate({ search: (prev) => ({ ...prev, cursor: undefined, view: value === 'list' ? undefined : value }) })
              }
            }}
          >
            <ToggleGroupItem value="list" aria-label="List">
              <ListIcon />
              List
            </ToggleGroupItem>
            <ToggleGroupItem value="tree" aria-label="Tree">
              <NetworkIcon />
              Tree
            </ToggleGroupItem>
          </ToggleGroup>
        }
      >
        {view === 'list' ? (
          <>
            <SearchFilter
              label="Address prefix"
              placeholder="Filter by address prefix"
              value={search.prefix}
              onChange={(prefix) => void navigate({ replace: true, search: (prev) => ({ ...prev, cursor: undefined, prefix }) })}
              className="sm:w-72"
            />
            <Hint>
              Whole segments: <span className="font-mono">users:1</span> does not match <span className="font-mono">users:12</span>.
            </Hint>
          </>
        ) : (
          <Hint>Prefix sums are on demand: they read every account under the prefix from TigerBeetle.</Hint>
        )}
      </FilterBar>
      {view === 'list' ? <AccountList ledger={ledger} /> : <AccountTree ledger={ledger} />}
    </Page>
  )
}

function AccountList({ ledger }: { ledger: string }) {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const params = { asset: search.asset, cursor: search.cursor, limit: 25, prefix: search.prefix, search: search.search }
  const list = useQuery({ ...accountQueries.list(ledger, params), placeholderData: keepPreviousData })
  const columns = useMemo(() => accountColumns(ledger), [ledger])
  const pager = useCursorStack({
    cursor: search.cursor,
    next: list.data?.next,
    onChange: (cursor) => void navigate({ search: (prev) => ({ ...prev, cursor }) }),
    scope: `${search.prefix ?? ''}|${search.search ?? ''}|${search.asset ?? ''}`
  })
  return (
    <>
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Accounts"
        columns={columns}
        data={list.data?.data}
        getRowId={(account) => account.address}
        loading={list.isPending}
        onRowClick={(account) => void navigate({ params: { address: account.address, ledger }, to: '/ledgers/$ledger/accounts/$address' })}
        empty={
          search.prefix ? (
            <>
              No account under <span className="font-mono">{search.prefix}</span>.
            </>
          ) : (
            'No accounts yet. The first transaction that names an address creates its account.'
          )
        }
      />
      <CursorPager
        info="Sorted by address. 25 per page."
        hasNext={pager.hasNext}
        hasPrevious={pager.hasPrevious}
        onNext={pager.next}
        onPrevious={pager.previous}
        busy={list.isFetching}
      />
    </>
  )
}

function AccountTree({ ledger }: { ledger: string }) {
  const roots = useQuery(accountQueries.segments(ledger, ''))
  const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set())
  const toggle = (prefix: string) =>
    setOpen((current) => {
      const next = new Set(current)
      if (next.has(prefix)) {
        next.delete(prefix)
      } else {
        next.add(prefix)
      }
      return next
    })
  if (roots.error) {
    return <ProblemPanel error={roots.error} onRetry={() => void roots.refetch()} />
  }
  return (
    <Table aria-label="Chart of accounts">
      <TableHeader>
        <TableRow>
          <TableHead className="w-full">Address</TableHead>
          <TableHead align="right">Accounts</TableHead>
          <TableHead>Assets</TableHead>
          <TableHead align="right">Balance</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {roots.isPending ? (
          <TableRow>
            <TableCell colSpan={4} className="text-muted-foreground">
              <Spinner /> Reading the chart of accounts
            </TableCell>
          </TableRow>
        ) : (
          (roots.data?.data ?? []).map((segment) => (
            <TreeRows key={segment.prefix} ledger={ledger} segment={segment} depth={0} open={open} onToggle={toggle} />
          ))
        )}
      </TableBody>
    </Table>
  )
}

interface TreeRowsProps {
  ledger: string
  segment: Segment
  depth: number
  open: ReadonlySet<string>
  onToggle: (prefix: string) => void
}

function TreeRows({ ledger, segment, depth, open, onToggle }: TreeRowsProps) {
  const expanded = open.has(segment.prefix)
  const children = useQuery({ ...accountQueries.segments(ledger, segment.prefix), enabled: expanded && segment.hasChildren })
  const isWorld = segment.prefix === 'world'
  const Icon = isWorld ? GlobeIcon : segment.hasChildren ? FolderIcon : WalletIcon
  return (
    <>
      <TableRow>
        <TableCell>
          <span className="flex items-center gap-1.5" style={{ paddingLeft: depth * 22 }}>
            {segment.hasChildren ? (
              <Button
                variant="ghost"
                size="icon-xs"
                aria-expanded={expanded}
                aria-label={`${expanded ? 'Collapse' : 'Expand'} ${segment.prefix}`}
                onClick={() => onToggle(segment.prefix)}
              >
                {expanded ? <ChevronDownIcon /> : <ChevronRightIcon />}
              </Button>
            ) : (
              <span className="w-6" />
            )}
            <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
            {segment.isAccount ? (
              <Link
                to="/ledgers/$ledger/accounts/$address"
                params={{ address: segment.prefix, ledger }}
                className={cn('font-mono text-small hover:underline', segment.hasChildren && 'font-medium')}
              >
                {segment.segment}
              </Link>
            ) : (
              <span className="font-mono text-small font-medium">{segment.segment}</span>
            )}
            {segment.hasChildren ? <span className="font-mono text-caption text-subtle-foreground">{segment.prefix}:</span> : null}
          </span>
        </TableCell>
        <TableCell align="right" className="font-mono tabular-nums">
          {segment.hasChildren ? segment.accounts.toLocaleString('en-US') : null}
        </TableCell>
        <TableCell>
          <span className="flex gap-1">
            {segment.assets.map((asset) => (
              <AssetBadge key={asset} asset={asset} />
            ))}
          </span>
        </TableCell>
        <TableCell align="right">
          {segment.hasChildren ? (
            <PrefixSum ledger={ledger} prefix={segment.prefix} accounts={segment.accounts} />
          ) : (
            <LeafBalance ledger={ledger} address={segment.prefix} />
          )}
        </TableCell>
      </TableRow>
      {expanded
        ? (children.data?.data ?? []).map((child) => (
            <TreeRows key={child.prefix} ledger={ledger} segment={child} depth={depth + 1} open={open} onToggle={onToggle} />
          ))
        : null}
      {expanded && children.isPending ? (
        <TableRow>
          <TableCell colSpan={4}>
            <span className="flex items-center gap-2 text-small text-muted-foreground" style={{ paddingLeft: (depth + 1) * 22 + 30 }}>
              <Spinner /> Loading
            </span>
          </TableCell>
        </TableRow>
      ) : null}
    </>
  )
}

/** A leaf is one address: its balance per asset comes from one account lookup. */
function LeafBalance({ ledger, address }: { ledger: string; address: string }) {
  const account = useQuery(accountQueries.detail(ledger, address))
  if (!account.data) {
    return account.isPending ? <Spinner /> : null
  }
  return (
    <span className="flex flex-col items-end gap-1">
      {account.data.balances.map((balance) => (
        <Amount key={balance.asset} value={balance.balance} asset={balance.asset} />
      ))}
    </span>
  )
}

/** A folder's balance is a prefix sum: computed when asked, because it reads every account below. */
function PrefixSum({ ledger, prefix, accounts }: { ledger: string; prefix: string; accounts: number }) {
  const [asked, setAsked] = useState(false)
  const sum = useQuery({ ...accountQueries.aggregate(ledger, prefix), enabled: asked })
  if (!asked) {
    return (
      <Button variant="link" size="sm" onClick={() => setAsked(true)}>
        Sum balances
      </Button>
    )
  }
  if (sum.isPending) {
    return (
      <span className="inline-flex items-center gap-2 text-caption text-muted-foreground">
        <Spinner /> Summing {accounts.toLocaleString('en-US')} {accounts === 1 ? 'account' : 'accounts'}
      </span>
    )
  }
  if (sum.error) {
    return <span className="text-caption text-danger-fg">Could not sum: narrow the prefix.</span>
  }
  return (
    <Link
      to="/ledgers/$ledger/balances"
      params={{ ledger }}
      search={{ prefix }}
      className="flex flex-col items-end gap-1 hover:underline"
      title={`Open the sum under ${prefix}:`}
    >
      {sum.data?.assets.map((asset) => (
        <Amount key={asset.asset} value={asset.balance} asset={asset.asset} />
      ))}
    </Link>
  )
}
