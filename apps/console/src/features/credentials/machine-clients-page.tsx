import { useMemo, useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'

import { type MachineClient } from '@tigerbeetle-platform/api-client'
import { InfoIcon, PlusIcon } from 'lucide-react'
import { toast } from 'sonner'

import { columnHelper, DataTable } from '~/components/data-table/data-table'
import { FilterBar, SearchFilter, SelectFilter } from '~/components/data-table/filters'
import { Page, PageHeader } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Field, FieldDescription, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'
import { Sheet, SheetBody, SheetContent, SheetFooter, SheetHeader, SheetTitle } from '~/components/ui/sheet'

import { LedgerRoles } from '~/features/members/components'
import { chosenLedgerRoles, type LedgerRoleRow, LedgerRoleRows } from '~/features/members/ledger-role-rows'

import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { credentialQueries, useCreateMachineClient } from './api'
import { CreatedCell, CREDENTIAL_NAME_PATTERN, CredentialStatus, SecretOnceDialog } from './components'

// .design/auth.pen, Machine clients: OAuth 2.0 clients in Keycloak that belong to the organization
// (ADR-0029). They trade their secret for a five-minute token and act with the roles granted here.

const route = getRouteApi('/_console/$org/machine-clients/')

const column = columnHelper<MachineClient>()

export function MachineClientsPage() {
  const org = useOrganization()
  const session = useSession()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const goTo = useNavigate()
  const clients = useQuery(credentialQueries.machineClients())
  const setSearch = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch }) })
  const name = session?.organizations.find((o) => o.alias === org)?.name ?? org
  const q = search.q?.toLowerCase()
  const rows = clients.data?.filter((c) => (!q || c.name.includes(q) || c.clientId.includes(q)) && (!search.status || c.status === search.status))

  const columns = useMemo(
    () => [
      column.accessor('name', {
        cell: ({ row }) => (
          <Link to="/$org/machine-clients/$client" params={{ client: row.original.clientId, org }} className="flex flex-col">
            <span className="flex items-center gap-2 text-body text-foreground">
              {row.original.name}
              {row.original.status === 'active' ? null : <CredentialStatus status={row.original.status} />}
            </span>
            <span className="font-mono text-caption text-muted-foreground">client_id {row.original.clientId}</span>
          </Link>
        ),
        header: 'Client'
      }),
      column.display({ cell: ({ row }) => <LedgerRoles member={row.original} />, header: 'Roles', id: 'roles', meta: { className: 'w-full' } }),
      column.accessor('createdAt', { cell: ({ row }) => <CreatedCell at={row.original.createdAt} by={row.original.createdBy} />, header: 'Created' }),
      column.accessor('lastTokenAt', {
        cell: (info) =>
          info.getValue() ? <Timestamp value={info.getValue() ?? ''} relative /> : <span className="text-subtle-foreground">no token yet</span>,
        header: 'Last token'
      }),
      column.accessor('secretSetAt', { cell: (info) => <Timestamp value={info.getValue()} />, header: 'Secret set' })
    ],
    [org]
  )

  return (
    <Page>
      <PageHeader
        title="Machine clients"
        description={`OAuth 2.0 clients in Keycloak that belong to ${name}. They get short-lived tokens with client credentials and act with the roles granted here.`}
        actions={
          <Button onClick={() => setSearch({ create: true })}>
            <PlusIcon />
            Create machine client
          </Button>
        }
      />
      <FilterBar>
        <SearchFilter
          label="Name or client ID"
          placeholder="Search name or client ID"
          mono={false}
          value={search.q}
          onChange={(value) => setSearch({ q: value })}
        />
        <SelectFilter
          label="Status"
          allLabel="All statuses"
          value={search.status}
          options={['active', 'deleted'].map((s) => ({ label: s, value: s }))}
          onChange={(status) => setSearch({ status: status as 'active' | 'deleted' | undefined })}
        />
      </FilterBar>
      {clients.error ? <ProblemPanel error={clients.error} onRetry={() => void clients.refetch()} /> : null}
      <DataTable
        label="Machine clients"
        columns={columns}
        data={rows}
        getRowId={(c) => c.clientId}
        loading={clients.isPending}
        onRowClick={(c) => void goTo({ params: { client: c.clientId, org }, to: '/$org/machine-clients/$client' })}
        empty="No machine client yet."
      />
      <Hint>Tokens come from Keycloak and last 5 minutes. Deleting a client stops new tokens at once.</Hint>
      <CreateClientSheet
        organization={name}
        alias={org}
        open={search.create === true}
        onOpenChange={(open) => setSearch({ create: open || undefined })}
      />
    </Page>
  )
}

