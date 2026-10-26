import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { settingsQueries } from '~/features/settings/api'
import { RulesPage } from '~/features/settings/rules-page'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/settings/rules')({
  validateSearch: z.object({ address: z.string().optional() }),
  loader: async ({ context, params }) => context.queryClient.ensureQueryData(settingsQueries.rules(params.ledger)),
  component: RulesPage,
  staticData: { crumb: { label: 'Account rules' } }
})
