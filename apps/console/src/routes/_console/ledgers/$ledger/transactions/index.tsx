import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { transactionQueries } from '~/features/transactions/api'
import { rangeStart, TIME_RANGE_KEYS } from '~/features/transactions/time-range'
import { TransactionsPage } from '~/features/transactions/transactions-page'

// Filters live in the URL, so a filtered list is a link (docs/console.md, Rules shared by every page).
const transactionsSearchSchema = z.object({
  address: z.string().optional(),
  after: z.string().optional(),
  asset: z.string().optional(),
  before: z.string().optional(),
  cursor: z.string().optional(),
  eventType: z.string().optional(),
  range: z.enum(TIME_RANGE_KEYS).optional(),
  reference: z.string().optional()
})

export const Route = createFileRoute('/_console/ledgers/$ledger/transactions/')({
  validateSearch: transactionsSearchSchema,
  loaderDeps: ({ search }) => search,
  loader: async ({ context, params, deps: { range, ...filters } }) =>
    context.queryClient.ensureQueryData(transactionQueries.list(params.ledger, { ...filters, after: filters.after ?? rangeStart(range), limit: 25 })),
  component: TransactionsPage
})
