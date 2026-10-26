import { useState } from 'react'

import { useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'

import { toast } from 'sonner'

import { useAppForm } from '~/components/form/form'
import { Page, PageHeader } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
import { MetadataEditor, metadataToRows, rowsToMetadata, validateMetadataRows } from '~/components/ledger/metadata-editor'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { ledgerQueries, useReplaceLedgerMetadata } from '~/features/ledgers/api'

// docs/console.md, Ledger settings: the name is permanent (it is in every API path), the metadata
// is the only thing that changes, and there is no delete because TigerBeetle keeps every account.

const route = getRouteApi('/_console/$org/ledgers/$ledger/settings/')

export function LedgerSettingsPage() {
  const { ledger } = route.useParams()
  const { data } = useSuspenseQuery(ledgerQueries.detail(ledger))
  return (
    <Page className="max-w-[880px]">
      <PageHeader title="Ledger settings" description="The name and creation date are permanent. Metadata is yours to change." />
      <Card>
        <CardContent className="py-1">
          <DescriptionList>
            <DescriptionItem term="Name" mono>
              {data.name}
            </DescriptionItem>
            <DescriptionItem term="Created">
              <Timestamp value={data.createdAt} />
            </DescriptionItem>
            <DescriptionItem term="Assets" mono>
              {data.assets.length > 0 ? data.assets.join(', ') : <span className="font-sans text-muted-foreground">none yet</span>}
            </DescriptionItem>
            <DescriptionItem term="Accounts" mono>
              {data.accounts.toLocaleString('en-US')}
            </DescriptionItem>
          </DescriptionList>
        </CardContent>
      </Card>
      {/* Keyed by the saved map, so the form starts over after a save or a change made elsewhere. */}
      <MetadataCard key={JSON.stringify(data.metadata)} ledger={ledger} metadata={data.metadata} />
      <Hint>Ledgers cannot be deleted or renamed: TigerBeetle accounts and transfers are permanent, and the name is part of every API path.</Hint>
    </Page>
  )
}

function MetadataCard({ ledger, metadata }: { ledger: string; metadata: Record<string, string> }) {
  const replace = useReplaceLedgerMetadata(ledger)
  const [error, setError] = useState<unknown>()
  const form = useAppForm({
    defaultValues: { rows: metadataToRows(metadata) },
    onSubmit: async ({ value }) => {
      setError(undefined)
      try {
        await replace.mutateAsync(rowsToMetadata(value.rows))
        toast.success('Metadata saved')
      } catch (error_) {
        setError(error_)
      }
    }
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>Metadata</CardTitle>
        <CardDescription>Key and value pairs for your own systems, stored in Postgres. Saving replaces the whole map.</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            event.stopPropagation()
            void form.handleSubmit()
          }}
        >
          <form.Field name="rows" validators={{ onChange: ({ value }) => validateMetadataRows(value) }}>
            {(field) => (
              <MetadataEditor
                rows={field.state.value}
                onChange={(rows) => field.handleChange(rows)}
                error={field.state.meta.errors[0]}
                label="Ledger metadata"
              />
            )}
          </form.Field>
          {error ? <ProblemPanel error={error} /> : null}
          <div className="flex justify-end">
            <form.Subscribe selector={(state) => state.isDirty}>
              {(dirty) =>
                dirty ? (
                  <form.AppForm>
                    <form.SubmitButton>Save metadata</form.SubmitButton>
                  </form.AppForm>
                ) : (
                  <span className="text-caption text-muted-foreground">No unsaved changes.</span>
                )
              }
            </form.Subscribe>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
