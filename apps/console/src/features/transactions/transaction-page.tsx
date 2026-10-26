import { useState } from 'react'

import { useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Transaction } from '@tigerbeetle-platform/api-client'
import { EllipsisIcon, InboxIcon, PencilIcon, RotateCcwIcon, TerminalIcon } from 'lucide-react'

import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { CodeBlock } from '~/components/ledger/code-block'
import { useCopy } from '~/components/ledger/copy-button'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { IdText } from '~/components/ledger/id-text'
import { MetadataTags } from '~/components/ledger/metadata'
import { MetadataDialog } from '~/components/ledger/metadata-dialog'
import { PostingChain } from '~/components/ledger/posting-chain'
import { StatusBadge } from '~/components/ledger/status-badge'
import { BusinessTime, Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLinkItem, DropdownMenuTrigger } from '~/components/ui/dropdown-menu'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'
import { Tabs, TabsContent, TabsCount, TabsList, TabsTrigger } from '~/components/ui/tabs'

import { apiPath, curlCommand } from '~/lib/curl'
import { netEffect } from '~/lib/net-effect'
import { useOrganization } from '~/lib/organization'

import { transactionQueries, useReplaceTransactionMetadata } from './api'
import { RevertDialog } from './components/revert-dialog'
import { requestBodyOf } from './request-body'

const route = getRouteApi('/_console/$org/ledgers/$ledger/transactions/$txId')

type Tab = 'postings' | 'metadata' | 'technical'

