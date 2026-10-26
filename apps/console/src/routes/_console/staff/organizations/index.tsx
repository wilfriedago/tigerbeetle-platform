import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { staffQueries } from '~/features/staff/api'
import { StaffOrganizationsPage } from '~/features/staff/organizations-page'

export const Route = createFileRoute('/_console/staff/organizations/')({
  validateSearch: z.object({
    cursor: z.string().optional(),
    new: z.boolean().optional(),
    q: z.string().optional(),
    status: z.enum(['active', 'suspended']).optional()
  }),
  loaderDeps: ({ search }) => ({ cursor: search.cursor }),
  loader: async ({ context, deps }) => context.queryClient.ensureQueryData(staffQueries.list(deps.cursor)),
  component: StaffOrganizationsPage
})
