import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { AccountsPage } from '~/features/accounts/accounts-page'
import { accountQueries } from '~/features/accounts/api'

export const Route = createFileRoute('/_console/ledgers/$ledger/accounts/')({
  validateSearch: z.object({
    asset: z.string().optional(),
    cursor: z.string().optional(),
    prefix: z.string().optional(),
    search: z.string().optional(),
    view: z.enum(['list', 'tree']).optional()
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ context: { queryClient }, params: { ledger }, deps: { view, prefix, search, asset, cursor } }) => {
    if (view === 'tree') {
      return queryClient.ensureQueryData(accountQueries.segments(ledger, ''))
    }
    return queryClient.ensureQueryData(accountQueries.list(ledger, { asset, cursor, limit: 25, prefix, search }))
  },
  component: AccountsPage
})
