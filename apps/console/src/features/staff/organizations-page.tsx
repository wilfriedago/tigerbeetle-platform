import { useMemo, useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'

import { type Organization } from '@tigerbeetle-platform/api-client'
import { InfoIcon, PlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter, SelectFilter } from '~/components/data-table/filters'
import { useCursorStack } from '~/components/data-table/use-cursor-stack'
import { Page, PageHeader } from '~/components/layout/page'
import { CursorPager } from '~/components/ledger/cursor-pager'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Dialog, DialogBody, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'

import { Avatar } from '~/features/members/components'
import { EMAIL_PATTERN } from '~/features/members/invite-dialog'

import { staffQueries, useCreateOrganization } from './api'

// .design/auth.pen, Staff: organizations. Every tenant of the platform with counts only: staff
// create, rename and suspend organizations, and read a tenant's ledgers only when it grants them a
// role (ADR-0027, ADR-0028).

const route = getRouteApi('/_console/staff/organizations/')

/** Mirrors the organizations_alias_format check of migration 00009. */
export const ALIAS_PATTERN = /^[a-z0-9][a-z0-9-]{0,62}$/

/** "Initech Corp" becomes "initech-corp". */
export function suggestAlias(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replaceAll(/[^a-z0-9]+/g, '-')
    .replaceAll(/^-+|-+$/g, '')
    .slice(0, 63)
}

const column = columnHelper<Organization>()

export function StatusBadge({ status }: { status: string }) {
  return <Badge variant={status === 'active' ? 'success' : 'danger'}>{status}</Badge>
}

function OwnerCell({ organization }: { organization: Organization }) {
  const [owner] = organization.owners
  if (!owner) {
    return <span className="text-caption text-subtle-foreground">{organization.alias === 'default' ? 'pre-alpha data' : 'no owner'}</span>
  }
  return (
    <span className="flex flex-col">
      <span className="flex items-center gap-1.5 text-body text-foreground">
        {owner.email}
        {organization.owners.length > 1 ? <span className="text-caption text-subtle-foreground">+{organization.owners.length - 1}</span> : null}
      </span>
      <span className="text-caption text-muted-foreground">
        {owner.status === 'active' ? 'owner since ' : owner.status === 'invited' ? 'invitation sent ' : 'account setup sent '}
        <Timestamp value={owner.since} />
      </span>
    </span>
  )
}

export function StaffOrganizationsPage() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const goTo = useNavigate()
  const list = useQuery(staffQueries.list(search.cursor))
  const setSearch = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch }) })
  const pager = useCursorStack({
    cursor: search.cursor,
    next: list.data?.next,
    onChange: (cursor) => void navigate({ search: (prev) => ({ ...prev, cursor }) }),
    scope: ''
  })
  const query = search.q?.toLowerCase()
  const rows = list.data?.data.filter(
    (o) => (!query || o.name.toLowerCase().includes(query) || o.alias.includes(query)) && (!search.status || o.status === search.status)
  )

  const columns = useMemo(
    () => [
      column.accessor('name', {
        cell: ({ row }) => (
          <Link to="/staff/organizations/$alias" params={{ alias: row.original.alias }} className="flex items-center gap-2.5">
            <Avatar name={row.original.name} className="rounded-md" />
            <span className="flex flex-col">
              <span className="text-body text-foreground">{row.original.name}</span>
              <span className="font-mono text-caption text-muted-foreground">{row.original.alias}</span>
            </span>
          </Link>
        ),
        header: 'Organization'
      }),
      column.accessor('status', { cell: (info) => <StatusBadge status={info.getValue()} />, header: 'Status' }),
      column.display({ cell: ({ row }) => <OwnerCell organization={row.original} />, header: 'Owner', id: 'owner', meta: { className: 'w-full' } }),
      column.accessor('members', {
        cell: (info) => <span className="font-mono tabular-nums">{info.getValue()}</span>,
        header: 'Members',
        meta: { align: 'right' }
      }),
      column.accessor('ledgers', {
        cell: (info) => <span className="font-mono tabular-nums">{info.getValue()}</span>,
        header: 'Ledgers',
        meta: { align: 'right' }
      }),
      column.accessor('createdAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Created' })
    ],
    []
  )

  return (
    <Page>
      <PageHeader
        title="Organizations"
        description="Every tenant of the platform. Staff create, rename and suspend organizations; ledger contents stay private to each one."
        actions={
          <Button onClick={() => setSearch({ new: true })}>
            <PlusIcon />
            New organization
          </Button>
        }
      />
      <FilterBar>
        <SearchFilter label="Name or alias" placeholder="Search name or alias" mono={false} value={search.q} onChange={(q) => setSearch({ q })} />
        <SelectFilter
          label="Status"
          allLabel="All statuses"
          value={search.status}
          options={[
            { label: 'active', value: 'active' },
            { label: 'suspended', value: 'suspended' }
          ]}
          onChange={(status) => setSearch({ status: status as 'active' | 'suspended' | undefined })}
        />
      </FilterBar>
      {list.error ? <ProblemPanel error={list.error} onRetry={() => void list.refetch()} /> : null}
      <DataTable
        label="Organizations"
        columns={columns}
        data={rows}
        getRowId={(o) => o.alias}
        loading={list.isPending}
        onRowClick={(o) => void goTo({ params: { alias: o.alias }, to: '/staff/organizations/$alias' })}
        empty="No organization matches these filters."
      />
      <CursorPager
        info="In alias order. 50 per page."
        hasNext={pager.hasNext}
        hasPrevious={pager.hasPrevious}
        onNext={pager.next}
        onPrevious={pager.previous}
        busy={list.isFetching}
      />
      <Hint>Counts only. Staff read an organization's ledgers only when it grants them a role, which shows in its member list.</Hint>
      <NewOrganizationDialog open={search.new === true} onOpenChange={(open) => setSearch({ new: open || undefined })} />
    </Page>
  )
}

function NewOrganizationDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">{open ? <NewOrganizationForm onDone={() => onOpenChange(false)} /> : null}</DialogContent>
    </Dialog>
  )
}

function NewOrganizationForm({ onDone }: { onDone: () => void }) {
  const navigate = useNavigate()
  const create = useCreateOrganization()
  const [name, setName] = useState('')
  const [alias, setAlias] = useState('')
  const [aliasTouched, setAliasTouched] = useState(false)
  const [email, setEmail] = useState('')
  const effectiveAlias = aliasTouched ? alias : suggestAlias(name)
  const aliasOk = ALIAS_PATTERN.test(effectiveAlias)
  const emailOk = EMAIL_PATTERN.test(email.trim())
  const canSubmit = name.trim().length > 0 && aliasOk && emailOk

  return (
    <form
      noValidate
      className="contents"
      onSubmit={(event) => {
        event.preventDefault()
        if (!canSubmit) {
          return
        }
        create.mutate(
          { alias: effectiveAlias, name: name.trim(), ownerEmail: email.trim() },
          {
            onSuccess: (org) => {
              toast.success(`${org.name} was created and its owner invited`)
              onDone()
              void navigate({ params: { alias: org.alias }, to: '/staff/organizations/$alias' })
            }
          }
        )
      }}
    >
      <DialogHeader>
        <DialogTitle>New organization</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <DialogDescription>Creates the organization in Keycloak, records it on the platform and invites its first owner by email.</DialogDescription>
        <Field>
          <FieldLabel htmlFor="org-name">Name</FieldLabel>
          <Input id="org-name" value={name} autoComplete="off" placeholder="Initech" onChange={(event) => setName(event.target.value)} />
          <FieldDescription>Shown in the console. Can be changed later.</FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="org-alias">Alias</FieldLabel>
          <Input
            id="org-alias"
            className="font-mono"
            value={effectiveAlias}
            autoComplete="off"
            spellCheck={false}
            aria-invalid={(effectiveAlias.length > 0 && !aliasOk) || undefined}
            onChange={(event) => {
              setAliasTouched(true)
              setAlias(event.target.value)
            }}
          />
          <FieldDescription>Permanent. Used in URLs and in the X-Organization header. Lowercase letters, digits and dashes.</FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="org-owner">First owner's email</FieldLabel>
          <Input
            id="org-owner"
            type="email"
            value={email}
            autoComplete="off"
            placeholder="it-admin@initech.example"
            onChange={(event) => setEmail(event.target.value)}
          />
          <FieldDescription>
            They get a Keycloak invitation. Without an account, one is created and the email links to setting a password.
          </FieldDescription>
        </Field>
        <Alert>
          <InfoIcon />
          <AlertTitle>Safe to retry</AlertTitle>
          <AlertDescription>
            If a step fails, creating it again with the same alias finishes the job instead of making a second organization.
          </AlertDescription>
        </Alert>
        {create.error ? <ProblemPanel error={create.error} /> : null}
      </DialogBody>
      <DialogFooter>
        <DialogClose render={<Button type="button" variant="secondary" />}>Cancel</DialogClose>
        <Button type="submit" disabled={!canSubmit} loading={create.isPending}>
          Create and invite owner
        </Button>
      </DialogFooter>
    </form>
  )
}
