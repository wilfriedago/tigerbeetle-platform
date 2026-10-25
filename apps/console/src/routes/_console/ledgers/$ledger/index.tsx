import { createFileRoute } from '@tanstack/react-router'

import { ledgerQueries } from '~/features/ledgers/api'
import { OverviewPage } from '~/features/overview/overview-page'
import { transactionQueries } from '~/features/transactions/api'

export const Route = createFileRoute('/_console/ledgers/$ledger/')({
  loader: async ({ context: { queryClient }, params: { ledger } }) => {
    // The cards and the recent transactions load together; neither blocks the other.
    await Promise.all([
      queryClient.ensureQueryData(ledgerQueries.stats(ledger)),
      queryClient.ensureQueryData(transactionQueries.list(ledger, { limit: 10 }))
    ])
  },
  component: OverviewPage
})
