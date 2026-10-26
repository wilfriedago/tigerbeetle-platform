import { createFileRoute } from '@tanstack/react-router'

import { memberQueries } from '~/features/members/api'
import { LedgerAccessPage } from '~/features/members/ledger-access-page'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/settings/access')({
  loader: async ({ context }) => context.queryClient.ensureQueryData(memberQueries.list()),
  component: LedgerAccessPage,
  staticData: { crumb: { label: 'Access' } }
})
