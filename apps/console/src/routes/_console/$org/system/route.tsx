import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/$org/system')({
  staticData: { crumb: { label: 'System' } }
})
