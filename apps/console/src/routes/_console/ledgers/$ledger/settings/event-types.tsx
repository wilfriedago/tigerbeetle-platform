import { createFileRoute } from '@tanstack/react-router'

import { settingsQueries } from '~/features/settings/api'
import { EventTypesPage } from '~/features/settings/event-types-page'

export const Route = createFileRoute('/_console/ledgers/$ledger/settings/event-types')({
  loader: async ({ context, params }) => context.queryClient.ensureQueryData(settingsQueries.eventTypes(params.ledger)),
  component: EventTypesPage,
  staticData: { crumb: { label: 'Event types' } }
})
