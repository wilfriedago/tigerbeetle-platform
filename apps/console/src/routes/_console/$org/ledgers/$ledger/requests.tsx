import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { requestQueries } from '~/features/requests/api'
import { RequestsPage } from '~/features/requests/requests-page'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/requests')({
  validateSearch: z.object({
    cursor: z.string().optional(),
    /** The Idempotency-Key whose drawer is open. */
    key: z.string().optional(),
    operation: z.enum(['transaction', 'revert', 'hold', 'capture', 'void']).optional(),
    search: z.string().optional(),
    status: z.enum(['pending', 'committed', 'declined', 'failed']).optional()
  }),
  loaderDeps: ({ search: { key: _key, ...filters } }) => filters,
  loader: async ({ context, params, deps }) => context.queryClient.ensureQueryData(requestQueries.list(params.ledger, { ...deps, limit: 25 })),
  component: RequestsPage,
  staticData: { crumb: { label: 'Requests' } }
})
