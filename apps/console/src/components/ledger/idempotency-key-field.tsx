import { useId } from 'react'

import { Field, FieldDescription, FieldHeader, FieldHint, FieldLabel } from '~/components/ui/field'
import { InputGroup, InputGroupAddon, InputGroupInput } from '~/components/ui/input-group'

import { CopyButton } from './copy-button'

// 20 Money and identity, Idempotency Key Field: read-only. The console makes the key when the form
// opens, sends it again on a retry and makes a new one when the request changes (ADR-0007).
export function IdempotencyKeyField({ value, description }: { value: string; description?: React.ReactNode }) {
  const id = useId()
  return (
    <Field>
      <FieldHeader>
        <FieldLabel htmlFor={id}>Idempotency-Key</FieldLabel>
        <FieldHint>header</FieldHint>
      </FieldHeader>
      <InputGroup className="bg-surface-sunken">
        <InputGroupInput id={id} readOnly value={value} className="font-mono text-small text-muted-foreground" aria-describedby={`${id}-help`} />
        <InputGroupAddon align="inline-end">
          <CopyButton value={value} label="Copy Idempotency-Key" />
        </InputGroupAddon>
      </InputGroup>
      <FieldDescription id={`${id}-help`}>
        {description ?? 'A retry sends the same key and cannot apply the write twice. Changing the request makes a new key.'}
      </FieldDescription>
    </Field>
  )
}
