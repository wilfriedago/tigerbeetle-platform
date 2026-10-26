import { Link } from '@tanstack/react-router'

import { BuildingIcon, CheckIcon, ChevronsUpDownIcon } from 'lucide-react'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuLinkItem,
  DropdownMenuTrigger
} from '~/components/ui/dropdown-menu'

import { useSession } from '~/lib/auth/use-session'
import { useOrganization } from '~/lib/organization'

// The organization the console acts for (ADR-0026), above the ledger switcher. Choosing another
// one opens its ledgers; the URL carries the organization, so each tab keeps its own.
export function OrganizationSwitcher() {
  const current = useOrganization()
  const session = useSession()
  const organizations = session?.organizations ?? []
  const name = organizations.find((o) => o.alias === current)?.name ?? current
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex h-9 w-full items-center gap-2 rounded-md px-2 text-left outline-none hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring aria-expanded:bg-surface-hover">
        <BuildingIcon aria-hidden="true" className="size-4 shrink-0 text-subtle-foreground" />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-[11px] leading-4 text-subtle-foreground">Organization</span>
          <span className="truncate text-small font-medium text-foreground">{name || 'Choose an organization'}</span>
        </span>
        <ChevronsUpDownIcon aria-hidden="true" className="size-4 shrink-0 text-subtle-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-(--anchor-width) min-w-60">
        <DropdownMenuGroup>
          <DropdownMenuLabel>Organizations</DropdownMenuLabel>
          {organizations.map((o) => (
            <DropdownMenuLinkItem key={o.alias} render={<Link to="/$org/ledgers" params={{ org: o.alias }} />}>
              <span className="flex-1 truncate">{o.name}</span>
              <span className="font-mono text-small text-subtle-foreground">{o.alias}</span>
              {o.alias === current ? <CheckIcon className="text-primary-text" /> : null}
            </DropdownMenuLinkItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
