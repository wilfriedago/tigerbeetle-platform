import { createFileRoute } from '@tanstack/react-router'

import { ledgerQueries } from '~/features/ledgers/api'
import { LedgerNotFound } from '~/features/ledgers/ledger-not-found'

import { setLastLedger } from '~/lib/last-ledger'
import { orNotFound } from '~/lib/loaders'

// Every page of a ledger starts here: the ledger must exist before any of its views load.
export const Route = createFileRoute('/_console/$org/ledgers/$ledger')({
  loader: async ({ context, params }) => orNotFound(context.queryClient.ensureQueryData(ledgerQueries.detail(params.ledger))),
  onEnter: ({ params }) => setLastLedger(params.ledger),
  notFoundComponent: LedgerNotFound,
  staticData: { crumb: { label: (params) => params.ledger ?? '', mono: true } }
})
