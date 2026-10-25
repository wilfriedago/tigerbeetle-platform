import { useNavigate } from '@tanstack/react-router'

import { z } from 'zod'

import { useAppForm } from '~/components/form/form'
import { MetadataEditor, type MetadataRow, rowsToMetadata, validateMetadataRows } from '~/components/ledger/metadata-editor'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldLabel } from '~/components/ui/field'

import { setLastLedger } from '~/lib/last-ledger'

import { useCreateLedger } from './api'

/** Mirrors ValidateLedgerName and the ledgers_name_format CHECK constraint. */
export const LEDGER_NAME_PATTERN = /^[a-z0-9][a-z0-9_-]{0,62}$/

/** "EU Refunds" becomes "eu-refunds", the suggestion shown next to an invalid name. */
export function suggestLedgerName(input: string): string {
  return input
    .trim()
    .toLowerCase()
    .replaceAll(/[^a-z0-9_-]+/g, '-')
    .replaceAll(/^[-_]+|-+$/g, '')
    .slice(0, 63)
}

const nameSchema = z.string().superRefine((value, ctx) => {
  if (value.length === 0) {
    ctx.addIssue({ code: 'custom', message: 'Name the ledger.' })
    return
  }
  if (!LEDGER_NAME_PATTERN.test(value)) {
    const suggestion = suggestLedgerName(value)
    ctx.addIssue({
      code: 'custom',
      message: `Use lowercase letters, digits, dash or underscore, 1 to 63 characters.${suggestion && LEDGER_NAME_PATTERN.test(suggestion) ? ` Try ${suggestion}.` : ''}`
    })
  }
})

interface NewLedgerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function NewLedgerDialog({ open, onOpenChange }: NewLedgerDialogProps) {
  const navigate = useNavigate()
  const create = useCreateLedger()
  const form = useAppForm({
    defaultValues: { metadata: [] as MetadataRow[], name: '' },
    onSubmit: async ({ value }) => {
      let ledger
      try {
        ledger = await create.mutateAsync({ body: { metadata: rowsToMetadata(value.metadata) }, name: value.name })
      } catch {
        // The problem panel shows create.error.
        return
      }
      setLastLedger(ledger.name)
      onOpenChange(false)
      await navigate({ params: { ledger: ledger.name }, to: '/ledgers/$ledger' })
    }
  })
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          form.reset()
          create.reset()
        }
      }}
    >
      <DialogContent>
        <form
          noValidate
          onSubmit={(event) => {
            event.preventDefault()
            event.stopPropagation()
            void form.handleSubmit()
          }}
          className="contents"
        >
          <DialogHeader>
            <DialogTitle>New ledger</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <DialogDescription>
              The name is permanent and appears in every API path, for example <span className="font-mono">/v1/ledgers/eu-refunds/transactions</span>.
              Ledgers cannot be deleted: TigerBeetle accounts and transfers are permanent.
            </DialogDescription>
            <form.AppField name="name" validators={{ onChange: nameSchema }}>
              {(field) => <field.TextField label="Name" mono autoComplete="off" spellCheck={false} placeholder="eu-refunds" />}
            </form.AppField>
            <form.Field name="metadata" validators={{ onChange: ({ value }) => validateMetadataRows(value) }}>
              {(field) => (
                <Field>
                  <FieldLabel>Metadata</FieldLabel>
                  <MetadataEditor
                    rows={field.state.value}
                    onChange={(rows) => field.handleChange(rows)}
                    error={field.state.meta.errors[0]}
                    label="Ledger metadata"
                  />
                </Field>
              )}
            </form.Field>
            {create.error ? <ProblemPanel error={create.error} /> : null}
          </DialogBody>
          <DialogFooter>
            <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
            <form.AppForm>
              <form.SubmitButton>Create ledger</form.SubmitButton>
            </form.AppForm>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
