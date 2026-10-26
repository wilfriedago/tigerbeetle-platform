import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { NewHoldPage } from '~/features/holds/new-hold-page'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/holds/new')({
  validateSearch: z.object({ asset: z.string().optional(), destination: z.string().optional(), source: z.string().optional() }),
  component: NewHoldPage,
  staticData: { crumb: { label: 'New hold' } }
})
