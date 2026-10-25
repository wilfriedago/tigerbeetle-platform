import { createFileRoute } from '@tanstack/react-router'

import { DesignSystemPage } from '~/features/design-system/design-system-page'

export const Route = createFileRoute('/_console/design-system')({
  component: DesignSystemPage,
  staticData: { crumb: { label: 'Design system' } }
})
