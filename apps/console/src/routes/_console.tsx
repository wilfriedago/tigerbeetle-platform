import { createFileRoute, Outlet, useParams } from '@tanstack/react-router'

import { AppShell } from '~/components/layout/app-shell'
import { AppSidebar } from '~/components/layout/app-sidebar'

import { LedgerSwitcher } from '~/features/ledgers/ledger-switcher'
import { ThemeToggle } from '~/features/preferences/theme-toggle'
import { CommandPalette } from '~/features/search/command-palette'
import { HealthIndicator, VersionLine } from '~/features/system/health-indicator'

import { getLastLedger } from '~/lib/last-ledger'

export const Route = createFileRoute('/_console')({
  component: ConsoleLayout
})

function ConsoleLayout() {
  // Ledger pages carry the ledger in the URL; System and Preferences keep the last one open.
  const params = useParams({ strict: false })
  const ledger = params.ledger ?? getLastLedger()
  return (
    <AppShell
      sidebar={(props) => <AppSidebar ledger={ledger} switcher={<LedgerSwitcher current={ledger} />} versions={<VersionLine />} {...props} />}
      topBarActions={
        <>
          <CommandPalette ledger={ledger} />
          <HealthIndicator />
          <ThemeToggle />
        </>
      }
    >
      <Outlet />
    </AppShell>
  )
}