export function TransactionPage() {
  const org = useOrganization()
  const { ledger, txId } = route.useParams()
  const { tab = 'postings' } = route.useSearch()
  const navigate = route.useNavigate()
  const { data: tx } = useSuspenseQuery(transactionQueries.detail(ledger, txId))
  const [reverting, setReverting] = useState(false)
  const [editing, setEditing] = useState(false)
  const replaceMetadata = useReplaceTransactionMetadata(ledger, txId)
  const [copied, copy] = useCopy()
  const body = requestBodyOf(tx)
  const curl = curlCommand({ body, idempotencyKey: tx.idempotencyKey, method: 'POST', path: apiPath('ledgers', ledger, 'transactions') })

  return (
    <Page>
      <PageHeader
        title="Transaction"
        titleAddon={
          <>
            <IdText value={tx.id} full className="text-body" />
            {tx.revertedBy ? <StatusBadge status="reverted" /> : <StatusBadge status="committed" />}
            {tx.reverts ? <StatusBadge status="revert" /> : null}
          </>
        }
        description={
          <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <span className="flex items-center gap-2">
              Committed <Timestamp value={tx.committedAt} relative />
            </span>
            {tx.businessTime ? <BusinessTime value={tx.businessTime} /> : null}
          </span>
        }
        actions={
          <>
            <Button variant="secondary" onClick={() => copy(curl)}>
              <TerminalIcon />
              {copied ? 'Copied' : 'Copy as curl'}
            </Button>
            <Button variant="secondary" disabled={Boolean(tx.revertedBy)} onClick={() => setReverting(true)}>
              <RotateCcwIcon />
              Revert
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="secondary" size="icon" aria-label="More actions" />}>
                <EllipsisIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => setEditing(true)}>
                  <PencilIcon />
                  Edit metadata
                </DropdownMenuItem>
                <DropdownMenuLinkItem
                  render={<Link to="/$org/ledgers/$ledger/requests" params={{ ledger, org }} search={{ key: tx.idempotencyKey }} />}
                >
                  <InboxIcon />
                  Open the write request
                </DropdownMenuLinkItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        }
      />

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <Tabs value={tab} onValueChange={(value: Tab) => void navigate({ replace: true, search: { tab: value === 'postings' ? undefined : value } })}>
          <TabsList>
            <TabsTrigger value="postings">
              Postings <TabsCount>{tx.postings.length}</TabsCount>
            </TabsTrigger>
            <TabsTrigger value="metadata">Metadata</TabsTrigger>
            <TabsTrigger value="technical">Technical</TabsTrigger>
          </TabsList>
          <TabsContent value="postings" className="flex flex-col gap-6">
            <PostingChain
              postings={tx.postings}
              status="committed"
              ledger={ledger}
              summary={`${tx.postings.length} ${tx.postings.length === 1 ? 'posting' : 'postings'}, all applied.`}
            />
            <NetEffectCard transaction={tx} ledger={ledger} />
            <CodeBlock value={body} label={`POST ${apiPath('ledgers', ledger, 'transactions')}`} />
          </TabsContent>
          <TabsContent value="metadata">
            <Card>
              <CardHeader>
                <CardTitle>Metadata</CardTitle>
                <CardDescription>Stored in Postgres with the transaction. Replacing it never touches the money.</CardDescription>
                <CardAction>
                  <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
                    <PencilIcon />
                    Edit
                  </Button>
                </CardAction>
              </CardHeader>
              <CardContent>
                <MetadataTags metadata={tx.metadata} stacked empty={<p className="text-small text-muted-foreground">No metadata.</p>} />
              </CardContent>
            </Card>
          </TabsContent>
          <TabsContent value="technical" className="flex flex-col gap-6">
            <TransfersCard transaction={tx} />
            <CodeBlock value={tx} label="Transaction, as the API returns it" />
          </TabsContent>
        </Tabs>

        <div className="flex flex-col gap-6">
          <Card>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="Reference" mono termWidth="sm">
                  {tx.reference ?? <span className="font-sans text-subtle-foreground">none</span>}
                </DescriptionItem>
                <DescriptionItem term="Event type" termWidth="sm">
                  <Link
                    to="/$org/ledgers/$ledger/transactions"
                    params={{ ledger, org }}
                    search={{ eventType: tx.eventType.name }}
                    className="font-mono hover:text-primary-text hover:underline"
                  >
                    {tx.eventType.name}
                  </Link>{' '}
                  <span className="text-muted-foreground">(code {tx.eventType.code})</span>
                </DescriptionItem>
                <DescriptionItem term="Idempotency-Key" mono termWidth="sm">
                  <Link
                    to="/$org/ledgers/$ledger/requests"
                    params={{ ledger, org }}
                    search={{ key: tx.idempotencyKey }}
                    className="break-all hover:text-primary-text hover:underline"
                  >
                    {tx.idempotencyKey}
                  </Link>
                </DescriptionItem>
                {tx.requestId ? (
                  <DescriptionItem term="Request ID" mono termWidth="sm">
                    {tx.requestId}
                  </DescriptionItem>
                ) : null}
                <DescriptionItem term="Ledger number" mono termWidth="sm">
                  {[...new Map(tx.postings.map((posting) => [posting.asset, posting.tbLedger])).entries()].map(([asset, number]) => (
                    <div key={asset}>
                      {asset} is {number}
                    </div>
                  ))}
                </DescriptionItem>
              </DescriptionList>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Metadata</CardTitle>
              <CardDescription>Stored in Postgres with the transaction.</CardDescription>
              <CardAction>
                <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
                  <PencilIcon />
                  Edit
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              <MetadataTags metadata={tx.metadata} stacked empty={<p className="text-small text-muted-foreground">No metadata.</p>} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Related</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-small text-muted-foreground">
              {tx.reverts ? (
                <p>
                  Reverts{' '}
                  <Link
                    to="/$org/ledgers/$ledger/transactions/$txId"
                    params={{ ledger, org, txId: tx.reverts }}
                    className="font-mono text-primary-text hover:underline"
                  >
                    <IdText value={tx.reverts} copy={false} className="text-primary-text" />
                  </Link>
                  .
                </p>
              ) : null}
              {tx.revertedBy ? (
                <p>
                  Reverted by{' '}
                  <Link
                    to="/$org/ledgers/$ledger/transactions/$txId"
                    params={{ ledger, org, txId: tx.revertedBy }}
                    className="font-mono text-primary-text hover:underline"
                  >
                    <IdText value={tx.revertedBy} copy={false} className="text-primary-text" />
                  </Link>
                  . A transaction is reverted once.
                </p>
              ) : (
                <p>
                  Not reverted. Reverting creates a new transaction with these postings reversed; it can be declined if the destinations have spent
                  the funds.
                </p>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <RevertDialog ledger={ledger} transaction={tx} open={reverting} onOpenChange={setReverting} />
      <MetadataDialog
        open={editing}
        onOpenChange={setEditing}
        title="Edit transaction metadata"
        description="The map replaces the current metadata. Postings and amounts never change."
        metadata={tx.metadata}
        onSave={async (metadata) => replaceMetadata.mutateAsync(metadata)}
      />
    </Page>
  )
}

