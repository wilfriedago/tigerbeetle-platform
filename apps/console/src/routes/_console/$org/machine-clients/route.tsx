import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/$org/machine-clients')({
  staticData: { crumb: { label: 'Machine clients' } }
})
