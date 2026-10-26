import { createFileRoute, redirect } from '@tanstack/react-router'

import { actForOrganization } from '~/lib/api'
import { sessionQuery } from '~/lib/auth/session'

// The staff's pages (ADR-0027): only staff reach them, and their calls act for no organization,
// since staff are members of none; a stale X-Organization would make the API answer 404.
export const Route = createFileRoute('/_console/staff')({
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery())
    if (!session.staff) {
      throw redirect({ to: '/' })
    }
    actForOrganization(undefined)
  },
  staticData: { crumb: { label: 'Platform' } }
})
