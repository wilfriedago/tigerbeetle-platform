import { Link } from '@tanstack/react-router'

import { CheckIcon } from 'lucide-react'

import { Page, PageHeader } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { Button } from '~/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'

import { useOrganization } from '~/lib/organization'

import { LEDGER_ROLES, ORGANIZATION_ROLES, PERMISSIONS } from './roles'

// .design/auth.pen, Roles and permissions: the fixed role table of ADR-0028, the same one the use
// cases check. Read-only: roles are granted in Members, or on a ledger's Access page.

/** The six role columns: three ledger roles, then three organization roles (admin is in both). */
const COLUMNS = [
  ...LEDGER_ROLES.map((role) => ({ key: `ledger-${role}`, label: `${role} on a ledger`, role })),
  ...ORGANIZATION_ROLES.map((role) => ({ key: `organization-${role}`, label: `organization ${role}`, role }))
]

function Held({ held, label }: { held: boolean; label: string }) {
  return held ? (
    <CheckIcon aria-label={`${label}: yes`} className="mx-auto size-4 text-success-fg" />
  ) : (
    <span aria-label={`${label}: no`} className="block text-center text-subtle-foreground">
      ·
    </span>
  )
}

export function RolesPage() {
  const org = useOrganization()
  return (
    <Page>
      <PageHeader
        title="Roles and permissions"
        description="The platform has a fixed set of roles. Grant them in Members, or on a ledger's Access page."
        actions={
          <Button variant="secondary" render={<Link to="/$org/members" params={{ org }} />}>
            Members
          </Button>
        }
      />
      <div className="overflow-x-auto rounded-md border border-border">
        <Table aria-label="Roles and permissions">
          <TableHeader>
            <TableRow>
              <TableHead rowSpan={2} className="w-full align-bottom">
                Permission
              </TableHead>
              <TableHead colSpan={3} className="text-center">
                On one ledger
              </TableHead>
              <TableHead colSpan={3} className="text-center">
                On the organization
              </TableHead>
            </TableRow>
            <TableRow>
              {COLUMNS.map((c) => (
                <TableHead key={c.key} className="text-center font-mono">
                  {c.role}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell>
                <span className="block text-foreground">See the organization</span>
                <span className="block text-caption text-muted-foreground">Its name, members list and your own roles. Every member, implicitly.</span>
              </TableCell>
              {COLUMNS.map((c) => (
                <TableCell key={c.key}>
                  <Held held label={c.label} />
                </TableCell>
              ))}
            </TableRow>
            {PERMISSIONS.map((p) => (
              <TableRow key={p.name}>
                <TableCell>
                  <span className="block font-mono text-foreground">{p.name}</span>
                  <span className="block text-caption text-muted-foreground">{p.description}</span>
                </TableCell>
                {LEDGER_ROLES.map((role) => (
                  <TableCell key={role}>
                    <Held held={p.ledger[role]} label={`${role} on a ledger`} />
                  </TableCell>
                ))}
                {ORGANIZATION_ROLES.map((role) => (
                  <TableCell key={role}>
                    <Held held={p.organization[role]} label={`organization ${role}`} />
                  </TableCell>
                ))}
              </TableRow>
            ))}
            <TableRow>
              <TableCell>
                <span className="block text-foreground">Grant or remove owner</span>
                <span className="block text-caption text-muted-foreground">An organization always keeps at least one owner.</span>
              </TableCell>
              {COLUMNS.map((c) => (
                <TableCell key={c.key}>
                  <Held held={c.key === 'organization-owner'} label={c.label} />
                </TableCell>
              ))}
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <Hint>
        Platform staff hold only platform.organizations. They see an organization's ledgers only when it grants them a role here, like anyone else.
      </Hint>
    </Page>
  )
}
