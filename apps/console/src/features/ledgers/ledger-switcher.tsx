import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { CheckIcon, ChevronsUpDownIcon, LibraryIcon, PlusIcon } from 'lucide-react'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuLinkItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '~/components/ui/dropdown-menu'

import { ledgerQueries } from './api'
import { NewLedgerDialog } from './new-ledger-dialog'

// 13 Navigation, Ledger Switcher: the ledger mark, a "Ledger" caption over the mono name and the
// up-down chevrons. The menu lists the ledgers, links to the ledger list and opens "New ledger".

export function LedgerSwitcher({ current }: { current: string | undefined }) {
  const [creating, setCreating] = useState(false)
  const { data } = useQuery(ledgerQueries.list())
  const ledgers = data?.data ?? []
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger className="flex h-11 w-full items-center gap-2.5 rounded-md border border-border bg-surface px-2 py-1.5 text-left outline-none hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring aria-expanded:bg-surface-hover">
          <span
            aria-hidden="true"
            className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary font-mono text-[14px] font-semibold text-primary-foreground uppercase"
          >
            {current ? current.charAt(0) : '?'}
          </span>
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-[11px] leading-4 text-subtle-foreground">Ledger</span>
            <span className="truncate font-mono text-body font-medium text-foreground">{current ?? 'Choose a ledger'}</span>
          </span>
          <ChevronsUpDownIcon aria-hidden="true" className="size-4 shrink-0 text-subtle-foreground" />
        </DropdownMenuTrigger>
        <DropdownMenuContent className="w-(--anchor-width) min-w-60">
          <DropdownMenuGroup>
            <DropdownMenuLabel>Ledgers</DropdownMenuLabel>
            {ledgers.length === 0 ? <div className="px-2 py-1.5 text-small text-muted-foreground">No ledgers yet.</div> : null}
            {ledgers.map((ledger) => (
              <DropdownMenuLinkItem key={ledger.name} render={<Link to="/ledgers/$ledger" params={{ ledger: ledger.name }} />}>
                <span className="flex-1 truncate font-mono">{ledger.name}</span>
                {ledger.name === current ? <CheckIcon className="text-primary-text" /> : null}
              </DropdownMenuLinkItem>
            ))}
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuLinkItem render={<Link to="/ledgers" />}>
            <LibraryIcon />
            All ledgers
          </DropdownMenuLinkItem>
          <DropdownMenuItem onClick={() => setCreating(true)}>
            <PlusIcon />
            New ledger
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <NewLedgerDialog open={creating} onOpenChange={setCreating} />
    </>
  )
}
