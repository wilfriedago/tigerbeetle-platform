import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'

import { toast } from 'sonner'

import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'

import { ledgerQueries } from '~/features/ledgers/api'

import { cn } from '~/lib/utils'

import { memberQueries, useSetLedgerRole } from './api'
import { PersonCell } from './components'
import { LEDGER_ROLES, ledgerRoleText } from './roles'

// .design/auth.pen, Ledger access, grant dialog: a role on one ledger for a person of the
// organization. From a member's page the person is fixed and the ledger is picked; from a ledger's
// Access page the ledger is fixed and the person is picked. API keys and machine clients join the
// people here in phase 6.

interface GrantLedgerRoleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The person, when the dialog opens from their page. */
  subject?: string | undefined
  /** The ledger, when the dialog opens from its Access page. */
  ledger?: string | undefined
}

export function GrantLedgerRoleDialog({ open, onOpenChange, subject, ledger }: GrantLedgerRoleDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        {open ? <GrantForm subject={subject} ledger={ledger} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function GrantForm({
  subject: fixedSubject,
  ledger: fixedLedger,
  onDone
}: {
  subject?: string | undefined
  ledger?: string | undefined
  onDone: () => void
}) {
  const members = useQuery(memberQueries.list())
  const ledgers = useQuery({ ...ledgerQueries.list(), enabled: fixedLedger === undefined })
  const grant = useSetLedgerRole()
  const [subject, setSubject] = useState(fixedSubject ?? '')
  const [ledger, setLedger] = useState(fixedLedger ?? '')
  const [role, setRole] = useState<string>('viewer')
  const [query, setQuery] = useState('')

  const people = (members.data ?? []).filter((m) => m.role !== 'admin' && m.role !== 'owner')
  const q = query.toLowerCase()
  const matches = people.filter((m) => !q || m.name.toLowerCase().includes(q) || m.email.toLowerCase().includes(q)).slice(0, 6)
  const person = members.data?.find((m) => m.subject === subject)

  return (
    <form
      noValidate
      className="contents"
      onSubmit={(event) => {
        event.preventDefault()
        if (!subject || !ledger) {
          return
        }
        grant.mutate(
          { ledger, role, subject },
          {
            onSuccess: () => {
              toast.success(`${person?.name || person?.email || 'The member'} is ${role} on ${ledger}`)
              onDone()
            }
          }
        )
      }}
    >
      <DialogHeader>
        <DialogTitle>
          {fixedLedger ? (
            <>
              Grant access to <span className="font-mono">{fixedLedger}</span>
            </>
          ) : (
            'Grant a ledger role'
          )}
        </DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>
          Only members of the organization can be granted a role. Invite new people from Members first. Organization admins and owners already have
          every role.
        </DialogDescription>
        {fixedSubject ? null : (
          <Field>
            <FieldLabel htmlFor="grant-who">Who</FieldLabel>
            <Input
              id="grant-who"
              value={query}
              placeholder="Search name or email"
              autoComplete="off"
              onChange={(event) => setQuery(event.target.value)}
            />
            <div role="listbox" aria-label="Members" className="flex flex-col gap-1">
              {matches.map((m) => (
                <button
                  key={m.subject}
                  type="button"
                  role="option"
                  aria-selected={m.subject === subject}
                  onClick={() => setSubject(m.subject)}
                  className={cn(
                    'flex items-center justify-between gap-3 rounded-md border border-transparent px-2 py-1.5 text-left outline-none hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring',
                    m.subject === subject && 'border-primary-border bg-primary-subtle'
                  )}
                >
                  <PersonCell name={m.name} email={m.email} />
                  <span className="shrink-0 text-caption text-muted-foreground">
                    {m.ledgers.length === 0 ? 'no ledger roles' : m.ledgers.map((l) => `${l.role} on ${l.ledger}`).join(', ')}
                  </span>
                </button>
              ))}
              {matches.length === 0 ? (
                <p className="px-2 text-caption text-muted-foreground">No member without an organization admin role matches.</p>
              ) : null}
            </div>
          </Field>
        )}
        {fixedLedger ? null : (
          <Field>
            <FieldLabel>Ledger</FieldLabel>
            <Select
              items={(ledgers.data?.data ?? []).map((l) => ({ label: l.name, value: l.name }))}
              value={ledger}
              onValueChange={(value) => setLedger(value ?? '')}
            >
              <SelectTrigger className="font-mono" aria-label="Ledger">
                <SelectValue placeholder="Choose a ledger" />
              </SelectTrigger>
              <SelectContent>
                {(ledgers.data?.data ?? []).map((l) => (
                  <SelectItem key={l.name} value={l.name} className="font-mono">
                    {l.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}
        <Field>
          <FieldLabel>
            Role
            {fixedLedger ? (
              <>
                {' '}
                on <span className="font-mono">{fixedLedger}</span>
              </>
            ) : null}
          </FieldLabel>
          <div role="radiogroup" aria-label="Role" className="grid gap-2 sm:grid-cols-3">
            {LEDGER_ROLES.map((r) => (
              <button
                key={r}
                type="button"
                role="radio"
                aria-checked={role === r}
                onClick={() => setRole(r)}
                className="flex flex-col items-start gap-1 rounded-md border border-border px-3 py-2 text-left outline-none focus-visible:ring-3 focus-visible:ring-focus-ring aria-checked:border-primary-border aria-checked:bg-primary-subtle"
              >
                <span className="font-mono text-body text-foreground">{r}</span>
                <span className="text-caption text-muted-foreground">{ledgerRoleText[r].summary}</span>
              </button>
            ))}
          </div>
        </Field>
        {grant.error ? <ProblemPanel error={grant.error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <Button type="submit" disabled={!subject || !ledger} loading={grant.isPending}>
          Grant {role}
        </Button>
      </DialogFooter>
    </form>
  )
}
