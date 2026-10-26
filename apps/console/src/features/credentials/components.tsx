import { useState } from 'react'

import { PlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { SectionTitle } from '~/components/layout/page'
import { CopyButton } from '~/components/ledger/copy-button'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
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
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Checkbox } from '~/components/ui/checkbox'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'

import { RoleBadge } from '~/features/members/components'
import { chosenLedgerRoles, type LedgerRoleRow, LedgerRoleRows } from '~/features/members/ledger-role-rows'
import { type LedgerRole, ledgerRoleText } from '~/features/members/roles'

import { type CredentialKind, useRevokeCredentialRole, useSetCredentialRole } from './api'

// The parts the API key and machine client pages of .design/auth.pen share.

/** A credential name, as migration 00014 checks it. */
export const CREDENTIAL_NAME_PATTERN = /^[a-z0-9][a-z0-9-]{0,62}$/

const statusVariant: Record<string, 'success' | 'warning' | 'muted'> = { active: 'success', deleted: 'muted', expired: 'warning', revoked: 'muted' }

export function CredentialStatus({ status }: { status: string }) {
  return <Badge variant={statusVariant[status] ?? 'neutral'}>{status}</Badge>
}

/** "2026-10-05, Hiro Tanaka" */
export function CreatedCell({ at, by }: { at: string; by: string | undefined }) {
  return (
    <span className="flex flex-col">
      <Timestamp value={at} />
      <span className="text-caption text-muted-foreground">{by ?? 'unknown'}</span>
    </span>
  )
}

interface SecretField {
  label: string
  value: string
  mono?: boolean
}

interface SecretOnceDialogProps {
  open: boolean
  onDone: () => void
  title: string
  description: string
  secretLabel: string
  secret: string
  fields: SecretField[]
  example: string
  confirmLabel: string
}

/**
 * .design/auth.pen, "secret shown once": the secret with a copy button, what it is for, a command
 * to try it, and a box to tick before closing, because nothing can show it again.
 */
