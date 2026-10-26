import { createFileRoute } from '@tanstack/react-router'

import { LedgerSettingsPage } from '~/features/settings/ledger-settings-page'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/settings/')({
  component: LedgerSettingsPage
})
