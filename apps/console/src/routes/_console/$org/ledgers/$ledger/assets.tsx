import { createFileRoute } from '@tanstack/react-router'

import { AssetsPage } from '~/features/assets/assets-page'
import { ledgerQueries } from '~/features/ledgers/api'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/assets')({
  loader: async ({ context, params }) => context.queryClient.ensureQueryData(ledgerQueries.assets(params.ledger)),
  component: AssetsPage,
  staticData: { crumb: { label: 'Assets' } }
})
