import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { memberQueries } from '~/features/members/api'
import { MembersPage } from '~/features/members/members-page'

export const Route = createFileRoute('/_console/$org/members/')({
  validateSearch: z.object({
    invite: z.boolean().optional(),
    ledger: z.string().optional(),
    q: z.string().optional(),
    role: z.string().optional(),
    tab: z.enum(['invitations']).optional()
  }),
  loader: async ({ context }) => context.queryClient.ensureQueryData(memberQueries.list()),
  component: MembersPage
})