export function SecretOnceDialog({ open, onDone, title, description, secretLabel, secret, fields, example, confirmLabel }: SecretOnceDialogProps) {
  const [stored, setStored] = useState(false)
  return (
    <Dialog open={open} onOpenChange={(next) => !next && stored && onDone()}>
      <DialogContent className="sm:max-w-[560px]" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <DialogDescription>{description}</DialogDescription>
          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-2 text-small font-medium text-foreground">
              {secretLabel}
              <Badge variant="warning" dot={false}>
                shown once
              </Badge>
            </span>
            <div className="flex items-center gap-2 rounded-md border border-warning-border bg-warning-bg px-3 py-2">
              <code className="flex-1 font-mono text-small break-all text-foreground">{secret}</code>
              <CopyButton value={secret} label="Copy the secret" size="icon-sm" variant="secondary" />
            </div>
          </div>
          <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-small">
            {fields.map((f) => (
              <div key={f.label} className="contents">
                <dt className="text-muted-foreground">{f.label}</dt>
                <dd className={f.mono ? 'font-mono text-foreground' : 'text-foreground'}>{f.value}</dd>
              </div>
            ))}
          </dl>
          <div className="flex flex-col gap-1.5">
            <span className="text-small font-medium text-foreground">Try it</span>
            <div className="relative rounded-md border border-border bg-surface-sunken p-3">
              <pre className="overflow-x-auto font-mono text-caption whitespace-pre text-foreground">{example}</pre>
              <CopyButton value={example} label="Copy the command" className="absolute top-2 right-2" />
            </div>
          </div>
          <label className="flex items-center gap-2 text-small text-foreground">
            <Checkbox checked={stored} onCheckedChange={(checked) => setStored(checked)} />
            {confirmLabel}
          </label>
        </DialogBody>
        <DialogFooter>
          <Button disabled={!stored} onClick={onDone}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface RoleRow {
  ledger: string
  role: string
  grantedAt: string
  grantedBy?: string | undefined
}

const roleColumn = columnHelper<RoleRow>()

/** The ledger roles of a key or a client, with grant and revoke. */
export function CredentialRoles({ kind, id, roles, editable }: { kind: CredentialKind; id: string; roles: readonly RoleRow[]; editable: boolean }) {
  const revoke = useRevokeCredentialRole()
  const [granting, setGranting] = useState(false)
  const columns = [
    roleColumn.accessor('ledger', { cell: (info) => <span className="font-mono">{info.getValue()}</span>, header: 'Ledger' }),
    roleColumn.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Role' }),
    roleColumn.display({
      cell: ({ row }) => <span className="text-muted-foreground">{ledgerRoleText[row.original.role as LedgerRole]?.permissions}</span>,
      header: 'Permissions',
      id: 'permissions',
      meta: { className: 'w-full' }
    }),
    roleColumn.accessor('grantedAt', {
      cell: ({ row }) => <CreatedCell at={row.original.grantedAt} by={row.original.grantedBy} />,
      header: 'Granted'
    }),
    roleColumn.display({
      cell: ({ row }) =>
        editable ? (
          <Button
            size="sm"
            variant="ghost"
            className="text-danger-fg"
            loading={revoke.isPending && revoke.variables?.ledger === row.original.ledger}
            onClick={() =>
              revoke.mutate(
                { id, kind, ledger: row.original.ledger },
                { onSuccess: () => toast.success(`${row.original.role} on ${row.original.ledger} revoked`) }
              )
            }
          >
            Revoke
          </Button>
        ) : null,
      header: '',
      id: 'actions'
    })
  ]
  return (
    <section className="flex flex-col gap-3">
      <SectionTitle
        title="Roles"
        description="What it can do. Changes apply on its next request."
        actions={
          editable ? (
            <Button size="sm" variant="secondary" onClick={() => setGranting(true)}>
              <PlusIcon />
              Grant role
            </Button>
          ) : null
        }
      />
      {revoke.error ? <ProblemPanel error={revoke.error} /> : null}
      <DataTable label="Roles" columns={columns} data={roles} getRowId={(r) => r.ledger} empty="No role: it can do nothing." />
      <GrantDialog kind={kind} id={id} open={granting} onOpenChange={setGranting} />
    </section>
  )
}

function GrantDialog({ kind, id, open, onOpenChange }: { kind: CredentialKind; id: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const grant = useSetCredentialRole()
  const [rows, setRows] = useState<LedgerRoleRow[]>([{ id: 1, ledger: '', role: 'viewer' }])
  const chosen = chosenLedgerRoles(rows)
  const submit = async () => {
    for (const r of chosen) {
      await grant.mutateAsync({ id, kind, ledger: r.ledger, role: r.role })
    }
    toast.success(chosen.length === 1 ? 'Role granted' : `${chosen.length} roles granted`)
    onOpenChange(false)
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Grant a ledger role</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <DialogDescription>A role on a ledger replaces the one it held there. Give it the smallest role it needs.</DialogDescription>
          <Field>
            <FieldLabel>Roles</FieldLabel>
            <LedgerRoleRows rows={rows} onChange={setRows} />
          </Field>
          {grant.error ? <ProblemPanel error={grant.error} /> : null}
        </DialogBody>
        <DialogFooter>
          <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
          <Button disabled={chosen.length === 0} loading={grant.isPending} onClick={() => void submit().catch(() => undefined)}>
            Grant
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface ConfirmByNameProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  name: string
  consequences: string[]
  action: string
  pending: boolean
  error: unknown
  onConfirm: () => void
}

/** .design/auth.pen, the revoke and delete dialogs: what happens, then type the name to confirm. */
export function ConfirmByName({ open, onOpenChange, title, name, consequences, action, pending, error, onConfirm }: ConfirmByNameProps) {
  const [typed, setTyped] = useState('')
  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setTyped('')
        }
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogBody>
          <AlertDialogDescription>{consequences[0]}</AlertDialogDescription>
          <ul className="flex list-disc flex-col gap-1 pl-5 text-small text-foreground">
            {consequences.slice(1).map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
          <Field>
            <FieldLabel htmlFor="confirm-name">
              Type <span className="font-mono">{name}</span> to confirm
            </FieldLabel>
            <Input id="confirm-name" className="font-mono" autoComplete="off" value={typed} onChange={(event) => setTyped(event.target.value)} />
          </Field>
          {error ? <ProblemPanel error={error} /> : null}
        </AlertDialogBody>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" disabled={typed !== name} loading={pending} onClick={onConfirm}>
            {action}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
