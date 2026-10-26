import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'

import { LogOutIcon, ShieldCheckIcon, TriangleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Page, PageHeader, SectionTitle } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Stat } from '~/components/ledger/stat'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
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
import { Card, CardContent } from '~/components/ui/card'

import { ledgerQueries } from '~/features/ledgers/api'
import { memberQueries, useRemoveMember } from '~/features/members/api'
import { PersonCell } from '~/features/members/components'

import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

// .design/auth.pen, Organization settings: the organization as Keycloak and the platform know it,
// its owners, and leaving it. Name and sign-in settings are platform staff's; owners are managed
// in Members.

export function OrganizationSettingsPage() {
  const org = useOrganization()
  const session = useSession()
  const members = useQuery(memberQueries.list())
  const ledgers = useQuery(ledgerQueries.list())
  const [leaving, setLeaving] = useState(false)
  const organization = session?.organizations.find((o) => o.alias === org)
  const owners = (members.data ?? []).filter((m) => m.role === 'owner')
  const onlyOwner = session?.role === 'owner' && owners.length === 1
  const active = (members.data ?? []).filter((m) => m.status === 'active').length
  const pending = (members.data ?? []).length - active

  return (
    <Page>
      <PageHeader
        title="Organization settings"
        description="This organization as Keycloak and the platform know it. Its name and sign-in settings are managed by platform staff."
        actions={
          <Button variant="secondary" render={<Link to="/$org/settings/roles" params={{ org }} />}>
            <ShieldCheckIcon />
            Roles and permissions
          </Button>
        }
      />
      {members.error ? <ProblemPanel error={members.error} onRetry={() => void members.refetch()} /> : null}
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex flex-col gap-6">
          <section className="flex flex-col gap-3">
            <SectionTitle title="General" />
            <Card>
              <CardContent>
                <DescriptionList>
                  <DescriptionItem term="Name">
                    {organization?.name} <span className="text-caption text-muted-foreground">changed by platform staff</span>
                  </DescriptionItem>
                  <DescriptionItem term="Alias">
                    <span className="font-mono">{org}</span>{' '}
                    <span className="text-caption text-muted-foreground">permanent, used in URLs and X-Organization</span>
                  </DescriptionItem>
                  <DescriptionItem term="Status">
                    <Badge variant="success">active</Badge>
                  </DescriptionItem>
                  <DescriptionItem term="Your role">
                    <span className="font-mono">{session?.role ?? 'none'}</span>
                  </DescriptionItem>
                </DescriptionList>
              </CardContent>
            </Card>
          </section>
          <section className="flex flex-col gap-3">
            <SectionTitle title="Owners" description="Owners can grant and remove owner. An organization always keeps at least one." />
            <Card>
              <CardContent className="flex flex-col gap-3">
                {owners.map((o) => (
                  <Link key={o.subject} to="/$org/members/$member" params={{ member: o.subject, org }}>
                    <PersonCell name={o.name} email={o.email} you={o.subject === session?.subject} />
                  </Link>
                ))}
                {session?.role === 'owner' ? (
                  <Link to="/$org/members" params={{ org }} className="text-small text-primary-text hover:underline">
                    Add an owner in Members
                  </Link>
                ) : null}
              </CardContent>
            </Card>
            {onlyOwner ? (
              <Alert variant="warning">
                <TriangleAlertIcon />
                <AlertTitle>You are the only owner</AlertTitle>
                <AlertDescription>You cannot leave or step down to admin until another member is an owner.</AlertDescription>
              </Alert>
            ) : null}
          </section>
          <section className="flex flex-col gap-3">
            <SectionTitle title="Leave the organization" description="Your Keycloak account and other organizations are kept." />
            <div>
              <Button variant="secondary" className="text-danger-fg" disabled={onlyOwner} onClick={() => setLeaving(true)}>
                <LogOutIcon />
                Leave organization
              </Button>
            </div>
          </section>
        </div>
        <aside className="flex flex-col gap-3">
          <SectionTitle title="In this organization" />
          <Card>
            <CardContent className="grid grid-cols-2 gap-4">
              <Stat label="Ledgers" value={ledgers.data ? String(ledgers.data.data.length) : '...'} />
              <Stat label="Members" value={members.data ? String(active) : '...'} />
              <Stat label="Pending invitations" value={members.data ? String(pending) : '...'} />
            </CardContent>
          </Card>
        </aside>
      </div>
      <LeaveDialog open={leaving} onOpenChange={setLeaving} subject={session?.subject ?? ''} name={organization?.name ?? org} />
    </Page>
  )
}

function LeaveDialog({ open, onOpenChange, subject, name }: { open: boolean; onOpenChange: (open: boolean) => void; subject: string; name: string }) {
  const remove = useRemoveMember()
  const navigate = useNavigate()
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Leave {name}?</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogBody>
          <AlertDialogDescription>
            Your roles here are removed at once and the membership ends in Keycloak. Your account and other organizations are kept. To come back, an
            admin invites you again.
          </AlertDialogDescription>
          {remove.error ? <ProblemPanel error={remove.error} /> : null}
        </AlertDialogBody>
        <AlertDialogFooter>
          <AlertDialogCancel>Stay</AlertDialogCancel>
          <Button
            variant="destructive"
            loading={remove.isPending}
            onClick={() =>
              remove.mutate(subject, {
                onSuccess: () => {
                  toast.success(`You left ${name}`)
                  onOpenChange(false)
                  void navigate({ to: '/' })
                }
              })
            }
          >
            Leave
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
