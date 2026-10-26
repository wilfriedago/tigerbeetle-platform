import { Link } from '@tanstack/react-router'

import { ExternalLinkIcon, LogOutIcon } from 'lucide-react'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { Page, SectionTitle } from '~/components/layout/page'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Card, CardContent } from '~/components/ui/card'

import { Avatar, LedgerRoles, RoleBadge } from '~/features/members/components'

import { type SessionOrganization, signOut } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

// .design/auth.pen, Profile: who is signed in, where they are a member and what they can do there.
// The account itself (password, two-factor, sessions) is Keycloak's, one link away.

const column = columnHelper<SessionOrganization>()

export function ProfilePage() {
  const session = useSession()
  const current = useOrganization()
  if (!session) {
    return <Page />
  }
  const name = session.name || session.email || session.subject
  const columns = [
    column.accessor('name', {
      cell: ({ row }) => (
        <span className="flex items-center gap-2.5">
          <Avatar name={row.original.name} className="rounded-md" />
          <span className="flex flex-col">
            <span className="text-body text-foreground">{row.original.name}</span>
            <span className="font-mono text-caption text-muted-foreground">{row.original.alias}</span>
          </span>
        </span>
      ),
      header: 'Organization'
    }),
    column.accessor('role', { cell: (info) => <RoleBadge role={info.getValue()} />, header: 'Organization role' }),
    column.display({
      cell: ({ row }) => (
        <LedgerRoles
          member={{ ledgers: Object.entries(row.original.ledgers).map(([ledger, role]) => ({ ledger, role })), role: row.original.role }}
        />
      ),
      header: 'Ledger roles',
      id: 'ledgers',
      meta: { className: 'w-full' }
    }),
    column.display({
      cell: ({ row }) =>
        row.original.alias === current ? (
          <Badge variant="accent" dot={false}>
            current
          </Badge>
        ) : (
          <Button size="sm" variant="secondary" render={<Link to="/$org/ledgers" params={{ org: row.original.alias }} />}>
            Open
          </Button>
        ),
      header: '',
      id: 'open'
    })
  ]
  return (
    <Page>
      <Card>
        <CardContent className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <Avatar name={name} className="size-12 text-body" />
            <div className="flex flex-col gap-0.5">
              <h1 className="flex items-center gap-2 text-page-title text-foreground">
                {name}
                {session.staff ? <Badge variant="accent">platform staff</Badge> : null}
              </h1>
              {session.email ? <p className="text-small text-muted-foreground">{session.email}</p> : null}
            </div>
          </div>
          <div className="flex gap-2">
            {session.accountUrl ? (
              <Button
                variant="secondary"
                render={<a href={session.accountUrl} target="_blank" rel="noreferrer" aria-label="Manage account in Keycloak (opens in a new tab)" />}
              >
                Manage account in Keycloak
                <ExternalLinkIcon />
              </Button>
            ) : null}
            <Button variant="secondary" onClick={() => void signOut()}>
              <LogOutIcon />
              Sign out
            </Button>
          </div>
        </CardContent>
      </Card>
      <section className="flex flex-col gap-3">
        <SectionTitle title="Your organizations" description="Where you are a member and what you can do there." />
        <DataTable
          label="Your organizations"
          columns={columns}
          data={session.organizations}
          getRowId={(o) => o.alias}
          empty="You are not in an organization yet."
        />
      </section>
      <section className="flex flex-col gap-3">
        <SectionTitle title="Sign-in and security" description="Kept by Keycloak: password, two-factor and passkeys, active sessions." />
        <p className="text-small text-muted-foreground">
          This console session renews your access every few minutes, automatically. Signing out ends it here and in Keycloak.
        </p>
      </section>
    </Page>
  )
}
