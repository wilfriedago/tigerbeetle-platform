import { createFileRoute } from '@tanstack/react-router'

import { staffQueries } from '~/features/staff/api'
import { StaffOrganizationPage } from '~/features/staff/organization-page'

export const Route = createFileRoute('/_console/staff/organizations/$alias')({
  loader: async ({ context, params }) => context.queryClient.ensureQueryData(staffQueries.one(params.alias)),
  component: StaffOrganizationPage,
  staticData: { crumb: { label: (params) => params.alias ?? '', mono: true } }
})
