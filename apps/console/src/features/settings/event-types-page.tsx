import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'

import { type EventType } from '@tigerbeetle-platform/api-client'
import { PlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { useAppForm } from '~/components/form/form'
import { Page, PageHeader } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'

import { settingsQueries, useCreateEventType } from './api'

// docs/console.md, Event types: a name for the code written to every transfer (ADR-0007). Add only:
// a code never changes meaning, so there is no edit and no delete. Event types are per ledger.

const route = getRouteApi('/_console/ledgers/$ledger/settings/event-types')

/** Mirrors eventTypeNamePattern in apps/api/internal/kernel/ledger/event.go. */
const NAME_PATTERN = /^[a-z][a-z0-9_.-]{0,62}$/

const column = columnHelper<EventType>()

const columns = [
  column.accessor('name', {
    cell: ({ row }) => (
      <span className="inline-flex items-center gap-2">
        <span className="font-mono">{row.original.name}</span>
        {row.original.builtIn ? <Badge variant="muted">built-in</Badge> : null}
      </span>
    ),
    header: 'Name',
    meta: { className: 'w-full' }
  }),
  column.accessor('code', { cell: (info) => <span className="font-mono">{info.getValue()}</span>, header: 'Code', meta: { align: 'right' } }),
  column.accessor('transactions', {
    cell: (info) => <span className="font-mono tabular-nums">{info.getValue().toLocaleString('en-US')}</span>,
    header: 'Used by',
    meta: { align: 'right' }
  }),
  column.accessor('createdAt', { cell: (info) => (info.getValue() ? <Timestamp value={info.getValue() ?? ''} /> : null), header: 'Created' })
]

export function EventTypesPage() {
  const { ledger } = route.useParams()
  const types = useQuery(settingsQueries.eventTypes(ledger))
  const [creating, setCreating] = useState(false)
  return (
    <Page>
      <PageHeader
        title="Event types"
        description="The business event behind each transaction, written to the code field of every transfer."
        actions={
          <Button onClick={() => setCreating(true)}>
            <PlusIcon />
            New event type
          </Button>
        }
      />
      {types.error ? <ProblemPanel error={types.error} onRetry={() => void types.refetch()} /> : null}
      <DataTable label="Event types" columns={columns} data={types.data?.data} getRowId={(type) => type.name} loading={types.isPending} />
      <Hint>A code never changes meaning once transfers carry it, so event types are added, never edited or deleted.</Hint>
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent className="sm:max-w-[480px]">
          {creating ? <EventTypeForm ledger={ledger} onDone={() => setCreating(false)} /> : null}
        </DialogContent>
      </Dialog>
    </Page>
  )
}

function EventTypeForm({ ledger, onDone }: { ledger: string; onDone: () => void }) {
  const create = useCreateEventType(ledger)
  const [error, setError] = useState<unknown>()
  const form = useAppForm({
    defaultValues: { code: '', name: '' },
    onSubmit: async ({ value }) => {
      setError(undefined)
      try {
        const created = await create.mutateAsync(value.code.trim() ? { code: Number(value.code), name: value.name } : { name: value.name })
        toast.success(`Event type ${created.name} is code ${created.code}`)
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
        <DialogTitle>New event type</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>The name is what clients send; the code is what TigerBeetle stores. Neither can change later.</DialogDescription>
        <form.AppField
          name="name"
          validators={{
            onChange: ({ value }) =>
              NAME_PATTERN.test(value) ? undefined : 'Start with a letter; lowercase letters, digits, dot, dash or underscore, up to 63.'
          }}
        >
          {(field) => <field.TextField label="Name" mono spellCheck={false} autoComplete="off" placeholder="card.authorized" />}
        </form.AppField>
        <form.AppField
          name="code"
          validators={{
            onChange: ({ value }) =>
              value.trim() === '' || (/^\d+$/.test(value) && Number(value) >= 2 && Number(value) <= 65_535)
                ? undefined
                : 'A whole number from 2 to 65535, or empty.'
          }}
        >
          {(field) => (
            <field.TextField
              label="Code"
              hint="Optional"
              mono
              inputMode="numeric"
              description="Leave empty to take the next free code. 0 is invalid and 1 is the default type."
            />
          )}
        </form.AppField>
        {error ? <ProblemPanel error={error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <form.AppForm>
          <form.SubmitButton>Add event type</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  )
}
