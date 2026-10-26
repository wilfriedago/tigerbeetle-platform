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
    // An active organization first: a suspended one would only show why it refuses every call.
    const active = session.organizations.filter((o) => o.status === 'active')
    const org = (active.find((o) => o.alias === last) ?? active[0] ?? session.organizations[0])?.alias
    if (!org) {
      throw redirect({ replace: true, to: session.staff ? '/staff/organizations' : '/welcome' })
    }
    setActiveOrganization(org)
    const ledger = getLastLedger()
    if (ledger) {
      throw redirect({ params: { ledger, org }, replace: true, to: '/$org/ledgers/$ledger' })
    }
    throw redirect({ params: { org }, replace: true, to: '/$org/ledgers' })
  }
})
