import { createFileRoute } from '@tanstack/react-router'

import { credentialQueries } from '~/features/credentials/api'
import { MachineClientPage } from '~/features/credentials/machine-client-page'

export const Route = createFileRoute('/_console/$org/machine-clients/$client')({
  loader: async ({ context }) => context.queryClient.ensureQueryData(credentialQueries.machineClients()),
  component: MachineClientPage,
  staticData: { crumb: { label: (params) => params.client ?? '', mono: true } }
})
