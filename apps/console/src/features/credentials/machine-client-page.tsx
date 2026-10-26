import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'

import { BotIcon, TriangleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Page } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Alert, AlertAction, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card'
import { Skeleton } from '~/components/ui/skeleton'

import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { credentialQueries, useDeleteMachineClient, useRegenerateSecret } from './api'
import { ConfirmByName, CredentialRoles, CredentialStatus, SecretOnceDialog } from './components'
import { RecentWrites } from './recent-writes'

// .design/auth.pen, Machine client: one client's roles and details, its secret, and deleting it.

const route = getRouteApi('/_console/$org/machine-clients/$client')

/** A secret older than this gets a reminder to regenerate it. */
const SECRET_MAX_AGE_DAYS = 180

export function MachineClientPage() {
  const org = useOrganization()
  const session = useSession()
  const { client: clientId } = route.useParams()
  const clients = useQuery(credentialQueries.machineClients())
  const remove = useDeleteMachineClient()
  const regenerate = useRegenerateSecret()
  const navigate = useNavigate()
  const [deleting, setDeleting] = useState(false)
  const [secret, setSecret] = useState<string | undefined>()
  // When the page opened, for the age of the secret; render stays pure.
  const [now] = useState(() => Date.now())
  const client = clients.data?.find((c) => c.clientId === clientId)
  const orgName = session?.organizations.find((o) => o.alias === org)?.name ?? org

  if (clients.error) {
    return (
      <Page>
        <ProblemPanel error={clients.error} onRetry={() => void clients.refetch()} />
      </Page>
    )
  }
  if (!client) {
    return (
      <Page>
        {clients.isPending ? <Skeleton className="h-40" /> : <ProblemPanel error={new Error(`No machine client ${clientId} in ${org}.`)} />}
      </Page>
    )
  }
  const active = client.status === 'active'
  const secretAgeDays = Math.floor((now - Date.parse(client.secretSetAt)) / 86_400_000)
  const regenerateNow = () => regenerate.mutate(client.clientId, { onSuccess: setSecret })
  return (
    <Page>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex items-center gap-4">
          <span className="flex size-12 items-center justify-center rounded-lg border border-border bg-surface-sunken text-muted-foreground">
            <BotIcon className="size-5" />
          </span>
          <div className="flex flex-col gap-1">
            <h1 className="flex items-center gap-2 font-mono text-page-title font-medium text-foreground">
              {client.name}
              <CredentialStatus status={client.status} />
            </h1>
            <p className="text-small text-muted-foreground">
              Machine client of {orgName}, created <Timestamp value={client.createdAt} />
              {client.createdBy ? ` by ${client.createdBy}` : ''}
            </p>
          </div>
        </div>
        {active ? (
          <div className="flex gap-2">
            <Button variant="secondary" loading={regenerate.isPending} onClick={regenerateNow}>
              Regenerate secret
            </Button>
            <Button variant="secondary" className="text-danger-fg" onClick={() => setDeleting(true)}>
              Delete client
            </Button>
          </div>
        ) : null}
      </header>
      {regenerate.error ? <ProblemPanel error={regenerate.error} /> : null}
      {active && secretAgeDays > SECRET_MAX_AGE_DAYS ? (
        <Alert variant="warning">
          <TriangleAlertIcon />
          <AlertTitle>Secret older than {SECRET_MAX_AGE_DAYS} days</AlertTitle>
          <AlertDescription>Regenerate it, then deploy the new secret. The old one stops working at once.</AlertDescription>
          <AlertAction>
            <Button size="sm" variant="secondary" onClick={regenerateNow}>
              Regenerate
            </Button>
          </AlertAction>
        </Alert>
      ) : null}
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex min-w-0 flex-col gap-6">
          <CredentialRoles kind="machine-client" id={client.clientId} roles={client.ledgers} editable={active} />
          <RecentWrites actor={client.subject} ledgers={client.ledgers.map((l) => l.ledger)} what="client" />
        </div>
        <aside className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle>Details</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <DescriptionList>
                <DescriptionItem term="Client ID" mono>
                  {client.clientId}
                </DescriptionItem>
                <DescriptionItem term="Principal" mono>
                  {client.subject}
                </DescriptionItem>
                <DescriptionItem term="Secret set">
                  <Timestamp value={client.secretSetAt} />
                </DescriptionItem>
                <DescriptionItem term="Token lifetime">5 minutes</DescriptionItem>
                <DescriptionItem term="Last token">
                  {client.lastTokenAt ? <Timestamp value={client.lastTokenAt} relative /> : 'none yet'}
                </DescriptionItem>
              </DescriptionList>
              <Hint>The client and its service account live in Keycloak; the platform records which organization it belongs to.</Hint>
            </CardContent>
          </Card>
        </aside>
      </div>
      <ConfirmByName
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${client.name}?`}
        name={client.name}
        consequences={[
          'Keycloak deletes the client: it gets no new token. A token already issued works until it expires, 5 minutes at most.',
          'Deleting cannot be undone; create a new client to replace it.',
          'Its writes and grants stay, with the client as actor.'
        ]}
        action="Delete client"
        pending={remove.isPending}
        error={remove.error}
        onConfirm={() =>
          remove.mutate(client.clientId, {
            onSuccess: () => {
              toast.success(`${client.name} was deleted`)
              setDeleting(false)
              void navigate({ params: { org }, to: '/$org/machine-clients' })
            }
          })
        }
      />
      {secret ? (
        <SecretOnceDialog
          open
          onDone={() => setSecret(undefined)}
          title="New client secret"
          description="The old secret stopped working. Store the new one and deploy it."
          secretLabel="Client secret"
          secret={secret}
          fields={[{ label: 'Client ID', mono: true, value: client.clientId }]}
          example={`curl $TOKEN_URL \\\n  -d grant_type=client_credentials \\\n  -u ${client.clientId}:$CLIENT_SECRET`}
          confirmLabel="I have stored the client secret somewhere safe"
        />
      ) : null}
    </Page>
  )
}
