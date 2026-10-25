import { useEffect, useState } from 'react'

import { useDebouncedValue } from '@tanstack/react-pacer'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'

import { ArrowLeftRightIcon, InboxIcon, LockIcon, PlusIcon, ScaleIcon, SearchIcon, WalletIcon } from 'lucide-react'

import { Address } from '~/components/ledger/address'
import { StatusBadge } from '~/components/ledger/status-badge'
import { Button } from '~/components/ui/button'
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandFooter,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut
} from '~/components/ui/command'
import { Kbd } from '~/components/ui/kbd'
import { Spinner } from '~/components/ui/spinner'

import { ledgerQueries } from '~/features/ledgers/api'

import { shortId } from '~/lib/ids'
import { formatTimestamp } from '~/lib/time'

// 14 Overlays, Command palette: one box for every identifier the ledger knows. A 32-hex ID opens a
// transaction or hold, an address or prefix finds accounts and offers the prefix sum, a reference
// finds transactions, an Idempotency-Key finds the write request. Results come from
// GET /v1/ledgers/{ledger}/search, so cmdk's own filtering is off.

const PREFIX_PATTERN = /^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*:?$/
const IS_MAC = /Mac|iPhone|iPad/.test(globalThis.navigator?.platform ?? '')

/** The open state and the global Cmd+K / Ctrl+K shortcut. */
function usePaletteHotkey(): [boolean, (open: boolean) => void] {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        setOpen((value) => !value)
      }
    }
    globalThis.addEventListener('keydown', onKeyDown)
    return () => globalThis.removeEventListener('keydown', onKeyDown)
  }, [])
  return [open, setOpen]
}

export function CommandPalette({ ledger }: { ledger: string | undefined }) {
  const [open, setOpen] = usePaletteHotkey()
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="hidden h-8 w-[280px] items-center gap-2 rounded-md border border-border bg-surface px-2.5 text-left text-small text-subtle-foreground outline-none hover:bg-surface-hover focus-visible:ring-3 focus-visible:ring-focus-ring md:flex"
      >
        <SearchIcon aria-hidden="true" className="size-4 text-muted-foreground" />
        <span className="flex-1 truncate">Search ID, address, reference</span>
        <Kbd>{IS_MAC ? '⌘K' : 'Ctrl K'}</Kbd>
      </button>
      <Button variant="ghost" size="icon-sm" className="md:hidden" aria-label="Search" onClick={() => setOpen(true)}>
        <SearchIcon />
      </Button>
      <CommandDialog open={open} onOpenChange={setOpen}>
        {/* Mounted only while open, so each opening starts from an empty query. */}
        {open ? <PaletteBody ledger={ledger} onClose={() => setOpen(false)} /> : null}
      </CommandDialog>
    </>
  )
}

