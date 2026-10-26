import { BuildingIcon } from 'lucide-react'

import { Page } from '~/components/layout/page'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

import { useSession } from '~/lib/auth/use-session'

/** A signed-in person who belongs to no organization yet (ADR-0027). */
export function Welcome() {
  const session = useSession()
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <BuildingIcon />
          </EmptyMedia>
          <EmptyTitle>Welcome{session?.name ? `, ${session.name}` : ''}</EmptyTitle>
          <EmptyDescription>
            You are not a member of any organization yet. An owner of an organization can invite you; the invitation arrives by email.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </Page>
  )
}