function NetEffectCard({ transaction, ledger }: { transaction: Transaction; ledger: string }) {
  const rows = netEffect(transaction.postings)
  return (
    <Card>
      <CardHeader>
        <CardTitle>Net effect</CardTitle>
        <CardDescription>Per account and asset. Balance is credits minus debits.</CardDescription>
      </CardHeader>
      <Table framed={false} aria-label="Net effect">
        <TableHeader>
          <TableRow>
            <TableHead>Account</TableHead>
            <TableHead>Asset</TableHead>
            <TableHead align="right">Debited</TableHead>
            <TableHead align="right">Credited</TableHead>
            <TableHead align="right">Balance change</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={`${row.asset}-${row.address}`}>
              <TableCell className="w-full">
                <Address value={row.address} ledger={ledger} />
              </TableCell>
              <TableCell>
                <AssetBadge asset={row.asset} />
              </TableCell>
              <TableCell align="right">
                {row.debited === '0' ? <span className="text-subtle-foreground">-</span> : <Amount value={row.debited} asset={row.asset} />}
              </TableCell>
              <TableCell align="right">
                {row.credited === '0' ? <span className="text-subtle-foreground">-</span> : <Amount value={row.credited} asset={row.asset} />}
              </TableCell>
              <TableCell align="right">
                <Amount value={row.net} asset={row.asset} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  )
}

/** The TigerBeetle transfers behind the transaction: one per posting, linked except the last. */
function TransfersCard({ transaction }: { transaction: Transaction }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>TigerBeetle transfers</CardTitle>
        <CardDescription>One linked chain; user_data_128 carries the transaction ID and code the event type.</CardDescription>
      </CardHeader>
      <Table framed={false} aria-label="TigerBeetle transfers">
        <TableHeader>
          <TableRow>
            <TableHead>#</TableHead>
            <TableHead>Transfer ID</TableHead>
            <TableHead align="right">Ledger</TableHead>
            <TableHead align="right">Code</TableHead>
            <TableHead>Flags</TableHead>
            <TableHead>user_data_128</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {transaction.postings.map((posting, index) => (
            <TableRow key={posting.transferId}>
              <TableCell className="font-mono text-subtle-foreground">{index + 1}</TableCell>
              <TableCell>
                <IdText value={posting.transferId} label="Copy transfer ID" />
              </TableCell>
              <TableCell align="right" className="font-mono">
                {posting.tbLedger}
              </TableCell>
              <TableCell align="right" className="font-mono">
                {transaction.eventType.code}
              </TableCell>
              <TableCell className="font-mono text-small">
                {index < transaction.postings.length - 1 ? 'linked' : <span className="text-subtle-foreground">none</span>}
              </TableCell>
              <TableCell>
                <IdText value={transaction.id} copy={false} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <CardContent className="py-1">
        <DescriptionList>
          <DescriptionItem term="TigerBeetle timestamp" mono>
            {transaction.timestamp}
          </DescriptionItem>
        </DescriptionList>
      </CardContent>
    </Card>
  )
}
