import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'

import { type Member } from '@tigerbeetle-platform/api-client'
import { CheckIcon, PlusIcon, UserMinusIcon, XIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, SectionTitle } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
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
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card'
import { Skeleton } from '~/components/ui/skeleton'

import { canManageLedgerAccess, canManageMembers } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { findMember, memberQueries, useRemoveMember, useRevokeLedgerRole, useSetMemberRole } from './api'
import { Avatar, RoleBadge } from './components'
import { GrantLedgerRoleDialog } from './grant-dialog'
import { capabilities, type LedgerRole, ledgerRoleText, ORGANIZATION_ROLES, organizationRoleText } from './roles'

// .design/auth.pen, Member: one person's organization role, ledger roles, and what Keycloak knows
// of their account. Owners and admins change the roles; owners alone touch owner (ADR-0028).

const route = getRouteApi('/_console/$org/members/$member')

type LedgerRoleRow = Member['ledgers'][number]

const column = columnHelper<LedgerRoleRow>()

function firstName(m: Member): string {
  return (m.name || m.email).split(/\s+/)[0] ?? m.email
}

export function MemberPage() {
  const org = useOrganization()
  const { member: subject } = route.useParams()
  const session = useSession()
  const members = useQuery(memberQueries.list())
  const member = findMember(members.data, subject)
  const [removing, setRemoving] = useState(false)
  const [granting, setGranting] = useState(false)
  const setRole = useSetMemberRole()
  const revoke = useRevokeLedgerRole()

  if (members.error) {
    return (
      <Page>
        <ProblemPanel error={members.error} onRetry={() => void members.refetch()} />
      </Page>
    )
  }
  if (!member) {
    return <Page>{members.isPending ? <Skeleton className="h-40" /> : <ProblemPanel error={new Error(`No member ${subject} in ${org}.`)} />}</Page>
  }

  const manage = canManageMembers(session)
  const isOwner = session?.role === 'owner'
  const you = member.subject === session?.subject
  const name = firstName(member)
  const can = capabilities(member.role, member.ledgers)

  const columns = [
    column.accessor('ledger', { cell: (info) => <span className="font-mono">{info.getValue()}</span>, header: 'Ledger' }),
    column.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Role' }),
    column.display({
      cell: ({ row }) => <span className="text-muted-foreground">{ledgerRoleText[row.original.role as LedgerRole]?.permissions}</span>,
      header: 'Permissions',
      id: 'permissions',
      meta: { className: 'w-full' }
    }),
    column.accessor('grantedAt', {
      cell: ({ row }) => (
        <span className="flex flex-col">
          <Timestamp value={row.original.grantedAt} />
          <span className="text-caption text-muted-foreground">{row.original.grantedBy ?? 'platform staff'}</span>
        </span>
      ),
      header: 'Granted'
    }),
    column.display({
      cell: ({ row }) =>
        canManageLedgerAccess(session, row.original.ledger) ? (
          <Button
            size="sm"
            variant="ghost"
            className="text-danger-fg"
            loading={revoke.isPending && revoke.variables?.ledger === row.original.ledger}
            onClick={() =>
              revoke.mutate(
                { ledger: row.original.ledger, subject: member.subject },
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
    <Page>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex items-center gap-4">
          <Avatar name={member.name || member.email} className="size-12 text-body" />
          <div className="flex flex-col gap-1">
            <h1 className="flex items-center gap-2 text-page-title text-foreground">
              {member.name || member.email}
              {member.status === 'active' ? null : <Badge variant="warning">{member.status === 'invited' ? 'invited' : 'setting up'}</Badge>}
            </h1>
            <p className="text-small text-muted-foreground">
              {member.email}
              {member.addedAt ? (
                <>
                  {' · '}
                  {member.status === 'active' ? 'member since ' : 'invited '}
                  <Timestamp value={member.addedAt} />
                  {member.addedBy ? `, by ${member.addedBy}` : ''}
                </>
              ) : null}
            </p>
          </div>
        </div>
        {manage || you ? (
          <Button variant="secondary" className="text-danger-fg" onClick={() => setRemoving(true)}>
            <UserMinusIcon />
            {you ? 'Leave the organization' : 'Remove from the organization'}
          </Button>
        ) : null}
      </header>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <section className="flex flex-col gap-3">
            <SectionTitle title="Organization role" description={`Decides what ${name} sees across the organization. Only owners can grant owner.`} />
            <div role="radiogroup" aria-label="Organization role" className="grid gap-2 sm:grid-cols-3">
              {ORGANIZATION_ROLES.map((r) => {
                const touchesOwner = r === 'owner' || member.role === 'owner'
                const disabled = !manage || (touchesOwner && !isOwner) || setRole.isPending
                const checked = member.role === r
                return (
                  <button
                    key={r}
                    type="button"
                    role="radio"
                    aria-checked={checked}
                    disabled={disabled && !checked}
                    onClick={() =>
                      !checked &&
                      setRole.mutate(
                        { role: r, subject: member.subject },
                        { onSuccess: () => toast.success(`${member.name || member.email} is now ${r}`) }
                      )
                    }
                    className="flex flex-col items-start gap-1 rounded-md border border-border px-3 py-2.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-focus-ring disabled:cursor-not-allowed disabled:opacity-45 aria-checked:border-primary-border aria-checked:bg-primary-subtle"
                  >
                    <span className="flex items-center gap-1.5 font-mono text-body text-foreground">
                      {checked ? <CheckIcon className="size-3.5 text-primary-text" /> : null}
                      {r}
                    </span>
                    <span className="text-caption text-muted-foreground">{organizationRoleText[r].summary}</span>
                  </button>
                )
              })}
            </div>
            {setRole.error ? <ProblemPanel error={setRole.error} /> : null}
          </section>

          <section className="flex flex-col gap-3">
            <SectionTitle
              title="Ledger roles"
              description="Roles on single ledgers. They stop at once when revoked."
              actions={
                manage ? (
                  <Button size="sm" variant="secondary" onClick={() => setGranting(true)}>
                    <PlusIcon />
                    Grant ledger role
                  </Button>
                ) : null
              }
            />
            {member.role === 'admin' || member.role === 'owner' ? (
              <Hint>
                As {member.role}, {name} can do everything on every ledger; ledger roles add nothing.
              </Hint>
            ) : null}
            {revoke.error ? <ProblemPanel error={revoke.error} /> : null}
            <DataTable label="Ledger roles" columns={columns} data={member.ledgers} getRowId={(l) => l.ledger} empty="No ledger role." />
          </section>
        </div>

        <aside className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle>Account in Keycloak</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <DescriptionList>
                <DescriptionItem term="Email">
                  {member.email}
                  {member.emailVerified ? (
                    <span className="text-muted-foreground">, verified</span>
                  ) : (
                    <span className="text-warning-fg">, not verified</span>
                  )}
                </DescriptionItem>
                <DescriptionItem term="Subject">
                  <span className="font-mono text-small break-all">{member.subject}</span>
                </DescriptionItem>
              </DescriptionList>
              <Hint>Name, email and sign-in methods are edited in Keycloak, not here.</Hint>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>What {name} can do</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="flex flex-col gap-2 text-small">
                {can.yes.map((line) => (
                  <li key={line} className="flex items-start gap-2 text-foreground">
                    <CheckIcon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-success-fg" />
                    {line}
                  </li>
                ))}
                {can.no.map((line) => (
                  <li key={line} className="flex items-start gap-2 text-muted-foreground">
                    <XIcon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle-foreground" />
                    <span className="sr-only">Cannot: </span>
                    {line}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </aside>
      </div>

      <RemoveDialog member={member} you={you} open={removing} onOpenChange={setRemoving} />
      <GrantLedgerRoleDialog open={granting} onOpenChange={setGranting} subject={member.subject} />
    </Page>
  )
}

function RemoveDialog({ member, you, open, onOpenChange }: { member: Member; you: boolean; open: boolean; onOpenChange: (open: boolean) => void }) {
  const org = useOrganization()
  const navigate = useNavigate()
  const remove = useRemoveMember()
  const name = member.name || member.email
  const roles = member.ledgers.map((l) => `${l.role} on ${l.ledger}`)
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{you ? 'Leave the organization?' : `Remove ${name} from the organization?`}</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogBody>
          <AlertDialogDescription>
            {you ? 'You lose' : `${firstName(member)} loses`} access when the session renews, within 5 minutes. Nothing written is changed.
          </AlertDialogDescription>
          <ul className="flex list-disc flex-col gap-1 pl-5 text-small text-foreground">
            {roles.length > 0 ? (
              <li>
                Revokes {roles.length === 1 ? 'one ledger role' : `${roles.length} ledger roles`}: {roles.join(', ')}.
              </li>
            ) : null}
            <li>Removes the membership in Keycloak.</li>
            <li>Keeps the Keycloak account and any other organization.</li>
            <li>Keeps the person as the actor on past transactions and write requests.</li>
          </ul>
          {remove.error ? <ProblemPanel error={remove.error} /> : null}
        </AlertDialogBody>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button
            variant="destructive"
            loading={remove.isPending}
            onClick={() =>
              remove.mutate(member.subject, {
                onSuccess: () => {
                  toast.success(you ? 'You left the organization' : `${name} was removed`)
                  onOpenChange(false)
                  void navigate(you ? { to: '/' } : { params: { org }, to: '/$org/members' })
                }
              })
            }
          >
            {you ? 'Leave' : 'Remove member'}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
