import { useState } from 'react'

import { type Hold } from '@tigerbeetle-platform/api-client'
import { toast } from 'sonner'

import { useFormattedAmount } from '~/components/ledger/amount'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '~/components/ui/alert-dialog'
import { Button } from '~/components/ui/button'

import { keyIsSpent, useIdempotencyKey } from '~/lib/idempotency'

import { useVoidHold } from '../api'

interface VoidDialogProps {
  ledger: string
  hold: Hold
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** Voiding releases the whole amount to the source; like a capture, it happens once (ADR-0010). */
export function VoidDialog({ ledger, hold, open, onOpenChange }: VoidDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>{open ? <VoidForm ledger={ledger} hold={hold} onDone={() => onOpenChange(false)} /> : null}</AlertDialogContent>
    </AlertDialog>
  )
}

function VoidForm({ ledger, hold, onDone }: { ledger: string; hold: Hold; onDone: () => void }) {
  const voidHold = useVoidHold(ledger, hold.id)
  const idempotency = useIdempotencyKey()
  const [error, setError] = useState<unknown>()
  const amount = useFormattedAmount(hold.amount, hold.asset)
  const submit = async () => {
    setError(undefined)
    try {
      await voidHold.mutateAsync({ idempotencyKey: idempotency.keyFor({ void: hold.id }) })
      toast.success('Hold voided', { description: `${amount} released to ${hold.source}` })
      onDone()
    } catch (error_) {
      setError(error_)
      if (keyIsSpent(error_)) {
        idempotency.renew()
      }
    }
  }
  return (
    <>
      <AlertDialogHeader>
        <AlertDialogTitle>Void this hold?</AlertDialogTitle>
      </AlertDialogHeader>
      <AlertDialogBody>
        <AlertDialogDescription>
          Voiding releases {amount} back to <span className="font-mono">{hold.source}</span> and closes the hold. Nothing reaches{' '}
          <span className="font-mono">{hold.destination}</span>. A voided hold cannot be captured.
        </AlertDialogDescription>
        {error ? <ProblemPanel error={error} /> : null}
      </AlertDialogBody>
      <AlertDialogFooter>
        <AlertDialogCancel>Keep the hold</AlertDialogCancel>
        <Button variant="destructive" loading={voidHold.isPending} onClick={() => void submit()}>
          Void hold
        </Button>
      </AlertDialogFooter>
    </>
  )
}
