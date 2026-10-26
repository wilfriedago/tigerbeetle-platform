import { createFileRoute } from '@tanstack/react-router'

import { ReconcilerPage } from '~/features/system/reconciler-page'

export const Route = createFileRoute('/_console/$org/system/reconciler')({
  component: ReconcilerPage,
  staticData: { crumb: { label: 'Reconciler' } }
})
