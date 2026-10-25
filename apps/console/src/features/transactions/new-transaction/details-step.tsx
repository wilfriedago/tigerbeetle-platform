import { type EventType } from '@tigerbeetle-platform/api-client'

import { withForm } from '~/components/form/form'
import { MetadataEditor, validateMetadataRows } from '~/components/ledger/metadata-editor'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Separator } from '~/components/ui/separator'

import { transactionFormOptions } from './form-options'

export const DetailsStep = withForm({
  ...transactionFormOptions,
  props: { eventTypes: [] as readonly EventType[] },
  render({ form, eventTypes }) {
    const options = eventTypes.map((type) => ({ label: `${type.name} (code ${type.code})`, value: type.name }))
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-card-title">Details</CardTitle>
          <CardDescription className="text-small">
            How this transaction is identified and described. Stored in Postgres with it; TigerBeetle only keeps the event code.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-5 p-5">
          <div className="grid gap-5 md:grid-cols-2">
            <form.AppField name="reference" validators={{ onChange: ({ value }) => (value.length > 255 ? 'At most 255 characters.' : undefined) }}>
              {(field) => (
                <field.TextField
                  label="Reference"
                  hint="Optional"
                  mono
                  maxLength={255}
                  spellCheck={false}
                  description="Your own identifier. Searchable in lists."
                />
              )}
            </form.AppField>
            <form.AppField name="eventType">
              {(field) => (
                <field.SelectField
                  label="Event type"
                  options={options}
                  mono
                  description="Written to code on every posting. Codes never change meaning."
                />
              )}
            </form.AppField>
            <form.AppField name="businessTime">
              {(field) => (
                <field.TextField
                  label="Business time"
                  hint="Optional"
                  type="date"
                  mono
                  description="Your date for the event. The commit time always comes from TigerBeetle."
                />
              )}
            </form.AppField>
          </div>
          <Separator />
          <form.Field name="metadata" validators={{ onChange: ({ value }) => validateMetadataRows(value) }}>
            {(field) => (
              <Field>
                <FieldLabel>Metadata</FieldLabel>
                <FieldDescription className="-mt-1">Key and value pairs for your own systems.</FieldDescription>
                <MetadataEditor
                  rows={field.state.value}
                  onChange={(rows) => field.handleChange(rows)}
                  error={field.state.meta.errors[0]}
                  label="Transaction metadata"
                />
              </Field>
            )}
          </form.Field>
        </CardContent>
      </Card>
    )
  }
})
