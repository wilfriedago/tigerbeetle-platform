import { createFileRoute, notFound, Outlet } from '@tanstack/react-router'

import { OrganizationNotFound } from '~/features/organizations/organization-not-found'
import { OrganizationSuspended } from '~/features/organizations/organization-suspended'

import { actForOrganization } from '~/lib/api'
import { sessionQuery } from '~/lib/auth/session'
import { activeOrganization, setActiveOrganization } from '~/lib/organization'

// Every page of an organization starts here (ADR-0027, ADR-0030): the person must belong to it,
// and every call below acts for it. Switching organizations drops the cached answers of the
// previous one, so its ledgers never show under the new one.
export const Route = createFileRoute('/_console/$org')({
  beforeLoad: async ({ context, params }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery())
    const organization = session.organizations.find((o) => o.alias === params.org)
    if (!organization) {
      throw notFound()
    }
    if (organization.status === 'suspended') {
      // Every call would answer 403 organization-suspended; the page says why instead.
      return { suspended: organization }
    }
    if (activeOrganization() !== params.org) {
      context.queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'session' })
    }
    setActiveOrganization(params.org)
    actForOrganization(params.org)
    await context.queryClient.ensureQueryData(sessionQuery(params.org))
    return { suspended: undefined }
  },
  component: OrganizationLayout,
  notFoundComponent: OrganizationNotFound,
  staticData: { crumb: { label: (params) => params.org ?? '' } }
})

function OrganizationLayout() {
  const { suspended } = Route.useRouteContext()
  return suspended ? <OrganizationSuspended organization={suspended} /> : <Outlet />
}
