import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { AccountPage } from '~/features/accounts/account-page'
import { accountQueries } from '~/features/accounts/api'

import { orNotFound } from '~/lib/loaders'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/accounts/$address')({
  validateSearch: z.object({ asset: z.string().optional(), tab: z.enum(['transactions', 'holds', 'history', 'metadata']).optional() }),
  loader: async ({ context, params }) => orNotFound(context.queryClient.ensureQueryData(accountQueries.detail(params.ledger, params.address))),
  component: AccountPage,
  staticData: { crumb: { label: (params) => params.address ?? '', mono: true } }
})