function PaletteBody({ ledger, onClose }: { ledger: string | undefined; onClose: () => void }) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [debounced] = useDebouncedValue(query, { wait: 180 })
  const q = debounced.trim()
  const search = useQuery({ ...ledgerQueries.search(ledger ?? '', q), enabled: Boolean(ledger) && q.length > 0, placeholderData: keepPreviousData })
  const results = q.length > 0 ? search.data : undefined
  // cmdk keeps its highlight when the items change; with server results the first match should
  // lead, so the highlight resets whenever a new answer arrives.
  const [selection, setSelection] = useState({ at: 0, value: '' })

  if (!ledger) {
    return (
      <Command>
        <CommandList>
          <CommandEmpty>Open a ledger first: search works inside one ledger.</CommandEmpty>
        </CommandList>
      </Command>
    )
  }

  const prefix = q.replace(/:$/, '')
  const offerPrefix = PREFIX_PATTERN.test(q) && !/^[0-9a-f]{32}$/.test(q)
  const nothing = results && results.accounts.length + results.transactions.length + results.holds.length + results.requests.length === 0
  const first =
    (results?.accounts[0] && `account:${results.accounts[0].address}`) ??
    (results?.transactions[0] && `tx:${results.transactions[0].id}`) ??
    (results?.holds[0] && `hold:${results.holds[0].id}`) ??
    (results?.requests[0] && `request:${results.requests[0].key}`) ??
    (offerPrefix ? `balances:${prefix}` : 'action:new-transaction')
  const answeredAt = q.length > 0 ? search.dataUpdatedAt : 0
  const selected = selection.at === answeredAt ? selection.value : first

  return (
    <Command shouldFilter={false} loop value={selected} onValueChange={(value) => setSelection({ at: answeredAt, value })}>
      <CommandInput value={query} onValueChange={setQuery} placeholder="Search ID, address, reference or Idempotency-Key" />
      <CommandList>
        {q.length > 0 && search.isFetching && !results ? (
          <div className="flex items-center justify-center gap-2 py-8 text-small text-muted-foreground">
            <Spinner /> Searching {ledger}
          </div>
        ) : null}
        {nothing && !offerPrefix ? <CommandEmpty>Nothing in {ledger} matches this.</CommandEmpty> : null}
        {results && results.accounts.length > 0 ? (
          <CommandGroup heading="Accounts">
            {results.accounts.map((account) => (
              <CommandItem
                key={account.address}
                value={`account:${account.address}`}
                onSelect={() => {
                  onClose()
                  void navigate({ params: { address: account.address, ledger }, to: '/ledgers/$ledger/accounts/$address' })
                }}
              >
                <WalletIcon />
                <Address value={account.address} />
                <CommandShortcut className="font-mono">{account.assets.join(' ')}</CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        {results && results.transactions.length > 0 ? (
          <CommandGroup heading="Transactions">
            {results.transactions.map((tx) => (
              <CommandItem
                key={tx.id}
                value={`tx:${tx.id}`}
                onSelect={() => {
                  onClose()
                  void navigate({ params: { ledger, txId: tx.id }, to: '/ledgers/$ledger/transactions/$txId' })
                }}
              >
                <ArrowLeftRightIcon />
                <span className="font-mono">{shortId(tx.id)}</span>
                {tx.reference ? <span className="truncate font-mono text-muted-foreground">{tx.reference}</span> : null}
                <CommandShortcut>{formatTimestamp(tx.committedAt)}</CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        {results && results.holds.length > 0 ? (
          <CommandGroup heading="Holds">
            {results.holds.map((hold) => (
              <CommandItem
                key={hold.id}
                value={`hold:${hold.id}`}
                onSelect={() => {
                  onClose()
                  void navigate({ params: { holdId: hold.id, ledger }, to: '/ledgers/$ledger/holds/$holdId' })
                }}
              >
                <LockIcon />
                <span className="font-mono">{shortId(hold.id)}</span>
                <CommandShortcut>
                  <StatusBadge status={hold.status} />
                </CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        {results && results.requests.length > 0 ? (
          <CommandGroup heading="Requests">
            {results.requests.map((request) => (
              <CommandItem
                key={request.key}
                value={`request:${request.key}`}
                onSelect={() => {
                  onClose()
                  void navigate({ params: { ledger }, search: { key: request.key }, to: '/ledgers/$ledger/requests' })
                }}
              >
                <InboxIcon />
                <span className="truncate font-mono">{request.key}</span>
                <CommandShortcut>
                  {request.operation} <StatusBadge status={request.status} />
                </CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        {offerPrefix ? (
          <CommandGroup heading="Balances">
            <CommandItem
              value={`balances:${prefix}`}
              onSelect={() => {
                onClose()
                void navigate({ params: { ledger }, search: { prefix }, to: '/ledgers/$ledger/balances' })
              }}
            >
              <ScaleIcon />
              <span>
                Sum balances under <span className="font-mono">{prefix}:</span>
              </span>
            </CommandItem>
          </CommandGroup>
        ) : null}
        <CommandGroup heading="Actions">
          <CommandItem
            value="action:new-transaction"
            onSelect={() => {
              onClose()
              void navigate({ params: { ledger }, to: '/ledgers/$ledger/transactions/new' })
            }}
          >
            <PlusIcon />
            New transaction
          </CommandItem>
          <CommandItem
            value="action:new-hold"
            onSelect={() => {
              onClose()
              void navigate({ params: { ledger }, to: '/ledgers/$ledger/holds/new' })
            }}
          >
            <LockIcon />
            New hold
          </CommandItem>
        </CommandGroup>
      </CommandList>
      <CommandFooter>
        <span>
          <Kbd>↑</Kbd>
          <Kbd>↓</Kbd> move
        </span>
        <span>
          <Kbd>↵</Kbd> open
        </span>
        <span>
          <Kbd>esc</Kbd> close
        </span>
      </CommandFooter>
    </Command>
  )
}
