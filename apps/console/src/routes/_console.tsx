import { createFileRoute, Outlet, useParams } from '@tanstack/react-router'

import { AppShell } from '~/components/layout/app-shell'
import { AppSidebar } from '~/components/layout/app-sidebar'

import { LedgerSwitcher } from '~/features/ledgers/ledger-switcher'
import { OrganizationSwitcher } from '~/features/organizations/organization-switcher'
import { ThemeToggle } from '~/features/preferences/theme-toggle'
import { CommandPalette } from '~/features/search/command-palette'
import { UserMenu } from '~/features/shell/user-menu'
import { HealthIndicator, VersionLine } from '~/features/system/health-indicator'

import { requireSession } from '~/lib/auth/session'
import { getLastLedger } from '~/lib/last-ledger'

export const Route = createFileRoute('/_console')({
  // Every page needs a signed-in person (ADR-0030). Without a session the browser goes to sign
  // in, and comes back to the page it asked for.
  beforeLoad: async ({ context, location }) => {
    await requireSession(context.queryClient, location.href)
  },
  component: ConsoleLayout
})

function ConsoleLayout() {
  // Ledger pages carry the organization and the ledger in the URL; Preferences keeps the last ones.
  const params = useParams({ strict: false })
  const ledger = params.ledger ?? (params.org ? getLastLedger() : undefined)
  return (
    <AppShell
      sidebar={(props) => (
        <AppSidebar
          ledger={ledger}
          switcher={
            <>
              <OrganizationSwitcher />
              <LedgerSwitcher current={ledger} />
            </>
          }
          versions={<VersionLine />}
          {...props}
        />
      )}
      topBarActions={
        <>
          <CommandPalette ledger={ledger} />
          <HealthIndicator />
          <ThemeToggle />
          <UserMenu />
        </>
      }
    >
      <Outlet />
    </AppShell>
  )
}
