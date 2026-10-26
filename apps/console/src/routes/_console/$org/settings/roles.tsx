import { createFileRoute } from '@tanstack/react-router'

import { RolesPage } from '~/features/members/roles-page'

export const Route = createFileRoute('/_console/$org/settings/roles')({
  component: RolesPage,
  staticData: { crumb: { label: 'Roles and permissions' } }
})
