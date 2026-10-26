import { createFileRoute, redirect } from '@tanstack/react-router'

import { requireSession } from '~/lib/auth/session'
import { getLastLedger } from '~/lib/last-ledger'
import { getLastOrganization, setActiveOrganization } from '~/lib/organization'

// "/" opens the organization and ledger used last in this browser, or the first organization's
// ledger list (docs/console.md, Sitemap). A person in no organization lands on Welcome.
export const Route = createFileRoute('/')({
  beforeLoad: async ({ context, location }) => {
    const session = await requireSession(context.queryClient, location.href)
    const last = getLastOrganization()
    const org = session.organizations.find((o) => o.alias === last)?.alias ?? session.organizations[0]?.alias
    if (!org) {
      throw redirect({ replace: true, to: '/welcome' })
    }
    setActiveOrganization(org)
    const ledger = getLastLedger()
    if (ledger) {
      throw redirect({ params: { ledger, org }, replace: true, to: '/$org/ledgers/$ledger' })
    }
    throw redirect({ params: { org }, replace: true, to: '/$org/ledgers' })
  }
})
