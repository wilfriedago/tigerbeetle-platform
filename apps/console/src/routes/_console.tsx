import { createFileRoute, Outlet, useLocation, useParams } from '@tanstack/react-router'

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
  // Staff pages act for no organization (ADR-0027): no organization or ledger to switch.
  const staffPage = useLocation({ select: (l) => l.pathname.startsWith('/staff') })
  const ledger = staffPage ? undefined : (params.ledger ?? (params.org ? getLastLedger() : undefined))
  return (
    <AppShell
      sidebar={(props) => (
        <AppSidebar
          ledger={ledger}
          switcher={
            staffPage ? (
              <div className="flex h-9 items-center gap-2 px-2 text-small font-medium text-foreground">Platform staff</div>
            ) : (
              <>
                <OrganizationSwitcher />
                <LedgerSwitcher current={ledger} />
              </>
            )
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
