import { useMemo } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'

import { type Member } from '@tigerbeetle-platform/api-client'
import { ShieldCheckIcon, UserPlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter, SelectFilter } from '~/components/data-table/filters'
import { Page, PageHeader } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Tabs, TabsCount, TabsList, TabsTrigger } from '~/components/ui/tabs'

import { ledgerQueries } from '~/features/ledgers/api'

import { canManageMembers } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { memberQueries, useDeleteInvitation, useInviteMember, useRemoveMember, useResendInvitation } from './api'
import { LedgerRoles, PersonCell, RoleBadge } from './components'
import { InviteDialog } from './invite-dialog'
import { ORGANIZATION_ROLES } from './roles'

// .design/auth.pen, Members: the people of the organization and their roles, in two tabs. Members
// are in Keycloak's organization; Invitations are the people on their way in: an invitation not
// yet accepted, or an account the platform created and its owner has not set up. Every member sees
// the list; owners and admins change it (ADR-0028).

const route = getRouteApi('/_console/$org/members/')

const column = columnHelper<Member>()

function AddedCell({ at, by }: { at: string | undefined; by: string | undefined }) {
  if (!at) {
    return null
  }
  return (
    <span className="flex flex-col">
      <Timestamp value={at} />
      <span className="text-caption text-muted-foreground">{by ? `by ${by}` : 'by platform staff'}</span>
    </span>
  )
}

export function MembersPage() {
  const org = useOrganization()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const goTo = useNavigate()
  const session = useSession()
  const manage = canManageMembers(session)
  const members = useQuery(memberQueries.list())
  const ledgers = useQuery(ledgerQueries.list())
  const tab = search.tab ?? 'members'
  const setSearch = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch }) })

  const all = members.data ?? []
  const active = all.filter((m) => m.status === 'active')
  const pending = all.filter((m) => m.status !== 'active')
  const query = search.q?.toLowerCase()
  const shown = active.filter(
    (m) =>
      (!query || m.name.toLowerCase().includes(query) || m.email.toLowerCase().includes(query)) &&
      (!search.role || m.role === search.role) &&
      (!search.ledger || m.role === 'admin' || m.role === 'owner' || m.ledgers.some((l) => l.ledger === search.ledger))
  )

  const memberColumns = useMemo(
    () => [
      column.accessor('name', {
        cell: ({ row }) => (
          <Link
            to="/$org/members/$member"
            params={{ member: row.original.subject, org }}
            className="outline-none focus-visible:ring-3 focus-visible:ring-focus-ring"
          >
            <PersonCell name={row.original.name} email={row.original.email} you={row.original.subject === session?.subject} />
          </Link>
        ),
        header: 'Member'
      }),
      column.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Organization role' }),
      column.display({
        cell: ({ row }) => <LedgerRoles member={row.original} />,
        header: 'Ledger roles',
        id: 'ledgers',
        meta: { className: 'w-full' }
      }),
      column.accessor('addedAt', { cell: ({ row }) => <AddedCell at={row.original.addedAt} by={row.original.addedBy} />, header: 'Added' })
    ],
    [org, session?.subject]
  )

  return (
    <Page>
      <PageHeader
        title="Members"
        description="People in this organization and the roles they hold. Passwords, MFA and single sign-on are managed in Keycloak."
        actions={
          <>
            <Button variant="secondary" render={<Link to="/$org/settings/roles" params={{ org }} />}>
              <ShieldCheckIcon />
              Roles and permissions
            </Button>
            {manage ? (
              <Button onClick={() => setSearch({ invite: true })}>
                <UserPlusIcon />
                Invite members
              </Button>
            ) : null}
          </>
        }
      />
      <Tabs value={tab} onValueChange={(value) => setSearch({ tab: value === 'members' ? undefined : (value as 'invitations') })}>
        <TabsList>
          <TabsTrigger value="members">
            Members <TabsCount>{active.length}</TabsCount>
          </TabsTrigger>
          <TabsTrigger value="invitations">
            Invitations <TabsCount>{pending.length}</TabsCount>
          </TabsTrigger>
        </TabsList>
      </Tabs>
      {members.error ? <ProblemPanel error={members.error} onRetry={() => void members.refetch()} /> : null}
      {tab === 'members' ? (
        <>
          <FilterBar>
            <SearchFilter label="Name or email" placeholder="Search name or email" mono={false} value={search.q} onChange={(q) => setSearch({ q })} />
            <SelectFilter
              label="Organization role"
              allLabel="All organization roles"
              value={search.role}
              options={ORGANIZATION_ROLES.map((role) => ({ label: role, value: role }))}
              onChange={(role) => setSearch({ role })}
            />
            <SelectFilter
              label="Ledger"
              allLabel="Any ledger"
              mono
              value={search.ledger}
              options={(ledgers.data?.data ?? []).map((l) => ({ label: l.name, value: l.name }))}
              onChange={(ledger) => setSearch({ ledger })}
            />
          </FilterBar>
          <DataTable
            label="Members"
            columns={memberColumns}
            data={members.data ? shown : undefined}
            getRowId={(m) => m.subject}
            loading={members.isPending}
            onRowClick={(m) => void goTo({ params: { member: m.subject, org }, to: '/$org/members/$member' })}
            empty="No member matches these filters."
          />
          <Hint>
            Removing a member stops their access when their session renews, within 5 minutes. Their Keycloak account and other organizations are kept.
          </Hint>
        </>
      ) : (
        <InvitationsTable members={members.data ? pending : undefined} loading={members.isPending} manage={manage} />
      )}
      {manage ? <InviteDialog open={search.invite === true} onOpenChange={(open) => setSearch({ invite: open || undefined })} /> : null}
    </Page>
  )
}

