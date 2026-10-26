import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/$org/settings')({
  staticData: { crumb: { label: 'Settings' } }
})
