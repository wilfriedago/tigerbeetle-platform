import { useState } from 'react'

import { CircleCheckIcon, CircleXIcon, InfoIcon, LockIcon, PlusIcon, TriangleAlertIcon } from 'lucide-react'

import { Page, PageHeader, SectionTitle } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount, AmountDisplay } from '~/components/ledger/amount'
import { AssetBadge } from '~/components/ledger/asset-badge'
import { BalanceCard } from '~/components/ledger/balance'
import { CodeBlock } from '~/components/ledger/code-block'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
import { IdText } from '~/components/ledger/id-text'
import { MetadataTags } from '~/components/ledger/metadata'
import { PostingChain } from '~/components/ledger/posting-chain'
import { Stat, StatGroup } from '~/components/ledger/stat'
import { type Status, StatusBadge } from '~/components/ledger/status-badge'
import { Stepper } from '~/components/ledger/stepper'
import { Timeline, TimelineItem } from '~/components/ledger/timeline'
import { BusinessTime, Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Checkbox } from '~/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'
import { Label } from '~/components/ui/label'
import { RadioGroup, RadioGroupItem } from '~/components/ui/radio-group'
import { Skeleton } from '~/components/ui/skeleton'
import { Switch } from '~/components/ui/switch'
import { Tabs, TabsCount, TabsList, TabsTrigger } from '~/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '~/components/ui/toggle-group'

// The Ledger Instrument library (.design/ds.lib.pen) rendered with the console's own components,
// to compare the two side by side. Sample values only; nothing here calls the API.

const SWATCHES: readonly { group: string; tokens: readonly (readonly [label: string, className: string])[] }[] = [
  {
    group: 'Surfaces',
    tokens: [
      ['background', 'bg-background'],
      ['surface', 'bg-surface'],
      ['surface-sunken', 'bg-surface-sunken'],
      ['surface-hover', 'bg-surface-hover'],
      ['surface-selected', 'bg-surface-selected'],
      ['sidebar', 'bg-sidebar']
    ]
  },
  {
    group: 'Text and lines',
    tokens: [
      ['foreground', 'bg-foreground'],
      ['muted-foreground', 'bg-muted-foreground'],
      ['subtle-foreground', 'bg-subtle-foreground'],
      ['border', 'bg-border'],
      ['border-strong', 'bg-border-strong'],
      ['focus-ring', 'bg-focus-ring']
    ]
  },
  {
    group: 'Accent',
    tokens: [
      ['primary', 'bg-primary'],
      ['primary-hover', 'bg-primary-hover'],
      ['primary-subtle', 'bg-primary-subtle'],
      ['primary-text', 'bg-primary-text'],
      ['primary-border', 'bg-primary-border']
    ]
  },
  {
    group: 'Status',
    tokens: [
      ['success', 'bg-success'],
      ['success-bg', 'bg-success-bg'],
      ['warning', 'bg-warning'],
      ['warning-bg', 'bg-warning-bg'],
      ['danger', 'bg-danger'],
      ['danger-bg', 'bg-danger-bg']
    ]
  },
  {
    group: 'Data',
    tokens: [
      ['bar-track', 'bg-bar-track'],
      ['bar-available', 'bg-bar-available'],
      ['bar-reserved', 'bg-bar-reserved']
    ]
  }
]

const TYPE_SCALE: readonly [label: string, className: string, sample: string][] = [
  ['page-title, 24', 'text-page-title', 'Transactions'],
  ['card-title, 16', 'text-card-title', 'Recent transactions'],
  ['body, 14', 'text-body', 'Every committed transaction on eu-payments, newest first.'],
  ['small, 13', 'text-small', 'Source is debited, destination is credited.'],
  ['caption, 12', 'text-caption', 'Kept under their Idempotency-Key'],
  ['mono, 13', 'font-mono text-small', 'users:1234:wallet  0192f3a1b7c94e2a'],
  ['amount display, 30', 'text-amount-display tabular-nums', '4,812,300.00']
]

const STATUSES: readonly Status[] = [
  'committed',
  'reverted',
  'revert',
  'open',
  'captured',
  'voided',
  'expired',
  'pending',
  'declined',
  'failed',
  'up',
  'down',
  'not sent'
]

const CHAIN = [
  { amount: '125000', asset: 'EUR/2', destination: 'users:1234:wallet', source: 'world' },
  { amount: '120000', asset: 'EUR/2', destination: 'merchants:42:main', source: 'users:1234:wallet' },
  { amount: '5000', asset: 'EUR/2', destination: 'fees:card', source: 'users:1234:wallet' }
]

