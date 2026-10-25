import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { NewTransactionPage } from '~/features/transactions/new-transaction/new-transaction-page'

// "Fund an account" and "New transaction from this address" open the form with a first posting.
export const Route = createFileRoute('/_console/ledgers/$ledger/transactions/new')({
  validateSearch: z.object({ asset: z.string().optional(), destination: z.string().optional(), source: z.string().optional() }),
  component: NewTransactionPage,
  staticData: { crumb: { label: 'New transaction' } }
})
