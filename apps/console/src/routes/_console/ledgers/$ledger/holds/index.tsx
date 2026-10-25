import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { holdQueries } from '~/features/holds/api'
import { HoldsPage } from '~/features/holds/holds-page'

export const Route = createFileRoute('/_console/ledgers/$ledger/holds/')({
  validateSearch: z.object({
    address: z.string().optional(),
    cursor: z.string().optional(),
    status: z.enum(['open', 'captured', 'voided', 'expired']).optional()
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ context, params, deps }) =>
    context.queryClient.ensureQueryData(
      holdQueries.list(params.ledger, { address: deps.address, cursor: deps.cursor, limit: 25, status: deps.status ?? 'open' })
    ),
  component: HoldsPage
})
