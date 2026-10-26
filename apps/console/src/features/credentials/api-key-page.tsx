import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'

import { KeyRoundIcon, RotateCwIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Page } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { Hint } from '~/components/ledger/hint'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Timestamp } from '~/components/ledger/timestamp'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card'
import { Skeleton } from '~/components/ui/skeleton'

import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

import { credentialQueries, useRevokeApiKey } from './api'
import { ConfirmByName, CredentialRoles, CredentialStatus } from './components'

// .design/auth.pen, API key: one key's roles and details, and revoking it.

const route = getRouteApi('/_console/$org/api-keys/$key')

export function ApiKeyPage() {
  const org = useOrganization()
  const session = useSession()
  const { key: id } = route.useParams()
  const keys = useQuery(credentialQueries.apiKeys())
  const revoke = useRevokeApiKey()
  const navigate = useNavigate()
  const [revoking, setRevoking] = useState(false)
  const key = keys.data?.find((k) => k.id === id)
  const orgName = session?.organizations.find((o) => o.alias === org)?.name ?? org

  if (keys.error) {
    return (
      <Page>
        <ProblemPanel error={keys.error} onRetry={() => void keys.refetch()} />
      </Page>
    )
  }
  if (!key) {
    return <Page>{keys.isPending ? <Skeleton className="h-40" /> : <ProblemPanel error={new Error(`No API key ${id} in ${org}.`)} />}</Page>
  }
  const active = key.status === 'active'
  return (
    <Page>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex items-center gap-4">
          <span className="flex size-12 items-center justify-center rounded-lg border border-border bg-surface-sunken text-muted-foreground">
            <KeyRoundIcon className="size-5" />
          </span>
          <div className="flex flex-col gap-1">
            <h1 className="flex items-center gap-2 font-mono text-page-title font-medium text-foreground">
              {key.name}
              <CredentialStatus status={key.status} />
            </h1>
            <p className="text-small text-muted-foreground">
              API key of {orgName}, created <Timestamp value={key.createdAt} />
              {key.createdBy ? ` by ${key.createdBy}` : ''}
            </p>
          </div>
        </div>
        {active ? (
          <Button variant="secondary" className="text-danger-fg" onClick={() => setRevoking(true)}>
            Revoke key
          </Button>
        ) : null}
      </header>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
        <CredentialRoles kind="api-key" id={key.id} roles={key.ledgers} editable={active} />
        <aside className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle>Details</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <DescriptionList>
                <DescriptionItem term="Key ID" mono>
                  tbp_{key.id}
                </DescriptionItem>
                <DescriptionItem term="Principal" mono>
                  tbp:api-key / {key.id}
                </DescriptionItem>
                <DescriptionItem term="Secret">shown once at creation, not stored</DescriptionItem>
                <DescriptionItem term="Expires">{key.expiresAt ? <Timestamp value={key.expiresAt} /> : 'never'}</DescriptionItem>
                <DescriptionItem term="Last used">{key.lastUsedAt ? <Timestamp value={key.lastUsedAt} relative /> : 'never'}</DescriptionItem>
                {key.revokedAt ? (
                  <DescriptionItem term="Revoked">
                    <Timestamp value={key.revokedAt} />
                  </DescriptionItem>
                ) : null}
              </DescriptionList>
              <Hint>Last used is updated at most once a minute.</Hint>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <RotateCwIcon className="size-4 text-muted-foreground" />
                Rotating a key
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-small text-muted-foreground">
                Create a new key with the same roles, deploy it, then revoke this one. Both work in between.
              </p>
            </CardContent>
          </Card>
        </aside>
      </div>
      <ConfirmByName
        open={revoking}
        onOpenChange={setRevoking}
        title={`Revoke ${key.name}?`}
        name={key.name}
        consequences={[
          key.lastUsedAt ? 'Anything still sending it gets 401 from its next request.' : 'It was never used. Revoking it now costs nothing.',
          'Revocation is immediate and cannot be undone.',
          'Its writes and grants stay, with the key as actor.'
        ]}
        action="Revoke key"
        pending={revoke.isPending}
        error={revoke.error}
        onConfirm={() =>
          revoke.mutate(key.id, {
            onSuccess: () => {
              toast.success(`${key.name} was revoked`)
              setRevoking(false)
              void navigate({ params: { org }, to: '/$org/api-keys' })
            }
          })
        }
      />
    </Page>
  )
}
