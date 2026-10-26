import { createFileRoute } from '@tanstack/react-router'

import { z } from 'zod'

import { credentialQueries } from '~/features/credentials/api'
import { MachineClientsPage } from '~/features/credentials/machine-clients-page'

export const Route = createFileRoute('/_console/$org/machine-clients/')({
  validateSearch: z.object({
    create: z.boolean().optional(),
    q: z.string().optional(),
    status: z.enum(['active', 'deleted']).optional()
  }),
  loader: async ({ context }) => context.queryClient.ensureQueryData(credentialQueries.machineClients()),
  component: MachineClientsPage
})
