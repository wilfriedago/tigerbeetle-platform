import { useState } from 'react'

import { useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { type Hold } from '@tigerbeetle-platform/api-client'
import { BanIcon, CheckCheckIcon } from 'lucide-react'

import { Page, PageHeader } from '~/components/layout/page'
import { Amount } from '~/components/ledger/amount'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { IdText } from '~/components/ledger/id-text'
import { MetadataTags } from '~/components/ledger/metadata'
import { PostingRow } from '~/components/ledger/posting-chain'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Timeline, TimelineItem } from '~/components/ledger/timeline'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { subtractAmounts } from '~/lib/money'
import { formatRelative, formatTimestamp } from '~/lib/time'
import { useNow } from '~/lib/use-now'

import { holdQueries } from './api'
import { CaptureDialog } from './components/capture-dialog'
import { VoidDialog } from './components/void-dialog'

// docs/console.md, Hold: the pending transfer, a timeline from creation to capture, void or expiry,
// and the transfers behind it (ADR-0010).

const route = getRouteApi('/_console/ledgers/$ledger/holds/$holdId')

function HoldTimeline({ hold, now }: { hold: Hold; now: Date }) {
  const created = (
    <TimelineItem
      tone="warning"
      title="Hold created"
      meta={`${formatTimestamp(hold.createdAt)}  timeout ${hold.timeoutSeconds === 0 ? 'none' : `${hold.timeoutSeconds} s`}`}
      aside={<Amount value={hold.amount} asset={hold.asset} />}
    />
  )
  if (hold.status === 'open') {
    return (
      <Timeline>
        {created}
        <TimelineItem
          tone="muted"
          hollow
          last
          title="Awaiting capture, void or expiry"
          meta={
            hold.expiresAt
              ? `expires ${formatTimestamp(hold.expiresAt)} cluster clock, ${formatRelative(hold.expiresAt, now)}`
              : 'no timeout: open until captured or voided'
          }
        />
      </Timeline>
    )
  }
  const at = hold.resolvedAt ?? hold.expiresAt
  if (hold.status === 'captured') {
    const captured = hold.capturedAmount ?? hold.amount
    const released = subtractAmounts(hold.amount, captured)
    return (
      <Timeline>
        {created}
        <TimelineItem
          tone="accent"
          last
          title={released === '0' ? 'Captured in full' : 'Captured in part, the rest released'}
          meta={at ? formatTimestamp(at) : undefined}
          aside={<Amount value={captured} asset={hold.asset} />}
        />
      </Timeline>
    )
  }
  return (
    <Timeline>
      {created}
      <TimelineItem
        tone="muted"
        last
        title={hold.status === 'voided' ? 'Voided, released to the source' : 'Expired, released by the cluster'}
        meta={at ? formatTimestamp(at) : undefined}
      />
    </Timeline>
  )
}

export function HoldPage() {
  const { ledger, holdId } = route.useParams()
  const { data: hold } = useSuspenseQuery(holdQueries.detail(ledger, holdId))
  const now = useNow(15_000)
  const [capturing, setCapturing] = useState(false)
  const [voiding, setVoiding] = useState(false)
  const open = hold.status === 'open'

  let description = `Created ${formatTimestamp(hold.createdAt)}.`
  if (open && hold.expiresAt) {
    description += ` Expires ${formatRelative(hold.expiresAt, now)} by the cluster clock.`
  } else if (open) {
    description += ' No timeout.'
  }

  return (
    <Page>
      <PageHeader
        title="Hold"
        titleAddon={
          <>
            <IdText value={hold.id} full className="text-body" />
            <StatusBadge status={hold.status} />
          </>
        }
        description={description}
        actions={
          open ? (
            <>
              <Button variant="secondary" onClick={() => setVoiding(true)}>
                <BanIcon />
                Void
              </Button>
              <Button onClick={() => setCapturing(true)}>
                <CheckCheckIcon />
                Capture
              </Button>
            </>
          ) : null
        }
      />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Pending transfer</CardTitle>
              <CardDescription>Reserves the amount on the source now; posts it on capture.</CardDescription>
            </CardHeader>
            <CardContent className="py-2">
              <PostingRow posting={hold} index={0} ledger={ledger} />
            </CardContent>
          </Card>
          <Card>
            <CardContent>
              <HoldTimeline hold={hold} now={now} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>TigerBeetle transfers</CardTitle>
              <CardDescription>The pending transfer and the one that resolved it.</CardDescription>
            </CardHeader>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="Pending transfer">
                  <IdText value={hold.pendingTransferId} full label="Copy transfer ID" />
                </DescriptionItem>
                <DescriptionItem term="Resolving transfer">
                  {hold.resolutionTransferId ? (
                    <IdText value={hold.resolutionTransferId} full label="Copy transfer ID" />
                  ) : (
                    <span className="text-muted-foreground">{hold.status === 'expired' ? 'none: the cluster released it' : 'not yet'}</span>
                  )}
                </DescriptionItem>
                {hold.tbLedger ? (
                  <DescriptionItem term="Ledger number" mono>
                    {hold.asset} is {hold.tbLedger}
                  </DescriptionItem>
                ) : null}
                <DescriptionItem term="TigerBeetle timestamp" mono>
                  {hold.timestamp}
                </DescriptionItem>
              </DescriptionList>
            </CardContent>
          </Card>
        </div>
        <div className="flex flex-col gap-6">
          <Card>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="Event type" termWidth="sm">
                  <span className="font-mono">{hold.eventType.name}</span> <span className="text-muted-foreground">(code {hold.eventType.code})</span>
                </DescriptionItem>
                <DescriptionItem term="Timeout" mono termWidth="sm">
                  {hold.timeoutSeconds === 0 ? 'none' : `${hold.timeoutSeconds} s`}
                </DescriptionItem>
                <DescriptionItem term="Idempotency-Key" mono termWidth="sm">
                  <Link
                    to="/ledgers/$ledger/requests"
                    params={{ ledger }}
                    search={{ key: hold.idempotencyKey }}
                    className="break-all hover:text-primary-text hover:underline"
                  >
                    {hold.idempotencyKey}
                  </Link>
                </DescriptionItem>
                {hold.reference ? (
                  <DescriptionItem term="Reference" mono termWidth="sm">
                    {hold.reference}
                  </DescriptionItem>
                ) : null}
              </DescriptionList>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Metadata</CardTitle>
            </CardHeader>
            <CardContent>
              <MetadataTags metadata={hold.metadata} stacked empty={<p className="text-small text-muted-foreground">No metadata.</p>} />
            </CardContent>
          </Card>
        </div>
      </div>
      <CaptureDialog ledger={ledger} hold={hold} open={capturing} onOpenChange={setCapturing} />
      <VoidDialog ledger={ledger} hold={hold} open={voiding} onOpenChange={setVoiding} />
    </Page>
  )
}
