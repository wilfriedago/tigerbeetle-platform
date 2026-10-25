import { createFileRoute, redirect } from '@tanstack/react-router'

import { getLastLedger } from '~/lib/last-ledger'

// "/" opens the ledger used last in this browser, or the ledger list (docs/console.md, Sitemap).
export const Route = createFileRoute('/')({
  beforeLoad: () => {
    const ledger = getLastLedger()
    if (ledger) {
      throw redirect({ params: { ledger }, replace: true, to: '/ledgers/$ledger' })
    }
    throw redirect({ replace: true, to: '/ledgers' })
  }
})
