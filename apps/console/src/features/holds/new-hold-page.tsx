import { useState } from 'react'

import { useStore } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'

import { type CreateHoldRequest } from '@tigerbeetle-platform/api-client'
import { ArrowRightIcon } from 'lucide-react'
import { toast } from 'sonner'

import { useAppForm } from '~/components/form/form'
import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { AmountField } from '~/components/ledger/amount-field'
import { IdempotencyKeyField } from '~/components/ledger/idempotency-key-field'
import { MetadataEditor, type MetadataRow, rowsToMetadata, validateMetadataRows } from '~/components/ledger/metadata-editor'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Field, FieldLabel } from '~/components/ui/field'
import { Separator } from '~/components/ui/separator'

import { AccountField } from '~/features/accounts/components/account-field'
import { useAccountStates } from '~/features/accounts/use-account-states'
import { ledgerQueries } from '~/features/ledgers/api'
import { settingsQueries } from '~/features/settings/api'

import { keyIsSpent, useIdempotencyKey } from '~/lib/idempotency'
import { formatAsset, parseAsset, parseMajorUnits, subtractAmounts } from '~/lib/money'
import { formatTime } from '~/lib/time'
import { useNow } from '~/lib/use-now'
import { addressError, amountError, assetError } from '~/lib/validation'

import { useCreateHold } from './api'

// docs/console.md, New hold: one posting that reserves the amount on the source (a pending
// transfer, ADR-0010), plus a timeout on the cluster clock. The preview shows what the source can
// still spend once the hold exists.

const route = getRouteApi('/_console/ledgers/$ledger/holds/new')

/** TigerBeetle stores the timeout in a u32 of seconds. */
const MAX_TIMEOUT = 4_294_967_295

interface HoldDraft {
  source: string
  destination: string
  asset: string
  amount: string
  timeout: string
  reference: string
  eventType: string
  metadata: MetadataRow[]
}

function timeoutError(value: string): string | undefined {
  if (value.trim() === '') {
    return undefined
  }
  if (!/^\d+$/.test(value.trim()) || Number(value) > MAX_TIMEOUT) {
    return 'Whole seconds, from 0 (no timeout) to 4294967295.'
  }
  return undefined
}

function toRequest(draft: HoldDraft): CreateHoldRequest | undefined {
  const asset = parseAsset(draft.asset)
  const parsed = asset ? parseMajorUnits(draft.amount, asset.scale) : undefined
  if (!asset || !parsed?.ok) {
    return undefined
  }
  const body: CreateHoldRequest = { amount: parsed.raw, asset: formatAsset(asset), destination: draft.destination, source: draft.source }
  const timeout = Number(draft.timeout.trim() || '0')
  if (timeout > 0) {
    body.timeoutSeconds = timeout
  }
  if (draft.reference.trim()) {
    body.reference = draft.reference.trim()
  }
  if (draft.eventType && draft.eventType !== 'default') {
    body.eventType = draft.eventType
  }
  const metadata = rowsToMetadata(draft.metadata)
  if (Object.keys(metadata).length > 0) {
    body.metadata = metadata
  }
  return body
}

