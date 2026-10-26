import { useState } from 'react'

import { InfoIcon } from 'lucide-react'
import { toast } from 'sonner'

import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Textarea } from '~/components/ui/textarea'

import { useSession } from '~/lib/auth/use-session'

import { useInviteMember } from './api'
import { chosenLedgerRoles, type LedgerRoleRow, LedgerRoleRows } from './ledger-role-rows'
import { ORGANIZATION_ROLES, organizationRoleText } from './roles'

// .design/auth.pen, Members, invite dialog: several people at once, one organization role for all
// of them, and optional ledger roles. Each person gets one email from Keycloak: an invitation when
// they have an account, an account setup email when they do not (ADR-0027).

const MAX_EMAILS = 20

/** Splits the textarea on commas, spaces and new lines. */
export function parseEmails(text: string): string[] {
  return [
    ...new Set(
      text
        .split(/[\s,;]+/)
        .map((e) => e.trim().toLowerCase())
        .filter(Boolean)
    )
  ]
}

/** A plain address such as ada@example.com; the API checks it again. */
export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function InviteDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px]">{open ? <InviteForm onDone={() => onOpenChange(false)} /> : null}</DialogContent>
    </Dialog>
  )
}

function InviteForm({ onDone }: { onDone: () => void }) {
  const session = useSession()
  const invite = useInviteMember()
  const [text, setText] = useState('')
  const [role, setRole] = useState('member')
  const [rows, setRows] = useState<LedgerRoleRow[]>([])
  const [failures, setFailures] = useState<{ email: string; error: unknown }[]>([])
  const [sending, setSending] = useState(false)

  const emails = parseEmails(text)
  const invalid = emails.filter((e) => !EMAIL_PATTERN.test(e))
  const tooMany = emails.length > MAX_EMAILS
  const ledgerRows = chosenLedgerRoles(rows)
  const canSubmit = emails.length > 0 && invalid.length === 0 && !tooMany && !sending

  const submit = async () => {
    setSending(true)
    const failed: { email: string; error: unknown }[] = []
    // One after the other: each invitation is a few calls to Keycloak, and a failure names its email.
    for (const email of emails) {
      try {
        await invite.mutateAsync({ email, ledgers: ledgerRows, role })
      } catch (error) {
        failed.push({ email, error })
      }
    }
    setSending(false)
    const sent = emails.length - failed.length
    if (sent > 0) {
      toast.success(sent === 1 ? 'Invitation sent' : `${sent} invitations sent`)
    }
    if (failed.length === 0) {
      onDone()
      return
    }
    setFailures(failed)
    setText(failed.map((f) => f.email).join('\n'))
  }

  return (
    <form
      noValidate
      className="contents"
      onSubmit={(event) => {
        event.preventDefault()
        if (canSubmit) {
          void submit()
        }
      }}
    >
      <DialogHeader>
        <DialogTitle>Invite members</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>
          Each person gets an email from Keycloak. The roles below are waiting for them and apply when they accept.
        </DialogDescription>
        <Field>
          <FieldLabel htmlFor="invite-emails">Email addresses</FieldLabel>
          <Textarea
            id="invite-emails"
            rows={3}
            className="font-mono"
            value={text}
            placeholder="li.na@example.com, maria@example.com"
            aria-invalid={invalid.length > 0 || tooMany || undefined}
            onChange={(event) => setText(event.target.value)}
          />
          <FieldDescription>
            {invalid.length > 0
              ? `Not an email address: ${invalid.join(', ')}`
              : tooMany
                ? `Up to ${MAX_EMAILS} at once; this is ${emails.length}.`
                : `Separate with commas or new lines, up to ${MAX_EMAILS} at once.`}
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel>Organization role</FieldLabel>
          <div role="radiogroup" aria-label="Organization role" className="flex flex-col gap-1.5">
            {ORGANIZATION_ROLES.map((r) => {
              const disabled = r === 'owner' && session?.role !== 'owner'
              return (
                <button
                  key={r}
                  type="button"
                  role="radio"
                  aria-checked={role === r}
                  disabled={disabled}
                  onClick={() => setRole(r)}
                  className="flex flex-col items-start gap-0.5 rounded-md border border-border px-3 py-2.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-focus-ring disabled:cursor-not-allowed disabled:opacity-45 aria-checked:border-primary-border aria-checked:bg-primary-subtle"
                >
                  <span className="font-mono text-body text-foreground">{r}</span>
                  <span className="text-caption text-muted-foreground">
                    {organizationRoleText[r].detail}
                    {disabled ? ' Only an owner can invite an owner.' : ''}
                  </span>
                </button>
              )
            })}
          </div>
        </Field>
        {role === 'member' ? (
          <Field>
            <FieldLabel>
              Ledger roles <span className="font-normal text-subtle-foreground">optional</span>
            </FieldLabel>
            <LedgerRoleRows rows={rows} onChange={setRows} />
          </Field>
        ) : null}
        <Alert>
          <InfoIcon />
          <AlertTitle>No account yet?</AlertTitle>
          <AlertDescription>
            The platform creates one in Keycloak and the email links to verifying the address and setting a password. A person who already has an
            account becomes a member by following the invitation.
          </AlertDescription>
        </Alert>
        {failures.map((f) => (
          <ProblemPanel key={f.email} error={f.error} title={`Could not invite ${f.email}`} />
        ))}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <Button type="submit" disabled={!canSubmit} loading={sending}>
          {emails.length > 1 ? `Send ${emails.length} invitations` : 'Send invitation'}
        </Button>
      </DialogFooter>
    </form>
  )
}