function Section({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-4 border-t border-border pt-6">
      <SectionTitle title={title} {...(description ? { description } : {})} />
      {children}
    </section>
  )
}

export function DesignSystemPage() {
  const [tab, setTab] = useState('postings')
  const [view, setView] = useState(['list'])
  return (
    <Page>
      <PageHeader
        title="Design system"
        description="Ledger Instrument, as the console renders it. Switch the theme and density in Preferences to see every mode."
      />

      <Section title="Colour roles" description="Colour means state, never direction.">
        <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
          {SWATCHES.map((group) => (
            <div key={group.group} className="flex flex-col gap-2">
              <span className="text-caption font-medium text-muted-foreground">{group.group}</span>
              {group.tokens.map(([label, className]) => (
                <div key={label} className="flex items-center gap-3">
                  <span className={`size-8 shrink-0 rounded-md border border-border ${className}`} />
                  <span className="font-mono text-small">{label}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
      </Section>

      <Section title="Type" description="IBM Plex Sans for words, IBM Plex Mono for machine values.">
        <div className="flex flex-col gap-3">
          {TYPE_SCALE.map(([label, className, sample]) => (
            <div key={label} className="grid items-baseline gap-4 sm:grid-cols-[160px_1fr]">
              <span className="font-mono text-caption text-subtle-foreground">{label}</span>
              <span className={className}>{sample}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Buttons" description="One primary action per section; labels are verbs.">
        <div className="flex flex-wrap items-center gap-3">
          <Button>
            <PlusIcon />
            New transaction
          </Button>
          <Button variant="secondary">
            <LockIcon />
            New hold
          </Button>
          <Button variant="ghost">Cancel</Button>
          <Button variant="destructive">Void hold</Button>
          <Button variant="link">View all</Button>
          <Button loading>Committing</Button>
          <Button disabled>Disabled</Button>
          <Button size="sm" variant="secondary">
            Small
          </Button>
          <Button size="lg">Large</Button>
        </div>
      </Section>

      <Section title="Status vocabulary" description="One word, one badge, everywhere.">
        <div className="flex flex-wrap gap-2">
          {STATUSES.map((status) => (
            <StatusBadge key={status} status={status} />
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          {(['neutral', 'muted', 'accent', 'success', 'warning', 'danger'] as const).map((variant) => (
            <Badge key={variant} variant={variant}>
              {variant}
            </Badge>
          ))}
        </div>
      </Section>

      <Section title="Form controls" description="Every field has a visible label; help states the rule the API enforces.">
        <div className="grid max-w-3xl gap-5 md:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="ds-reference">Reference</FieldLabel>
            <Input id="ds-reference" className="font-mono" defaultValue="inv-2026-0043" />
            <FieldDescription>Your own identifier. Searchable in lists.</FieldDescription>
          </Field>
          <Field data-invalid>
            <FieldLabel htmlFor="ds-name">Name</FieldLabel>
            <Input id="ds-name" className="font-mono" defaultValue="EU Refunds" aria-invalid />
            <FieldError>Use lowercase letters, digits, dash or underscore. Try eu-refunds.</FieldError>
          </Field>
          <Field>
            <FieldLabel htmlFor="ds-readonly">Idempotency-Key</FieldLabel>
            <Input id="ds-readonly" className="font-mono" readOnly value="idem_7f3c9a12-4b0e-4c1d-9a55" />
          </Field>
          <div className="flex flex-col gap-3">
            <Label className="font-normal">
              <Checkbox defaultChecked /> Keep balance history
            </Label>
            <RadioGroup defaultValue="full" aria-label="Capture">
              <Label className="font-normal">
                <RadioGroupItem value="full" /> Full amount
              </Label>
              <Label className="font-normal">
                <RadioGroupItem value="partial" /> Partial amount
              </Label>
            </RadioGroup>
            <Label className="font-normal">
              <Switch defaultChecked /> Compact rows
            </Label>
          </div>
        </div>
      </Section>

      <Section title="Feedback">
        <div className="grid gap-3 md:grid-cols-2">
          <Alert>
            <InfoIcon />
            <AlertTitle>Prefix sums are on demand</AlertTitle>
            <AlertDescription>They read every account under the prefix from TigerBeetle.</AlertDescription>
          </Alert>
          <Alert variant="success">
            <CircleCheckIcon />
            <AlertTitle>All systems up</AlertTitle>
            <AlertDescription>Writes and reads are served.</AlertDescription>
          </Alert>
          <Alert variant="warning">
            <TriangleAlertIcon />
            <AlertTitle>Rule changes apply to new accounts only</AlertTitle>
            <AlertDescription>Existing accounts keep their flags.</AlertDescription>
          </Alert>
          <Alert variant="danger">
            <CircleXIcon />
            <AlertTitle>TigerBeetle down</AlertTitle>
            <AlertDescription>Writes answer 503 until it is back.</AlertDescription>
          </Alert>
        </div>
        <Hint>Balances live in TigerBeetle, so lists cannot sort by them.</Hint>
        <div className="flex max-w-md flex-col gap-2">
          <Skeleton className="w-1/2" />
          <Skeleton className="w-3/4" />
        </div>
      </Section>

      <Section title="Money and identity" description="Amounts never use JavaScript numbers; hover shows the raw integer.">
        <div className="flex flex-wrap items-center gap-6">
          <Amount value="125000" asset="EUR/2" />
          <Amount value="-481230000" asset="EUR/2" />
          <Amount value="250000" asset="XOF/0" />
          <AmountDisplay value="481230000" asset="EUR/2" />
          <AssetBadge asset="EUR/2" />
          <AssetBadge asset="XOF/0" />
        </div>
        <div className="flex flex-wrap items-center gap-6">
          <Address value="users:1234:wallet" />
          <Address value="world" />
          <IdText value="0192f3a1b7c94e2a8d5f6c0b1e2d7c2e" />
          <Timestamp value="2026-10-24T14:32:08Z" />
          <BusinessTime value="2026-10-23T00:00:00Z" />
          <MetadataTags metadata={{ order_id: 'ord_8812', region: 'eu-west' }} />
        </div>
      </Section>

      <Section title="Ledger patterns">
        <div className="grid gap-4 xl:grid-cols-2">
          <PostingChain postings={CHAIN} status="committed" summary="3 postings, all applied." />
          <PostingChain
            postings={CHAIN}
            status="declined"
            summary="Nothing applied. Posting 2 failed."
            steps={[
              { note: 'not applied: linked_event_failed', state: 'skipped' },
              { note: 'exceeds_credits: users:1234:wallet would go below zero in EUR/2', state: 'failed' },
              { note: 'not applied: linked_event_failed', state: 'skipped' }
            ]}
          />
          <BalanceCard
            balance={{ asset: 'EUR/2', balance: '340000', credits: '980000', debits: '640000', incoming: '0', reserved: '240000' }}
            holds={2}
          />
          <Card>
            <CardHeader>
              <CardTitle>Hold timeline</CardTitle>
              <CardDescription>Created, then captured, voided or expired.</CardDescription>
            </CardHeader>
            <CardContent>
              <Timeline>
                <TimelineItem
                  tone="warning"
                  title="Hold created"
                  meta="Oct 24, 14:24:51  timeout 3600 s"
                  aside={<Amount value="200000" asset="EUR/2" />}
                />
                <TimelineItem tone="muted" hollow last title="Awaiting capture, void or expiry" meta="expires 15:24:51 cluster clock" />
              </Timeline>
            </CardContent>
          </Card>
        </div>
        <StatGroup>
          <Stat label="Money issued, EUR/2" value="4,812,300.00" unit="EUR" sub="minus the balance of world" />
          <Stat label="Accounts" value="18,204" sub="3 created today" />
          <Stat label="Open holds" value="37" sub="12,400.00 EUR reserved" />
        </StatGroup>
        <Card>
          <CardContent className="py-1">
            <DescriptionList>
              <DescriptionItem term="Reference" mono>
                inv-2026-0042
              </DescriptionItem>
              <DescriptionItem term="Event type">deposit (code 3)</DescriptionItem>
            </DescriptionList>
          </CardContent>
        </Card>
        <CodeBlock
          value={{ amount: '125000', asset: 'EUR/2', destination: 'users:1234:wallet', source: 'world' }}
          label="POST /v1/ledgers/eu-payments/transactions"
        />
      </Section>

      <Section title="Navigation">
        <Tabs value={tab} onValueChange={(value: string) => setTab(value)}>
          <TabsList>
            <TabsTrigger value="postings">
              Postings <TabsCount>3</TabsCount>
            </TabsTrigger>
            <TabsTrigger value="metadata">Metadata</TabsTrigger>
            <TabsTrigger value="technical">Technical</TabsTrigger>
          </TabsList>
        </Tabs>
        <Stepper steps={['Postings', 'Details', 'Review']} current={1} />
        <ToggleGroup aria-label="View" value={view} onValueChange={setView}>
          <ToggleGroupItem value="list">List</ToggleGroupItem>
          <ToggleGroupItem value="tree">Tree</ToggleGroupItem>
        </ToggleGroup>
        <CursorPager hasNext hasPrevious={false} onNext={() => undefined} onPrevious={() => undefined} />
      </Section>
    </Page>
  )
}
