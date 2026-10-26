import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/$org/api-keys')({
  staticData: { crumb: { label: 'API keys' } }
})
