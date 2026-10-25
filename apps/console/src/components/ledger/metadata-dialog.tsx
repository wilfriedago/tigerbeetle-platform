import { useState } from 'react'

import { useAppForm } from '~/components/form/form'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'

import { MetadataEditor, metadataToRows, rowsToMetadata, validateMetadataRows } from './metadata-editor'
import { ProblemPanel } from './problem-panel'

interface MetadataDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: React.ReactNode
  metadata: Record<string, string> | undefined
  /** Replaces the whole map, like the PUT endpoints do. */
  onSave: (metadata: Record<string, string>) => Promise<unknown>
}

/** Edits a metadata map in a dialog; the form mounts with the dialog, so it starts from the saved map. */
export function MetadataDialog({ open, onOpenChange, ...props }: MetadataDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px]">{open ? <MetadataForm {...props} onDone={() => onOpenChange(false)} /> : null}</DialogContent>
    </Dialog>
  )
}

function MetadataForm({ title, description, metadata, onSave, onDone }: Omit<MetadataDialogProps, 'open' | 'onOpenChange'> & { onDone: () => void }) {
  const [error, setError] = useState<unknown>()
  const form = useAppForm({
    defaultValues: { rows: metadataToRows(metadata) },
    onSubmit: async ({ value }) => {
      setError(undefined)
      try {
        await onSave(rowsToMetadata(value.rows))
        onDone()
      } catch (error_) {
        setError(error_)
      }
    }
  })
  return (
    <form
      noValidate
      className="contents"
      onSubmit={(event) => {
        event.preventDefault()
        event.stopPropagation()
        void form.handleSubmit()
      }}
    >
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>{description}</DialogDescription>
        <form.Field name="rows" validators={{ onChange: ({ value }) => validateMetadataRows(value) }}>
          {(field) => <MetadataEditor rows={field.state.value} onChange={(rows) => field.handleChange(rows)} error={field.state.meta.errors[0]} />}
        </form.Field>
        {error ? <ProblemPanel error={error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <form.AppForm>
          <form.SubmitButton>Save metadata</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  )
}