interface Created {
  client: MachineClient
  secret: string
  tokenUrl: string
}

function CreateClientSheet({
  organization,
  alias,
  open,
  onOpenChange
}: {
  organization: string
  alias: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const create = useCreateMachineClient()
  const [name, setName] = useState('')
  const [rows, setRows] = useState<LedgerRoleRow[]>([{ id: 1, ledger: '', role: 'viewer' }])
  const [created, setCreated] = useState<Created | undefined>()
  const ledgers = chosenLedgerRoles(rows)
  const valid = CREDENTIAL_NAME_PATTERN.test(name) && ledgers.length > 0
  const reset = () => {
    setName('')
    setRows([{ id: 1, ledger: '', role: 'viewer' }])
    setCreated(undefined)
    create.reset()
  }
  return (
    <>
      <Sheet
        open={open && !created}
        onOpenChange={(next) => {
          onOpenChange(next)
          if (!next) {
            reset()
          }
        }}
      >
        <SheetContent className="sm:max-w-[480px]">
          <form
            noValidate
            className="contents"
            onSubmit={(event) => {
              event.preventDefault()
              if (valid) {
                create.mutate({ ledgers, name }, { onSuccess: setCreated })
              }
            }}
          >
            <SheetHeader>
              <SheetTitle>Create machine client</SheetTitle>
              <p className="text-small text-muted-foreground">{organization}</p>
            </SheetHeader>
            <SheetBody className="flex flex-col gap-5">
              <Field>
                <FieldLabel htmlFor="client-name">Name</FieldLabel>
                <Input
                  id="client-name"
                  className="font-mono"
                  autoComplete="off"
                  placeholder="refunds-worker"
                  value={name}
                  aria-invalid={(name.length > 0 && !CREDENTIAL_NAME_PATTERN.test(name)) || undefined}
                  onChange={(event) => setName(event.target.value)}
                />
                <FieldDescription>
                  The client ID becomes <span className="font-mono">{`${alias}-${name || 'name'}`}</span> in Keycloak.
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel>
                  Roles <span className="font-normal text-subtle-foreground">at least one</span>
                </FieldLabel>
                <LedgerRoleRows rows={rows} onChange={setRows} />
              </Field>
              <Alert>
                <InfoIcon />
                <AlertTitle>API key or machine client?</AlertTitle>
                <AlertDescription>
                  A machine client never sends a long-lived secret to the API: it trades its secret for a 5 minute token at Keycloak. Prefer it when
                  your platform supports OAuth 2.0 client credentials.
                </AlertDescription>
              </Alert>
              {create.error ? <ProblemPanel error={create.error} /> : null}
            </SheetBody>
            <SheetFooter>
              <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!valid} loading={create.isPending}>
                Create client
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
      {created ? (
        <SecretOnceDialog
          open
          onDone={() => {
            toast.success(`${created.client.name} is ready`)
            onOpenChange(false)
            reset()
          }}
          title={`${created.client.name} is ready`}
          description="Store the client secret now. Keycloak shows it again only to a realm administrator; from here, regenerate it if it is lost."
          secretLabel="Client secret"
          secret={created.secret}
          fields={[
            { label: 'Client ID', mono: true, value: created.client.clientId },
            { label: 'Token URL', mono: true, value: created.tokenUrl },
            { label: 'Roles', value: created.client.ledgers.map((l) => `${l.role} on ${l.ledger}`).join(', ') }
          ]}
          example={`curl ${created.tokenUrl} \\\n  -d grant_type=client_credentials \\\n  -u ${created.client.clientId}:$CLIENT_SECRET`}
          confirmLabel="I have stored the client secret somewhere safe"
        />
      ) : null}
    </>
  )
}
