import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/staff/organizations')({
  staticData: { crumb: { label: 'Organizations' } }
})
