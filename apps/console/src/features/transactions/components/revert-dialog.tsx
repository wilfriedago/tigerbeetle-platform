import { useState } from 'react'

import { useNavigate } from '@tanstack/react-router'

import { type Transaction } from '@tigerbeetle-platform/api-client'
import { toast } from 'sonner'

import { useAppForm } from '~/components/form/form'
import { IdempotencyKeyField } from '~/components/ledger/idempotency-key-field'
import { PostingRow } from '~/components/ledger/posting-chain'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'

import { keyIsSpent, useIdempotencyKey } from '~/lib/idempotency'
import { shortId } from '~/lib/ids'

import { useRevertTransaction } from '../api'

interface RevertDialogProps {
  ledger: string
  transaction: Transaction
  open: boolean
  onOpenChange: (open: boolean) => void
}

// docs/console.md, Transaction: a revert is a new transaction with every posting reversed. It can be
// declined like any other, for example when a destination has spent the money, and there is no
// force option (ADR-0004).
export function RevertDialog({ ledger, transaction, open, onOpenChange }: RevertDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        {open ? <RevertForm ledger={ledger} transaction={transaction} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function RevertForm({ ledger, transaction, onDone }: { ledger: string; transaction: Transaction; onDone: () => void }) {
  const navigate = useNavigate()
  const revert = useRevertTransaction(ledger, transaction.id)
  const idempotency = useIdempotencyKey()
  const [error, setError] = useState<unknown>()
  const form = useAppForm({
    defaultValues: { reference: transaction.reference ? `${transaction.reference}-revert` : '' },
    onSubmit: async ({ value }) => {
      const body = value.reference.trim() ? { reference: value.reference.trim() } : {}
      setError(undefined)
      try {
        const created = await revert.mutateAsync({ body, idempotencyKey: idempotency.keyFor(body) })
        toast.success(`Reverted ${shortId(transaction.id)}`, { description: `New transaction ${shortId(created.id)}` })
        onDone()
        await navigate({ params: { ledger, txId: created.id }, to: '/ledgers/$ledger/transactions/$txId' })
      } catch (error_) {
        setError(error_)
        if (keyIsSpent(error_)) {
          idempotency.renew()
        }
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
        <DialogTitle>Revert transaction</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>
          A revert is a new transaction that moves the money back, posting by posting. TigerBeetle applies it only if every account can afford it, so
          it can be declined when a destination has already spent the funds. A transaction is reverted once.
        </DialogDescription>
        <div className="rounded-md border border-border bg-surface-sunken px-3 py-1">
          {transaction.postings.toReversed().map((posting) => (
            <PostingRow
              key={posting.transferId}
              posting={{ amount: posting.amount, asset: posting.asset, destination: posting.source, source: posting.destination }}
            />
          ))}
        </div>
        <form.AppField name="reference">
          {(field) => (
            <field.TextField label="Reference" hint="optional" mono maxLength={255} description="Stored on the revert, for your own systems." />
          )}
        </form.AppField>
        <IdempotencyKeyField value={idempotency.key} />
        {error ? (
          <ProblemPanel error={error}>
            {keyIsSpent(error) ? (
              <p className="text-caption text-muted-foreground">This key keeps its answer. Reverting again sends a new key.</p>
            ) : null}
          </ProblemPanel>
        ) : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <form.AppForm>
          <form.SubmitButton>{error && keyIsSpent(error) ? 'Revert with a new key' : 'Revert'}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  )
}
