import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/system')({
  staticData: { crumb: { label: 'System' } }
})
