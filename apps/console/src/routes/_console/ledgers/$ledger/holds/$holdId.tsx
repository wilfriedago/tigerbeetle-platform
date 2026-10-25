import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { holdQueries } from '~/features/holds/api'
import { HoldPage } from '~/features/holds/hold-page'

import { shortId } from '~/lib/ids'
import { orNotFound } from '~/lib/loaders'

export const Route = createFileRoute('/_console/ledgers/$ledger/holds/$holdId')({
  validateSearch: z.object({ tab: z.enum(['details', 'metadata', 'technical']).optional() }),
  loader: async ({ context, params }) => orNotFound(context.queryClient.ensureQueryData(holdQueries.detail(params.ledger, params.holdId))),
  component: HoldPage,
  staticData: { crumb: { label: (params) => shortId(params.holdId ?? ''), mono: true } }
})
