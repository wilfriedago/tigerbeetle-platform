import { createFileRoute } from '@tanstack/react-router'

import { memberQueries } from '~/features/members/api'
import { MemberPage } from '~/features/members/member-page'

export const Route = createFileRoute('/_console/$org/members/$member')({
  loader: async ({ context }) => context.queryClient.ensureQueryData(memberQueries.list()),
  component: MemberPage,
  staticData: { crumb: { label: 'Member' } }
})
