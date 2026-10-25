import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { transactionQueries } from '~/features/transactions/api'
import { TransactionPage } from '~/features/transactions/transaction-page'

import { shortId } from '~/lib/ids'
import { orNotFound } from '~/lib/loaders'

export const Route = createFileRoute('/_console/ledgers/$ledger/transactions/$txId')({
  validateSearch: z.object({ tab: z.enum(['postings', 'metadata', 'technical']).optional() }),
  loader: async ({ context, params }) => orNotFound(context.queryClient.ensureQueryData(transactionQueries.detail(params.ledger, params.txId))),
  component: TransactionPage,
  staticData: { crumb: { label: (params) => shortId(params.txId ?? ''), mono: true } }
})
