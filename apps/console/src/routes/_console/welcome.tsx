import { createFileRoute } from '@tanstack/react-router'

import { Welcome } from '~/features/organizations/welcome'

// A signed-in person who belongs to no organization yet (ADR-0027).
export const Route = createFileRoute('/_console/welcome')({
  component: Welcome,
  staticData: { crumb: { label: 'Welcome' } }
})
