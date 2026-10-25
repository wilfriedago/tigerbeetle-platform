import { createFileRoute } from '@tanstack/react-router'

import { PreferencesPage } from '~/features/preferences/preferences-page'

export const Route = createFileRoute('/_console/preferences')({
  component: PreferencesPage,
  staticData: { crumb: { label: 'Preferences' } }
})
