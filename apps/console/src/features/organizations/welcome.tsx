import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'

import { BuildingIcon } from 'lucide-react'

import { Page } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

import { sessionQuery, signOut } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'

/**
 * .design/auth.pen, No organization yet: a signed-in person who belongs to no organization
 * (ADR-0027). Membership comes from the token, so a fresh one appears within five minutes.
 */
export function Welcome() {
  const session = useSession()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const checkAgain = async () => {
    const fresh = await queryClient.fetchQuery({ ...sessionQuery(), staleTime: 0 })
    if (fresh.organizations.length > 0) {
      await navigate({ to: '/' })
    }
  }
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <BuildingIcon />
          </EmptyMedia>
          <EmptyTitle>You are not in an organization yet</EmptyTitle>
          <EmptyDescription>
            Your account works, but no organization has added you, so there is nothing to show. Ask an admin of your organization to invite{' '}
            {session?.email ?? 'you'}. The invitation arrives by email from Keycloak.
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent className="flex-row justify-center">
          <Button onClick={() => void checkAgain()}>Check again</Button>
          <Button variant="secondary" onClick={() => void signOut()}>
            Sign out
          </Button>
        </EmptyContent>
        <Hint>Accepted an invitation a moment ago? Check again: your session picks up a new membership within 5 minutes.</Hint>
      </Empty>
    </Page>
  )
}
