import { Link, type LinkComponentProps } from '@tanstack/react-router'

import {
  ActivityIcon,
  ArrowLeftRightIcon,
  CoinsIcon,
  InboxIcon,
  LayoutDashboardIcon,
  ListChecksIcon,
  LockIcon,
  ScaleIcon,
  SettingsIcon,
  SlidersHorizontalIcon,
  TagIcon,
  WalletIcon
} from 'lucide-react'

import { useOrganization } from '~/lib/organization'
import { cn } from '~/lib/utils'

// 13 Navigation, Sidebar: 240 px on the sidebar surface, ledger switcher on top, navigation scoped
// to the current ledger, System and Preferences pinned at the bottom. The active item uses the
// selected surface with accent text. One fixed icon per concept (04 Iconography).

const navItemClassName =
  'flex h-8 items-center gap-2.5 rounded-md px-2.5 text-body text-foreground outline-none transition-colors hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring data-[status=active]:bg-surface-selected data-[status=active]:font-medium data-[status=active]:text-primary-text [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-muted-foreground data-[status=active]:[&_svg]:text-primary-text'

function NavItem({ icon: Icon, label, ...link }: { icon: React.ElementType; label: string } & LinkComponentProps) {
  return (
    <Link className={navItemClassName} {...link}>
      <Icon aria-hidden="true" />
      <span className="flex-1 truncate">{label}</span>
    </Link>
  )
}

function NavSection({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div role="group" aria-label={label} className="flex flex-col gap-0.5">
      <div className="px-2.5 pt-4 pb-1 text-caption font-medium text-subtle-foreground">{label}</div>
      {children}
    </div>
  )
}

interface AppSidebarProps {
  ledger: string | undefined
  switcher: React.ReactNode
  versions?: React.ReactNode
  className?: string
  onNavigate?: () => void
}

export function AppSidebar({ ledger, switcher, versions, className, onNavigate }: AppSidebarProps) {
  const org = useOrganization()
  return (
    <nav
      aria-label="Console"
      className={cn('flex h-full w-60 shrink-0 flex-col gap-0.5 border-r border-sidebar-border bg-sidebar p-3', className)}
      onClickCapture={onNavigate}
    >
      {switcher}
      <div className="flex flex-1 flex-col gap-0.5 overflow-y-auto pt-3">
        {ledger ? (
          <>
            <NavItem
              icon={LayoutDashboardIcon}
              label="Overview"
              to="/$org/ledgers/$ledger"
              params={{ ledger, org }}
              activeOptions={{ exact: true }}
            />
            <NavSection label="Activity">
              <NavItem icon={ArrowLeftRightIcon} label="Transactions" to="/$org/ledgers/$ledger/transactions" params={{ ledger, org }} />
              <NavItem icon={LockIcon} label="Holds" to="/$org/ledgers/$ledger/holds" params={{ ledger, org }} />
            </NavSection>
            <NavSection label="Money">
              <NavItem icon={WalletIcon} label="Accounts" to="/$org/ledgers/$ledger/accounts" params={{ ledger, org }} />
              <NavItem icon={ScaleIcon} label="Balances" to="/$org/ledgers/$ledger/balances" params={{ ledger, org }} />
              <NavItem icon={CoinsIcon} label="Assets" to="/$org/ledgers/$ledger/assets" params={{ ledger, org }} />
            </NavSection>
            <NavSection label="Operations">
              <NavItem icon={InboxIcon} label="Requests" to="/$org/ledgers/$ledger/requests" params={{ ledger, org }} />
            </NavSection>
            <NavSection label="Settings">
              <NavItem
                icon={SettingsIcon}
                label="Ledger"
                to="/$org/ledgers/$ledger/settings"
                params={{ ledger, org }}
                activeOptions={{ exact: true }}
              />
              <NavItem icon={ListChecksIcon} label="Account rules" to="/$org/ledgers/$ledger/settings/rules" params={{ ledger, org }} />
              <NavItem icon={TagIcon} label="Event types" to="/$org/ledgers/$ledger/settings/event-types" params={{ ledger, org }} />
            </NavSection>
          </>
        ) : (
          <p className="px-2.5 pt-2 text-small text-muted-foreground">Pick or create a ledger to see its accounts and transactions.</p>
        )}
      </div>
      <div className="flex flex-col gap-0.5 border-t border-sidebar-border pt-2">
        <NavItem icon={ActivityIcon} label="System" to="/$org/system" params={{ org }} />
        <NavItem icon={SlidersHorizontalIcon} label="Preferences" to="/preferences" />
        {versions ? <div className="flex gap-3 px-0 pt-1 font-mono text-[11px] text-subtle-foreground">{versions}</div> : null}
      </div>
    </nav>
  )
}
