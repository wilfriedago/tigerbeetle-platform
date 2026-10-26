import { createFileRoute } from '@tanstack/react-router'

import { OrganizationSettingsPage } from '~/features/organizations/settings-page'

export const Route = createFileRoute('/_console/$org/settings/')({
  component: OrganizationSettingsPage
})
