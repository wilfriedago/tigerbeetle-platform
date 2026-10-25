import { useState } from 'react'

import { type BalanceLimit, type Rule } from '@tigerbeetle-platform/api-client'
import { toast } from 'sonner'

import { useAppForm } from '~/components/form/form'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Switch } from '~/components/ui/switch'

import { ADDRESS_PATTERN } from '~/lib/validation'

import { usePutRule } from '../api'

// ADR-0006: a rule decides the flags of the accounts created after it; existing accounts keep
// theirs. A pattern is *, an exact address, or a prefix ending with a colon.

export const LIMIT_OPTIONS: readonly { value: BalanceLimit; label: string }[] = [
  { label: 'Debits must not exceed credits (cannot go negative)', value: 'debits_must_not_exceed_credits' },
  { label: 'Credits must not exceed debits (cannot go positive)', value: 'credits_must_not_exceed_debits' },
  { label: 'None (can go either way, like world)', value: 'none' }
]

function patternError(value: string): string | undefined {
  const pattern = value.trim()
  if (pattern === '') {
    return 'Name the addresses the rule covers.'
  }
  if (pattern === '*' || ADDRESS_PATTERN.test(pattern.replace(/:$/, ''))) {
    return undefined
  }
  return 'Use *, an address such as world, or a prefix that ends with a colon, such as users:.'
}

function codeError(value: string): string | undefined {
  const code = Number(value)
  return /^\d+$/.test(value) && code >= 1 && code <= 65_535 ? undefined : 'A whole number from 1 to 65535.'
}

interface RuleDialogProps {
  ledger: string
  /** The rule to edit; a new rule when absent. */
  rule?: Rule | undefined
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function RuleDialog({ ledger, rule, open, onOpenChange }: RuleDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        {open ? <RuleForm ledger={ledger} rule={rule} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function RuleForm({ ledger, rule, onDone }: { ledger: string; rule: Rule | undefined; onDone: () => void }) {
  const put = usePutRule(ledger)
  const [error, setError] = useState<unknown>()
  const form = useAppForm({
    defaultValues: {
      code: String(rule?.code ?? ''),
      history: rule?.history ?? false,
      limit: rule?.limit ?? 'debits_must_not_exceed_credits',
      pattern: rule?.pattern ?? ''
    },
    onSubmit: async ({ value }) => {
      setError(undefined)
      const limit = LIMIT_OPTIONS.find((option) => option.value === value.limit)?.value ?? 'debits_must_not_exceed_credits'
      try {
        await put.mutateAsync({ body: { code: Number(value.code), history: value.history, limit }, pattern: value.pattern.trim() })
        toast.success(rule ? `Rule ${value.pattern.trim()} updated` : `Rule ${value.pattern.trim()} created`)
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
        <DialogTitle>{rule ? 'Edit rule' : 'New rule'}</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>
          The flags apply to accounts created from now on. Accounts that already exist keep theirs: TigerBeetle account flags never change.
        </DialogDescription>
        <form.AppField name="pattern" validators={{ onChange: ({ value }) => patternError(value) }}>
          {(field) => (
            <field.TextField
              label="Pattern"
              mono
              spellCheck={false}
              autoComplete="off"
              placeholder="users:"
              readOnly={rule !== undefined}
              description="* for every address, an exact address, or a prefix ending with a colon. The most specific match wins."
            />
          )}
        </form.AppField>
        <form.AppField name="limit">{(field) => <field.SelectField label="Balance limit" options={LIMIT_OPTIONS} />}</form.AppField>
        <form.Field name="history">
          {(field) => (
            <Field orientation="horizontal">
              <Switch id="rule-history" checked={field.state.value} onCheckedChange={(checked) => field.handleChange(checked)} />
              <div className="flex flex-col gap-0.5">
                <FieldLabel htmlFor="rule-history">Keep balance history</FieldLabel>
                <FieldDescription>TigerBeetle stores a balance for every transfer of these accounts, for the history chart.</FieldDescription>
              </div>
            </Field>
          )}
        </form.Field>
        <form.AppField name="code" validators={{ onChange: ({ value }) => codeError(value) }}>
          {(field) => (
            <field.TextField label="Account code" mono inputMode="numeric" description="Stored in the code field of each account, from 1 to 65535." />
          )}
        </form.AppField>
        {error ? <ProblemPanel error={error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <form.AppForm>
          <form.SubmitButton>{rule ? 'Save rule' : 'Create rule'}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  )
}