export function NewHoldPage() {
  const { ledger } = route.useParams()
  const search = route.useSearch()
  const navigate = useNavigate()
  const create = useCreateHold(ledger)
  const idempotency = useIdempotencyKey()
  const [error, setError] = useState<unknown>()
  const assets = useQuery(ledgerQueries.assets(ledger))
  const eventTypes = useQuery(settingsQueries.eventTypes(ledger))
  const now = useNow(30_000)

  const defaultValues: HoldDraft = {
    amount: '',
    asset: search.asset ?? '',
    destination: search.destination ?? '',
    eventType: 'default',
    metadata: [],
    reference: '',
    source: search.source ?? '',
    timeout: '3600'
  }
  const form = useAppForm({
    defaultValues,
    onSubmit: async ({ value }) => {
      const body = toRequest(value)
      if (!body) {
        return
      }
      setError(undefined)
      try {
        const hold = await create.mutateAsync({ body, idempotencyKey: idempotency.keyFor(body) })
        toast.success('Hold created', { description: `${hold.source} reserves the amount until capture, void or expiry.` })
        await navigate({ params: { holdId: hold.id, ledger }, to: '/ledgers/$ledger/holds/$holdId' })
      } catch (error_) {
        setError(error_)
        if (keyIsSpent(error_)) {
          idempotency.renew()
        }
      }
    }
  })
  const draft = useStore(form.store, (state) => state.values)
  const states = useAccountStates(ledger, [draft.source])
  const body = toRequest(draft)
  const canonicalAsset = body?.asset ?? draft.asset
  const sourceState = draft.source && canonicalAsset ? states.stateOf(draft.source, canonicalAsset) : undefined
  const available = sourceState ? subtractAmounts(sourceState.balance, sourceState.reserved) : undefined
  const after = available !== undefined && body ? subtractAmounts(available, body.amount) : undefined
  const limited = sourceState?.limit === 'debits_must_not_exceed_credits'
  const timeoutSeconds = Number(draft.timeout.trim() || '0')
  const expiry = timeoutError(draft.timeout) === undefined && timeoutSeconds > 0 ? new Date(now.getTime() + timeoutSeconds * 1000) : undefined
  const eventOptions = (eventTypes.data?.data ?? []).map((type) => ({ label: `${type.name} (code ${type.code})`, value: type.name }))

  return (
    <Page>
      <PageHeader
        title="New hold"
        description="Reserve funds on the source now; capture them later, release them with a void, or let the hold expire."
      />
      <form
        noValidate
        className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]"
        onSubmit={(event) => {
          event.preventDefault()
          event.stopPropagation()
          void form.handleSubmit()
        }}
      >
        <Card className="self-start">
          <CardHeader>
            <CardTitle className="text-card-title">Hold</CardTitle>
            <CardDescription className="text-small">
              A pending transfer: the amount leaves the source's available balance now and reaches the destination on capture.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5 p-5">
            <div className="grid gap-4 md:grid-cols-[1fr_1fr_minmax(0,1.1fr)]">
              <form.AppField name="source" validators={{ onChange: ({ value }) => addressError(value, 'source') }}>
                {() => <AccountField ledger={ledger} label="Source" asset={canonicalAsset} description="Reserved now." />}
              </form.AppField>
              <form.AppField
                name="destination"
                validators={{
                  onChange: ({ value, fieldApi }) =>
                    addressError(value, 'destination') ??
                    (value === fieldApi.form.getFieldValue('source') ? 'Pick another account than the source.' : undefined),
                  onChangeListenTo: ['source']
                }}
              >
                {() => <AccountField ledger={ledger} label="Destination" asset={canonicalAsset} description="Credited on capture." />}
              </form.AppField>
              <form.Field name="asset" validators={{ onChange: ({ value }) => assetError(value) }}>
                {(assetField) => (
                  <form.AppField
                    name="amount"
                    validators={{
                      onChange: ({ value, fieldApi }) => amountError(value, fieldApi.form.getFieldValue('asset')),
                      onChangeListenTo: ['asset']
                    }}
                  >
                    {() => (
                      <AmountField
                        asset={assetField.state.value}
                        onAssetChange={(asset) => assetField.handleChange(asset)}
                        assets={assets.data?.data.map((asset) => asset.asset) ?? []}
                        assetInvalid={assetField.state.meta.isTouched && assetField.state.meta.errors.length > 0}
                      />
                    )}
                  </form.AppField>
                )}
              </form.Field>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <form.AppField name="timeout" validators={{ onChange: ({ value }) => timeoutError(value) }}>
                {(field) => (
                  <field.TextField
                    label="Timeout"
                    hint="seconds"
                    mono
                    inputMode="numeric"
                    description={
                      expiry ? `Expires about ${formatTime(expiry)}, cluster clock.` : 'No timeout: the hold stays open until captured or voided.'
                    }
                  />
                )}
              </form.AppField>
              <form.AppField name="reference">
                {(field) => <field.TextField label="Reference" hint="Optional" mono maxLength={255} spellCheck={false} />}
              </form.AppField>
              <form.AppField name="eventType">{(field) => <field.SelectField label="Event type" options={eventOptions} mono />}</form.AppField>
            </div>
            <Separator />
            <form.Field name="metadata" validators={{ onChange: ({ value }) => validateMetadataRows(value) }}>
              {(field) => (
                <Field>
                  <FieldLabel>Metadata</FieldLabel>
                  <MetadataEditor
                    rows={field.state.value}
                    onChange={(rows) => field.handleChange(rows)}
                    error={field.state.meta.errors[0]}
                    label="Hold metadata"
                  />
                </Field>
              )}
            </form.Field>
            <IdempotencyKeyField value={idempotency.key} />
            {error ? <ProblemPanel error={error} /> : null}
            <div className="flex justify-end gap-2">
              <Button variant="ghost" render={<Link to="/ledgers/$ledger/holds" params={{ ledger }} />}>
                Cancel
              </Button>
              <form.AppForm>
                <form.SubmitButton>Create hold</form.SubmitButton>
              </form.AppForm>
            </div>
          </CardContent>
        </Card>
        <aside aria-label="Preview" className="flex flex-col gap-4 self-start rounded-lg border border-border bg-surface p-4">
          <h2 className="text-small font-semibold">Preview</h2>
          {draft.source && draft.destination ? (
            <span className="flex flex-wrap items-center gap-2">
              <Address value={draft.source} />
              <ArrowRightIcon aria-label="to" className="size-3.5 text-subtle-foreground" />
              <Address value={draft.destination} />
            </span>
          ) : (
            <p className="text-small text-muted-foreground">Pick the source and destination.</p>
          )}
          {available !== undefined && canonicalAsset ? (
            <dl className="grid grid-cols-[1fr_auto] gap-x-3 gap-y-2 text-small">
              <dt className="text-muted-foreground">Available now</dt>
              <dd>
                <Amount value={available} asset={canonicalAsset} />
              </dd>
              {body ? (
                <>
                  <dt className="text-muted-foreground">Reserved by this hold</dt>
                  <dd>
                    <Amount value={body.amount} asset={canonicalAsset} />
                  </dd>
                </>
              ) : null}
              {after !== undefined ? (
                <>
                  <dt className="text-muted-foreground">Available after</dt>
                  <dd>
                    <Amount value={after} asset={canonicalAsset} className={limited && after.startsWith('-') ? 'text-danger-fg' : undefined} />
                  </dd>
                </>
              ) : null}
            </dl>
          ) : null}
          {limited && after?.startsWith('-') ? (
            <p className="text-caption text-danger-fg">The source cannot go negative: TigerBeetle would decline this hold.</p>
          ) : (
            <p className="text-caption text-muted-foreground">Advisory: balances can move before the hold reaches TigerBeetle.</p>
          )}
        </aside>
      </form>
    </Page>
  )
}
