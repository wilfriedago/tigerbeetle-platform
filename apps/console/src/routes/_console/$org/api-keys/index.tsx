import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { credentialQueries } from '~/features/credentials/api'
import { ApiKeysPage } from '~/features/credentials/api-keys-page'

export const Route = createFileRoute('/_console/$org/api-keys/')({
  validateSearch: z.object({
    create: z.boolean().optional(),
    q: z.string().optional(),
    status: z.enum(['active', 'expired', 'revoked']).optional()
  }),
  loader: async ({ context }) => context.queryClient.ensureQueryData(credentialQueries.apiKeys()),
  component: ApiKeysPage
})
