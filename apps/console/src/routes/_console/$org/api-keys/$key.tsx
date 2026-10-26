import { createFileRoute } from '@tanstack/react-router'

import { credentialQueries } from '~/features/credentials/api'
import { ApiKeyPage } from '~/features/credentials/api-key-page'

export const Route = createFileRoute('/_console/$org/api-keys/$key')({
  loader: async ({ context }) => context.queryClient.ensureQueryData(credentialQueries.apiKeys()),
  component: ApiKeyPage,
  staticData: { crumb: { label: (params) => `tbp_${params.key ?? ''}`, mono: true } }
})
