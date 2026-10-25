import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { BalancesPage } from '~/features/balances/balances-page'

export const Route = createFileRoute('/_console/ledgers/$ledger/balances')({
  validateSearch: z.object({ prefix: z.string().optional() }),
  component: BalancesPage,
  staticData: { crumb: { label: 'Balances' } }
})
