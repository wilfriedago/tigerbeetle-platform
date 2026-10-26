import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'

import { PlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, PageHeader, SectionTitle } from '~/components/layout/page'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card'

import { canManageLedgerAccess } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { memberQueries, useRevokeLedgerRole } from './api'
import { PersonCell, RoleBadge } from './components'
import { GrantLedgerRoleDialog } from './grant-dialog'
import { LEDGER_ROLES, ledgerRoleText } from './roles'

// .design/auth.pen, Ledger access: who can use one ledger, from the members list every member may
// read (ADR-0028). Ledger roles are granted on this ledger only; organization admins and owners can
// do everything on every ledger and are listed apart. Ledger admins grant and revoke here.

const route = getRouteApi('/_console/$org/ledgers/$ledger/settings/access')

interface AccessRow {
  subject: string
  name: string
  email: string
  role: string
  grantedAt?: string | undefined
  grantedBy?: string | undefined
}

const column = columnHelper<AccessRow>()

export function LedgerAccessPage() {
  const org = useOrganization()
  const { ledger } = route.useParams()
  const session = useSession()
  const members = useQuery(memberQueries.list())
  const revoke = useRevokeLedgerRole()
  const [granting, setGranting] = useState(false)
  const manage = canManageLedgerAccess(session, ledger)

  const granted: AccessRow[] = []
  const throughOrganization: AccessRow[] = []
  for (const m of members.data ?? []) {
    if (m.role === 'admin' || m.role === 'owner') {
      throughOrganization.push({ email: m.email, name: m.name, role: m.role, subject: m.subject })
      continue
    }
    const role = m.ledgers.find((l) => l.ledger === ledger)
    if (role) {
      granted.push({ email: m.email, grantedAt: role.grantedAt, grantedBy: role.grantedBy, name: m.name, role: role.role, subject: m.subject })
    }
  }

  const who = column.accessor('name', {
    cell: ({ row }) => (
      <Link to="/$org/members/$member" params={{ member: row.original.subject, org }}>
        <PersonCell name={row.original.name} email={row.original.email} you={row.original.subject === session?.subject} />
      </Link>
    ),
    header: 'Who',
    meta: { className: 'w-full' }
  })
  const grantedColumns = [
    who,
    column.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Role' }),
    column.accessor('grantedAt', {
      cell: ({ row }) =>
        row.original.grantedAt ? (
          <span className="flex flex-col">
            <Timestamp value={row.original.grantedAt} />
            <span className="text-caption text-muted-foreground">{row.original.grantedBy ?? 'platform staff'}</span>
          </span>
        ) : null,
      header: 'Granted'
    }),
    column.display({
      cell: ({ row }) =>
        manage ? (
          <Button
            size="sm"
            variant="ghost"
            className="text-danger-fg"
            loading={revoke.isPending && revoke.variables?.subject === row.original.subject}
            onClick={() =>
              revoke.mutate(
                { ledger, subject: row.original.subject },
                { onSuccess: () => toast.success(`${row.original.name || row.original.email} no longer has a role on ${ledger}`) }
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
  const organizationColumns = [
    who,
    column.accessor('role', {
      cell: (info) => (
        <span className="flex items-center gap-2">
          <RoleBadge role={info.getValue()} />
          <span className="text-caption text-muted-foreground">every permission on this ledger</span>
        </span>
      ),
      header: 'Organization role'
    })
  ]

  return (
    <Page>
      <PageHeader
        title="Access"
        description={
          <>
            Who can use <span className="font-mono">{ledger}</span>. A revoked role stops working on the next request.
          </>
        }
        actions={
          manage ? (
            <Button onClick={() => setGranting(true)}>
              <PlusIcon />
              Grant access
            </Button>
          ) : null
        }
      />
      {members.error ? <ProblemPanel error={members.error} onRetry={() => void members.refetch()} /> : null}
      {revoke.error ? <ProblemPanel error={revoke.error} /> : null}
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex flex-col gap-6">
          <section className="flex flex-col gap-3">
            <SectionTitle title={`Ledger roles · ${granted.length}`} description={`Granted on ${ledger} only.`} />
            <DataTable
              label="Ledger roles"
              columns={grantedColumns}
              data={members.data ? granted : undefined}
              getRowId={(r) => r.subject}
              loading={members.isPending}
              empty="Nobody holds a role on this ledger yet."
            />
          </section>
          <section className="flex flex-col gap-3">
            <SectionTitle
              title={`Through the organization role · ${throughOrganization.length}`}
              description="Owners and admins can do everything on every ledger. Change it in Members."
            />
            <DataTable
              label="Organization admins and owners"
              columns={organizationColumns}
              data={members.data ? throughOrganization : undefined}
              getRowId={(r) => r.subject}
              loading={members.isPending}
            />
          </section>
        </div>
        <aside>
          <Card>
            <CardHeader>
              <CardTitle>Ledger roles</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              {LEDGER_ROLES.map((r) => (
                <div key={r} className="flex flex-col gap-0.5">
                  <span className="font-mono text-body text-foreground">{r}</span>
                  <span className="text-small text-muted-foreground">{ledgerRoleText[r].detail}</span>
                </div>
              ))}
              <Link to="/$org/settings/roles" params={{ org }} className="text-small text-primary-text hover:underline">
                Every role and permission
              </Link>
            </CardContent>
          </Card>
        </aside>
      </div>
      <GrantLedgerRoleDialog open={granting} onOpenChange={setGranting} ledger={ledger} />
    </Page>
  )
}