function InvitationsTable({ members, loading, manage }: { members: Member[] | undefined; loading: boolean; manage: boolean }) {
  const resend = useResendInvitation()
  const invite = useInviteMember()
  const revoke = useDeleteInvitation()
  const remove = useRemoveMember()
  const error = resend.error ?? invite.error ?? revoke.error ?? remove.error

  const columns = useMemo(
    () => [
      column.accessor('email', { cell: (info) => <span className="text-body text-foreground">{info.getValue()}</span>, header: 'Email' }),
      column.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Organization role' }),
      column.display({
        cell: ({ row }) => <LedgerRoles member={row.original} />,
        header: 'Ledger roles',
        id: 'ledgers',
        meta: { className: 'w-full' }
      }),
      column.accessor('addedAt', { cell: ({ row }) => <AddedCell at={row.original.addedAt} by={row.original.addedBy} />, header: 'Invited' }),
      column.display({
        cell: ({ row }) => <InvitationStatus member={row.original} />,
        header: 'Status',
        id: 'status'
      }),
      column.display({
        cell: ({ row }) => {
          const m = row.original
          if (!manage) {
            return null
          }
          // An invitation is Keycloak's to resend and withdraw. An account the platform created
          // gets its setup email again through a new invitation, and is withdrawn by removing it.
          const onResend = () =>
            m.invitation
              ? resend.mutate(m.invitation.id, { onSuccess: () => toast.success(`Invitation sent again to ${m.email}`) })
              : invite.mutate(
                  { email: m.email, ledgers: [], role: m.role ?? 'member' },
                  { onSuccess: () => toast.success(`Setup email sent again to ${m.email}`) }
                )
          const onRevoke = () =>
            m.invitation
              ? revoke.mutate(m.invitation.id, { onSuccess: () => toast.success(`Invitation to ${m.email} withdrawn`) })
              : remove.mutate(m.subject, { onSuccess: () => toast.success(`Invitation to ${m.email} withdrawn`) })
          return (
            <span className="flex gap-1">
              <Button size="sm" variant="ghost" onClick={onResend}>
                Resend
              </Button>
              <Button size="sm" variant="ghost" className="text-danger-fg" onClick={onRevoke}>
                Revoke
              </Button>
            </span>
          )
        },
        header: '',
        id: 'actions'
      })
    ],
    [manage, resend, invite, revoke, remove]
  )
  return (
    <>
      {error ? <ProblemPanel error={error} /> : null}
      <DataTable
        label="Invitations"
        columns={columns}
        data={members}
        getRowId={(m) => m.subject}
        loading={loading}
        empty="No invitation is waiting."
      />
      <Hint>Invitations are stored in Keycloak. Revoking one also drops the roles waiting for that person.</Hint>
    </>
  )
}

function InvitationStatus({ member }: { member: Member }) {
  if (member.status === 'setup-pending') {
    return (
      <span className="flex flex-col">
        <Badge variant="warning">setting up</Badge>
        <span className="text-caption text-muted-foreground">account created, password not set</span>
      </span>
    )
  }
  const { invitation } = member
  if (!invitation) {
    return <Badge variant="warning">pending</Badge>
  }
  return (
    <span className="flex flex-col gap-0.5">
      {invitation.expired ? <Badge variant="danger">expired</Badge> : <Badge variant="warning">pending</Badge>}
      <span className="text-caption text-muted-foreground">
        {invitation.expired ? 'on ' : 'expires '}
        <Timestamp value={invitation.expiresAt} />
      </span>
    </span>
  )
}
