import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { ledgerQueries } from '~/features/ledgers/api'
import { LedgersPage } from '~/features/ledgers/ledgers-page'

export const Route = createFileRoute('/_console/$org/ledgers/')({
  validateSearch: z.object({ cursor: z.string().optional() }),
  loaderDeps: ({ search }) => ({ cursor: search.cursor }),
  loader: async ({ context, deps }) => context.queryClient.ensureQueryData(ledgerQueries.list(deps.cursor)),
  component: LedgersPage,
  staticData: { crumb: { label: 'Ledgers' } }
})
