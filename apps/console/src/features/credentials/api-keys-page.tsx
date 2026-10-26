import { useMemo, useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'

import { type APIKey } from '@tigerbeetle-platform/api-client'
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
import { ToggleGroup, ToggleGroupItem } from '~/components/ui/toggle-group'

import { LedgerRoles } from '~/features/members/components'
import { chosenLedgerRoles, type LedgerRoleRow, LedgerRoleRows } from '~/features/members/ledger-role-rows'

import { useSession } from '~/lib/auth/use-session'
import { publicApiUrl } from '~/lib/curl'
import { useOrganization } from '~/lib/organization'

import { credentialQueries, useCreateApiKey } from './api'
import { CreatedCell, CREDENTIAL_NAME_PATTERN, CredentialStatus, SecretOnceDialog } from './components'

// .design/auth.pen, API keys: an organization's long-lived credentials (ADR-0029). They act with
// the ledger roles granted to them, on every transport; the secret is shown once.

const route = getRouteApi('/_console/$org/api-keys/')

const column = columnHelper<APIKey>()

export function ApiKeysPage() {
  const org = useOrganization()
  const session = useSession()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const goTo = useNavigate()
  const keys = useQuery(credentialQueries.apiKeys())
  const setSearch = (patch: Partial<typeof search>) => void navigate({ replace: true, search: (prev) => ({ ...prev, ...patch }) })
  const name = session?.organizations.find((o) => o.alias === org)?.name ?? org
  const q = search.q?.toLowerCase()
  const rows = keys.data?.filter(
    (k) => (!q || k.name.includes(q) || k.id.toLowerCase().includes(q)) && (!search.status || k.status === search.status)
  )

  const columns = useMemo(
    () => [
      column.accessor('name', {
        cell: ({ row }) => (
          <Link to="/$org/api-keys/$key" params={{ key: row.original.id, org }} className="flex flex-col">
            <span className="flex items-center gap-2 text-body text-foreground">
              {row.original.name}
              {row.original.status === 'active' ? null : <CredentialStatus status={row.original.status} />}
            </span>
            <span className="font-mono text-caption text-muted-foreground">tbp_{row.original.id}_...</span>
          </Link>
        ),
        header: 'Key'
      }),
      column.display({ cell: ({ row }) => <LedgerRoles member={row.original} />, header: 'Roles', id: 'roles', meta: { className: 'w-full' } }),
      column.accessor('createdAt', { cell: ({ row }) => <CreatedCell at={row.original.createdAt} by={row.original.createdBy} />, header: 'Created' }),
      column.accessor('lastUsedAt', {
        cell: (info) =>
          info.getValue() ? <Timestamp value={info.getValue() ?? ''} relative /> : <span className="text-subtle-foreground">never used</span>,
        header: 'Last used'
      }),
      column.accessor('expiresAt', {
        cell: (info) => (info.getValue() ? <Timestamp value={info.getValue() ?? ''} /> : <span className="text-subtle-foreground">never</span>),
        header: 'Expires'
      })
    ],
    [org]
  )

  return (
    <Page>
      <PageHeader
        title="API keys"
        description={`Keys belong to ${name} and act with the roles they are granted, on every transport. The secret is shown once.`}
        actions={
          <Button onClick={() => setSearch({ create: true })}>
            <PlusIcon />
            Create API key
          </Button>
        }
      />
      <FilterBar>
        <SearchFilter
          label="Name or key ID"
          placeholder="Search name or key ID"
          mono={false}
          value={search.q}
          onChange={(value) => setSearch({ q: value })}
        />
        <SelectFilter
          label="Status"
          allLabel="All statuses"
          value={search.status}
          options={['active', 'expired', 'revoked'].map((s) => ({ label: s, value: s }))}
          onChange={(status) => setSearch({ status: status as 'active' | 'expired' | 'revoked' | undefined })}
        />
      </FilterBar>
      {keys.error ? <ProblemPanel error={keys.error} onRetry={() => void keys.refetch()} /> : null}
      <DataTable
        label="API keys"
        columns={columns}
        data={rows}
        getRowId={(k) => k.id}
        loading={keys.isPending}
        onRowClick={(k) => void goTo({ params: { key: k.id, org }, to: '/$org/api-keys/$key' })}
        empty="No API key yet."
      />
      <Hint>
        Send a key as <span className="font-mono">Authorization: Bearer tbp_&lt;id&gt;_&lt;secret&gt;</span>. A revoked key stops working on its next
        request.
      </Hint>
      <CreateKeySheet organization={name} open={search.create === true} onOpenChange={(open) => setSearch({ create: open || undefined })} />
    </Page>
  )
}

const EXPIRIES = [
  { days: 30, label: '30 days' },
  { days: 90, label: '90 days' },
  { days: 365, label: '1 year' },
  { days: 0, label: 'Never' }
] as const

function CreateKeySheet({ organization, open, onOpenChange }: { organization: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const create = useCreateApiKey()
  const [name, setName] = useState('')
  const [days, setDays] = useState<number>(90)
  const [rows, setRows] = useState<LedgerRoleRow[]>([{ id: 1, ledger: '', role: 'viewer' }])
  const [created, setCreated] = useState<{ key: APIKey; secret: string } | undefined>()
  const ledgers = chosenLedgerRoles(rows)
  // When the sheet opened: the expiry counts from it, and render stays pure.
  const [openedAt] = useState(() => Date.now())
  const expiresAt = days > 0 ? new Date(openedAt + days * 86_400_000) : undefined
  const valid = CREDENTIAL_NAME_PATTERN.test(name) && ledgers.length > 0

  const reset = () => {
    setName('')
    setDays(90)
    setRows([{ id: 1, ledger: '', role: 'viewer' }])
    setCreated(undefined)
    create.reset()
  }
  const firstLedger = created?.key.ledgers[0]?.ledger ?? 'payments'
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
                create.mutate({ ledgers, name, ...(expiresAt ? { expiresAt: expiresAt.toISOString() } : {}) }, { onSuccess: setCreated })
              }
            }}
          >
            <SheetHeader>
              <SheetTitle>Create API key</SheetTitle>
              <p className="text-small text-muted-foreground">{organization}</p>
            </SheetHeader>
            <SheetBody className="flex flex-col gap-5">
              <Field>
                <FieldLabel htmlFor="key-name">Name</FieldLabel>
                <Input
                  id="key-name"
                  className="font-mono"
                  autoComplete="off"
                  placeholder="refunds-worker"
                  value={name}
                  aria-invalid={(name.length > 0 && !CREDENTIAL_NAME_PATTERN.test(name)) || undefined}
                  onChange={(event) => setName(event.target.value)}
                />
                <FieldDescription>Shown in grants, write requests and logs. Lowercase letters, digits and dashes.</FieldDescription>
              </Field>
              <Field>
                <FieldLabel>Expires</FieldLabel>
                <ToggleGroup value={[String(days)]} onValueChange={(value) => value[0] && setDays(Number(value[0]))} aria-label="Expires">
                  {EXPIRIES.map((e) => (
                    <ToggleGroupItem key={e.days} value={String(e.days)}>
                      {e.label}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
                <FieldDescription>
                  {expiresAt
                    ? `On ${expiresAt.toISOString().slice(0, 10)}. You can revoke it at any time before that.`
                    : 'It works until it is revoked.'}
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel>
                  Roles <span className="font-normal text-subtle-foreground">at least one</span>
                </FieldLabel>
                <LedgerRoleRows rows={rows} onChange={setRows} />
                <FieldDescription>Give a key the smallest role it needs. It can only act on the ledgers listed here.</FieldDescription>
              </Field>
              <Alert>
                <InfoIcon />
                <AlertTitle>The secret is shown once</AlertTitle>
                <AlertDescription>
                  After you create the key, copy the secret and store it in your secret manager. The platform keeps only a keyed hash of it.
                </AlertDescription>
              </Alert>
              {create.error ? <ProblemPanel error={create.error} /> : null}
            </SheetBody>
            <SheetFooter>
              <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!valid} loading={create.isPending}>
                Create key
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
      {created ? (
        <SecretOnceDialog
          open
          onDone={() => {
            toast.success(`${created.key.name} is ready`)
            onOpenChange(false)
            reset()
          }}
          title={`${created.key.name} is ready`}
          description="Copy the key now and store it in your secret manager. It is shown once; if it is lost, revoke it and create another."
          secretLabel="API key"
          secret={created.secret}
          fields={[
            { label: 'Key ID', mono: true, value: `tbp_${created.key.id}` },
            { label: 'Roles', value: created.key.ledgers.map((l) => `${l.role} on ${l.ledger}`).join(', ') },
            { label: 'Expires', value: created.key.expiresAt?.slice(0, 10) ?? 'never' }
          ]}
          example={`curl ${publicApiUrl()}/v1/ledgers/${firstLedger}/accounts \\\n  -H "Authorization: Bearer $TBP_API_KEY"`}
          confirmLabel="I have stored the key somewhere safe"
        />
      ) : null}
    </>
  )
}
