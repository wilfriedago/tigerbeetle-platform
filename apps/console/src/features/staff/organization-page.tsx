import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'

import { type Organization } from '@tigerbeetle-platform/api-client'
import { LockIcon, PauseIcon, PencilIcon, PlayIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Page, SectionTitle } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Stat, StatGroup } from '~/components/ledger/stat'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Card, CardContent } from '~/components/ui/card'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'
import { Skeleton } from '~/components/ui/skeleton'
import { Textarea } from '~/components/ui/textarea'

import { Avatar, PersonCell } from '~/features/members/components'

import { staffQueries, useUpdateOrganization } from './api'
import { StatusBadge } from './organizations-page'

// .design/auth.pen, Staff: organization. Who the organization is in Keycloak and on the platform,
// its owners, and counts; never its ledgers' contents (ADR-0027). Staff rename, suspend and
// reactivate it here.

const route = getRouteApi('/_console/staff/organizations/$alias')

export function StaffOrganizationPage() {
  const { alias } = route.useParams()
  const query = useQuery(staffQueries.one(alias))
  const [renaming, setRenaming] = useState(false)
  const [suspending, setSuspending] = useState(false)
  const update = useUpdateOrganization()
  const org = query.data

  if (query.error) {
    return (
      <Page>
        <ProblemPanel error={query.error} onRetry={() => void query.refetch()} />
      </Page>
    )
  }
  if (!org) {
    return (
      <Page>{query.isPending ? <Skeleton className="h-40" /> : <ProblemPanel error={new Error(`No organization has the alias ${alias}.`)} />}</Page>
    )
  }

  return (
    <Page>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex items-center gap-4">
          <Avatar name={org.name} className="size-12 rounded-lg text-body" />
          <div className="flex flex-col gap-1">
            <h1 className="flex items-center gap-2 text-page-title text-foreground">
              {org.name}
              <StatusBadge status={org.status} />
            </h1>
            <p className="text-small text-muted-foreground">
              <span className="font-mono">{org.alias}</span>
              {' · created '}
              <Timestamp value={org.createdAt} />
              {org.createdBy ? ` by ${org.createdBy}` : ''}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => setRenaming(true)}>
            <PencilIcon />
            Rename
          </Button>
          {org.status === 'active' ? (
            <Button variant="secondary" className="text-danger-fg" onClick={() => setSuspending(true)}>
              <PauseIcon />
              Suspend
            </Button>
          ) : (
            <Button
              variant="secondary"
              loading={update.isPending}
              onClick={() => update.mutate({ alias, status: 'active' }, { onSuccess: () => toast.success(`${org.name} is active again`) })}
            >
              <PlayIcon />
              Reactivate
            </Button>
          )}
        </div>
      </header>
      {update.error && !suspending && !renaming ? <ProblemPanel error={update.error} /> : null}
      {org.status === 'suspended' ? (
        <Alert variant="danger">
          <PauseIcon />
          <AlertTitle>Suspended</AlertTitle>
          <AlertDescription>
            Every request in this organization's name gets 403 organization-suspended.
            {org.suspensionReason ? ` Reason shown to its members: ${org.suspensionReason}` : ''}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <section className="flex flex-col gap-3">
            <SectionTitle title="Identity" />
            <Card>
              <CardContent>
                <DescriptionList>
                  <DescriptionItem term="Name">{org.name}</DescriptionItem>
                  <DescriptionItem term="Alias" mono>
                    {org.alias}
                  </DescriptionItem>
                  <DescriptionItem term="Platform ID" mono>
                    {org.id}
                  </DescriptionItem>
                  <DescriptionItem term="Keycloak organization" mono>
                    {org.keycloakId ?? 'not found in Keycloak'}
                  </DescriptionItem>
                </DescriptionList>
              </CardContent>
            </Card>
          </section>
          <section className="flex flex-col gap-3">
            <SectionTitle title="Owners" description="Owners manage the organization's members, roles and credentials." />
            <Card>
              <CardContent className="flex flex-col gap-3">
                {org.owners.length === 0 ? <p className="text-small text-muted-foreground">No owner.</p> : null}
                {org.owners.map((o) => (
                  <div key={o.subject} className="flex items-center justify-between gap-3">
                    <PersonCell name={o.name} email={o.email} />
                    <span className="text-caption text-muted-foreground">
                      {o.status === 'active' ? 'accepted ' : o.status === 'invited' ? 'invited ' : 'account setup sent '}
                      <Timestamp value={o.since} />
                    </span>
                  </div>
                ))}
              </CardContent>
            </Card>
            <Hint>Staff invite the first owner only. After that, owners invite everyone else.</Hint>
          </section>
        </div>
        <aside className="flex flex-col gap-4">
          <SectionTitle title="Counts" />
          <StatGroup className="sm:grid-flow-row">
            <Stat label="Ledgers" value={String(org.ledgers)} />
            <Stat label="Members" value={String(org.members)} />
          </StatGroup>
          <Alert>
            <LockIcon />
            <AlertTitle>Ledger contents are private</AlertTitle>
            <AlertDescription>
              You see counts, not accounts or transactions. To help with a ledger, ask an owner to grant you a role; it shows in their member list.
            </AlertDescription>
          </Alert>
        </aside>
      </div>
      <RenameDialog organization={org} open={renaming} onOpenChange={setRenaming} />
      <SuspendDialog organization={org} open={suspending} onOpenChange={setSuspending} />
    </Page>
  )
}

function RenameDialog({ organization, open, onOpenChange }: { organization: Organization; open: boolean; onOpenChange: (open: boolean) => void }) {
  const update = useUpdateOrganization()
  const [name, setName] = useState(organization.name)
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setName(organization.name)
          update.reset()
        }
      }}
    >
      <DialogContent className="sm:max-w-[440px]">
        <form
          noValidate
          className="contents"
          onSubmit={(event) => {
            event.preventDefault()
            update.mutate(
              { alias: organization.alias, name: name.trim() },
              {
                onSuccess: () => {
                  toast.success('Renamed')
                  onOpenChange(false)
                }
              }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>Rename {organization.name}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <DialogDescription>The name changes in Keycloak and in the console. The alias stays.</DialogDescription>
            <Field>
              <FieldLabel htmlFor="rename-name">Name</FieldLabel>
              <Input id="rename-name" value={name} onChange={(event) => setName(event.target.value)} />
            </Field>
            {update.error ? <ProblemPanel error={update.error} /> : null}
          </DialogBody>
          <DialogFooter>
            <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
            <Button type="submit" disabled={name.trim().length === 0 || name.trim() === organization.name} loading={update.isPending}>
              Rename
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SuspendDialog({ organization, open, onOpenChange }: { organization: Organization; open: boolean; onOpenChange: (open: boolean) => void }) {
  const update = useUpdateOrganization()
  const [reason, setReason] = useState('')
  const [confirm, setConfirm] = useState('')
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setReason('')
          setConfirm('')
          update.reset()
        }
      }}
    >
      <DialogContent className="sm:max-w-[520px]">
        <form
          noValidate
          className="contents"
          onSubmit={(event) => {
            event.preventDefault()
            update.mutate(
              { alias: organization.alias, reason: reason.trim(), status: 'suspended' },
              {
                onSuccess: () => {
                  toast.success(`${organization.name} is suspended`)
                  onOpenChange(false)
                }
              }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>Suspend {organization.name}?</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <DialogDescription>Every request in its name gets 403 organization-suspended, from the next request on.</DialogDescription>
            <ul className="flex list-disc flex-col gap-1 pl-5 text-small text-foreground">
              <li>{organization.members === 1 ? 'Its one member loses' : `Its ${organization.members} members lose`} access at once.</li>
              <li>Holds that expire meanwhile are voided by TigerBeetle as usual.</li>
              <li>Nothing is deleted: ledgers, balances, members and Keycloak stay as they are.</li>
              <li>Reactivating restores access with the same roles.</li>
            </ul>
            <Field>
              <FieldLabel htmlFor="suspend-reason">Reason, shown to the organization</FieldLabel>
              <Textarea id="suspend-reason" rows={2} maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="suspend-confirm">
                Type <span className="font-mono">{organization.alias}</span> to confirm
              </FieldLabel>
              <Input
                id="suspend-confirm"
                className="font-mono"
                autoComplete="off"
                value={confirm}
                onChange={(event) => setConfirm(event.target.value)}
              />
              <FieldDescription>Suspending stops every member, API key and machine client of the organization.</FieldDescription>
            </Field>
            {update.error ? <ProblemPanel error={update.error} /> : null}
          </DialogBody>
          <DialogFooter>
            <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
            <Button type="submit" variant="destructive" disabled={confirm !== organization.alias} loading={update.isPending}>
              Suspend organization
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
