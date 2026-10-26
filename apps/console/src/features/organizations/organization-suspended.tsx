import { Link } from '@tanstack/react-router'

import { PauseIcon } from 'lucide-react'

import { Page } from '~/components/layout/page'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

import { type SessionOrganization, signOut } from '~/lib/auth/session'

/**
 * .design/auth.pen, Organization suspended: platform staff suspended the organization, so the API
 * refuses every call in its name with 403 organization-suspended (ADR-0027). Nothing was deleted.
 */
export function OrganizationSuspended({ organization }: { organization: SessionOrganization }) {
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <PauseIcon />
          </EmptyMedia>
          <EmptyTitle>{organization.name} is suspended</EmptyTitle>
          <EmptyDescription>
            Platform staff suspended {organization.name}. Every request in its name is refused, from the console, API keys and machine clients. No
            data was deleted; its ledgers resume as they were when it is reactivated.
          </EmptyDescription>
          {organization.suspensionReason ? <p className="text-small text-foreground">{organization.suspensionReason}</p> : null}
          <p className="font-mono text-caption text-subtle-foreground">403 organization-suspended</p>
        </EmptyHeader>
        <EmptyContent className="flex-row justify-center">
          <Button render={<Link to="/" />}>Switch organization</Button>
          <Button variant="secondary" onClick={() => void signOut()}>
            Sign out
          </Button>
        </EmptyContent>
      </Empty>
    </Page>
  )
}
