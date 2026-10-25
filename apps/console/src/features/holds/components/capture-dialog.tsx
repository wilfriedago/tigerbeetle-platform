import { useId, useState } from 'react'

import { type Hold } from '@tigerbeetle-platform/api-client'
import { toast } from 'sonner'

import { useFormattedAmount } from '~/components/ledger/amount'
import { IdempotencyKeyField } from '~/components/ledger/idempotency-key-field'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldDescription, FieldError, FieldLabel } from '~/components/ui/field'
import { InputGroup, InputGroupAddon, InputGroupInput, InputGroupText } from '~/components/ui/input-group'
import { Label } from '~/components/ui/label'
import { RadioGroup, RadioGroupItem } from '~/components/ui/radio-group'

import { keyIsSpent, useIdempotencyKey } from '~/lib/idempotency'
import { assetScale, compareAmounts, parseMajorUnits, subtractAmounts } from '~/lib/money'

import { useCaptureHold } from '../api'

// docs/console.md, Hold: a capture posts the pending transfer, in full or in part. A partial capture
// closes the hold and releases the rest to the source; a hold is captured once (ADR-0010).

interface CaptureDialogProps {
  ledger: string
  hold: Hold
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CaptureDialog({ ledger, hold, open, onOpenChange }: CaptureDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        {open ? <CaptureForm ledger={ledger} hold={hold} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function CaptureForm({ ledger, hold, onDone }: { ledger: string; hold: Hold; onDone: () => void }) {
  const id = useId()
  const capture = useCaptureHold(ledger, hold.id)
  const idempotency = useIdempotencyKey()
  const [mode, setMode] = useState<'full' | 'partial'>('full')
  const [text, setText] = useState('')
  const [error, setError] = useState<unknown>()
  const scale = assetScale(hold.asset)
  const parsed = parseMajorUnits(text, scale)
  let amountError: string | undefined
  if (mode === 'partial' && text !== '') {
    if (!parsed.ok) {
      amountError = parsed.error
    } else if (parsed.raw === '0') {
      amountError = 'Capture more than zero, or void the hold.'
    } else if (compareAmounts(parsed.raw, hold.amount) > 0) {
      amountError = 'A capture cannot exceed the amount held.'
    }
  }
  const raw = mode === 'full' ? hold.amount : parsed.ok ? parsed.raw : undefined
  const ready = raw !== undefined && amountError === undefined && (mode === 'full' || text !== '')
  const released = raw && ready ? subtractAmounts(hold.amount, raw) : '0'
  const fullText = useFormattedAmount(hold.amount, hold.asset)
  const captureText = useFormattedAmount(raw ?? '0', hold.asset)
  const releasedText = useFormattedAmount(released, hold.asset)

  const submit = async () => {
    if (!ready) {
      return
    }
    const body = mode === 'full' ? {} : { amount: raw }
    setError(undefined)
    try {
      await capture.mutateAsync({ body, idempotencyKey: idempotency.keyFor(body) })
      toast.success('Hold captured', { description: `${captureText} posted to ${hold.destination}` })
      onDone()
    } catch (error_) {
      setError(error_)
      if (keyIsSpent(error_)) {
        idempotency.renew()
      }
    }
  }

  return (
    <form
      noValidate
      className="contents"
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <DialogHeader>
        <DialogTitle>Capture this hold?</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>
          Capturing posts the pending transfer. A partial capture posts the amount you enter, releases the rest to{' '}
          <span className="font-mono">{hold.source}</span> and closes the hold. A hold is captured only once.
        </DialogDescription>
        <RadioGroup value={mode} onValueChange={(value) => setMode(value === 'partial' ? 'partial' : 'full')} aria-label="Capture amount">
          <Label className="font-normal">
            <RadioGroupItem value="full" />
            Full amount, {fullText}
          </Label>
          <Label className="font-normal">
            <RadioGroupItem value="partial" />
            Partial amount
          </Label>
        </RadioGroup>
        {mode === 'partial' ? (
          <Field data-invalid={amountError ? true : undefined}>
            <FieldLabel htmlFor={id}>Amount to capture</FieldLabel>
            <InputGroup>
              <InputGroupInput
                id={id}
                inputMode="decimal"
                autoComplete="off"
                value={text}
                onChange={(event) => setText(event.target.value)}
                aria-invalid={amountError ? true : undefined}
                className="text-right font-mono tabular-nums"
              />
              <InputGroupAddon align="inline-end">
                <InputGroupText className="font-mono">{hold.asset}</InputGroupText>
              </InputGroupAddon>
            </InputGroup>
            {amountError ? (
              <FieldError>{amountError}</FieldError>
            ) : parsed.ok && text !== '' ? (
              <FieldDescription>
                Sent as <span className="font-mono text-foreground">"{parsed.raw}"</span> in the smallest unit.
              </FieldDescription>
            ) : null}
          </Field>
        ) : null}
        <div className="flex items-center justify-between rounded-md bg-surface-sunken px-3 py-2.5 text-small">
          <span className="text-muted-foreground">Released back to the source</span>
          <span className="font-mono tabular-nums">{releasedText}</span>
        </div>
        <IdempotencyKeyField value={idempotency.key} />
        {error ? <ProblemPanel error={error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <Button type="submit" disabled={!ready} loading={capture.isPending}>
          Capture {ready ? captureText : ''}
        </Button>
      </DialogFooter>
    </form>
  )
}
