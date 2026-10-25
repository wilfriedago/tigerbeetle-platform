import { useState } from 'react'

import { useDebouncedValue } from '@tanstack/react-pacer'
import { keepPreviousData, useQuery } from '@tanstack/react-query'

import { type AccountSummary } from '@tigerbeetle-platform/api-client'
import { GlobeIcon, PlusIcon, WalletIcon } from 'lucide-react'

import { Amount } from '~/components/ledger/amount'
import {
  Combobox,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxInput,
  ComboboxItem,
  ComboboxLabel,
  ComboboxList
} from '~/components/ui/combobox'

import { settingsQueries } from '~/features/settings/api'

import { formatDate } from '~/lib/time'
import { cn } from '~/lib/utils'

import { accountQueries } from '../api'

// 22 Pickers, Account combobox: typing filters existing accounts by address, each option shows the
// available balance in the posting's asset, world is always offered, and an address that does not
// exist yet is offered as "Use new address" with the rule that will create it (ADR-0006).

const ADDRESS = /^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*$/
const WORLD = 'world'

interface Option {
  value: string
  kind: 'account' | 'world' | 'new'
  account?: AccountSummary
}

interface Group {
  value: string
  items: Option[]
}

interface AccountComboboxProps {
  ledger: string
  value: string
  onChange: (address: string) => void
  onBlur?: () => void
  /** The posting's asset: options show their available balance in it. */
  asset?: string
  id?: string
  invalid?: boolean
  describedBy?: string | undefined
  placeholder?: string
}

export function AccountCombobox({
  ledger,
  value,
  onChange,
  onBlur,
  asset,
  id,
  invalid = false,
  describedBy,
  placeholder = 'Search or type an address'
}: AccountComboboxProps) {
  const [input, setInput] = useState(value)
  const [open, setOpen] = useState(false)
  const [query] = useDebouncedValue(input.trim(), { wait: 150 })
  // While the menu is closed the input shows the selection; searching starts when it opens.
  const search = open && query !== value ? query : ''
  const accounts = useQuery({
    ...accountQueries.list(ledger, { limit: 8, search: search || undefined }),
    enabled: open,
    placeholderData: keepPreviousData
  })
  const typed = input.trim()
  const offerNew = typed !== '' && typed !== WORLD && ADDRESS.test(typed) && !(accounts.data?.data ?? []).some((account) => account.address === typed)
  const rule = useQuery({ ...settingsQueries.match(ledger, typed), enabled: open && offerNew })

  const groups: Group[] = []
  const found = (accounts.data?.data ?? []).filter((account) => account.address !== WORLD)
  if (found.length > 0) {
    groups.push({
      items: found.map((account) => ({ account, kind: 'account', value: account.address })),
      value: search ? `Accounts matching ${search}` : 'Accounts'
    })
  }
  if (offerNew) {
    groups.push({ items: [{ kind: 'new', value: typed }], value: 'New address' })
  }
  groups.push({ items: [{ kind: 'world', value: WORLD }], value: 'Always available' })

  const selected: Option | null = value ? { kind: value === WORLD ? 'world' : 'account', value } : null
  return (
    <Combobox<Option>
      items={groups}
      filter={null}
      value={selected}
      onValueChange={(option) => {
        onChange(option?.value ?? '')
        setInput(option?.value ?? '')
      }}
      inputValue={input}
      onInputValueChange={setInput}
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) {
          setInput(value)
          onBlur?.()
        }
      }}
      itemToStringLabel={(option) => option.value}
      isItemEqualToValue={(a, b) => a.value === b.value}
      autoHighlight
    >
      <ComboboxInput
        id={id}
        placeholder={placeholder}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        spellCheck={false}
        autoComplete="off"
        className={cn('w-full [&_input]:font-mono [&_input]:placeholder:font-sans')}
      />
      <ComboboxContent className="min-w-[min(420px,var(--available-width))]">
        <ComboboxEmpty>No account matches.</ComboboxEmpty>
        <ComboboxList>
          {(group: Group) => (
            <ComboboxGroup key={group.value} items={group.items}>
              <ComboboxLabel>{group.value}</ComboboxLabel>
              <ComboboxCollection>
                {(option: Option) => (
                  <ComboboxItem key={option.value} value={option} className="items-start py-1.5">
                    <OptionRow option={option} asset={asset} rule={option.kind === 'new' ? rule.data?.rule : undefined} />
                  </ComboboxItem>
                )}
              </ComboboxCollection>
            </ComboboxGroup>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}

const LIMIT_TEXT = {
  credits_must_not_exceed_debits: 'credits cannot exceed debits',
  debits_must_not_exceed_credits: 'cannot go negative',
  none: 'no limit'
} as const

function OptionRow({
  option,
  asset,
  rule
}: {
  option: Option
  asset: string | undefined
  rule: { pattern: string; limit: keyof typeof LIMIT_TEXT } | undefined
}) {
  if (option.kind === 'world') {
    return (
      <>
        <GlobeIcon className="mt-0.5" />
        <span className="flex min-w-0 flex-col">
          <span className="font-mono text-small">world</span>
          <span className="text-caption text-muted-foreground">Money enters and leaves the ledger here. No limit.</span>
        </span>
      </>
    )
  }
  if (option.kind === 'new') {
    return (
      <>
        <PlusIcon className="mt-0.5" />
        <span className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-small">Use new address {option.value}</span>
          <span className="text-caption text-muted-foreground">
            {rule ? (
              <>
                Created by the first posting, rule <span className="font-mono">{rule.pattern}</span> ({LIMIT_TEXT[rule.limit]}).
              </>
            ) : (
              'Created by the first posting that uses it.'
            )}
          </span>
        </span>
      </>
    )
  }
  const { account } = option
  const balance = account?.balances.find((entry) => entry.asset === asset) ?? account?.balances[0]
  return (
    <>
      <WalletIcon className="mt-0.5" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-mono text-small">{option.value}</span>
        {account ? (
          <span className="text-caption text-muted-foreground">
            {account.balances.map((entry) => entry.asset).join(', ') || 'no assets'} created {formatDate(account.createdAt)}
          </span>
        ) : null}
      </span>
      {balance ? (
        <span className="flex shrink-0 flex-col items-end">
          <Amount value={balance.available} asset={balance.asset} />
          <span className="text-[11px] text-subtle-foreground">available</span>
        </span>
      ) : null}
    </>
  )
}
