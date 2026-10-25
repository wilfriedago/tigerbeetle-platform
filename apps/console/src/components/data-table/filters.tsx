import { useState } from 'react'

import { useDebouncer } from '@tanstack/react-pacer'

import { SearchIcon, XIcon } from 'lucide-react'

import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from '~/components/ui/input-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'

import { cn } from '~/lib/utils'

// The filter row above a table. Every filter writes to the URL (docs/console.md, URL as state);
// text filters wait for a pause in typing so each keystroke does not become a request.

interface SearchFilterProps {
  /** The value in the URL. */
  value: string | undefined
  onChange: (value: string | undefined) => void
  placeholder: string
  label: string
  mono?: boolean
  className?: string
}

export function SearchFilter({ value, onChange, placeholder, label, mono = true, className }: SearchFilterProps) {
  const [draft, setDraft] = useState({ source: value, text: value ?? '' })
  // The URL changed from elsewhere (a link, Back): show it.
  const text = draft.source === value ? draft.text : (value ?? '')
  const commit = useDebouncer((next: string) => onChange(next.trim() === '' ? undefined : next.trim()), { wait: 300 })
  const update = (next: string) => {
    setDraft({ source: value, text: next })
    commit.maybeExecute(next)
  }
  return (
    <InputGroup className={cn('w-full sm:w-60', className)}>
      <InputGroupAddon>
        <SearchIcon aria-hidden="true" />
      </InputGroupAddon>
      <InputGroupInput
        aria-label={label}
        placeholder={placeholder}
        value={text}
        spellCheck={false}
        autoComplete="off"
        onChange={(event) => update(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            commit.flush()
          }
        }}
        className={cn(mono && 'font-mono placeholder:font-sans')}
      />
      {text ? (
        <InputGroupAddon align="inline-end">
          <InputGroupButton
            size="icon-xs"
            aria-label={`Clear ${label.toLowerCase()}`}
            onClick={() => {
              setDraft({ source: undefined, text: '' })
              commit.cancel()
              onChange(undefined)
            }}
          >
            <XIcon />
          </InputGroupButton>
        </InputGroupAddon>
      ) : null}
    </InputGroup>
  )
}

const ALL = '__all__'

interface SelectFilterProps {
  value: string | undefined
  onChange: (value: string | undefined) => void
  /** The label of the "no filter" option, such as "All assets". */
  allLabel: string
  options: readonly { value: string; label: React.ReactNode }[]
  label: string
  mono?: boolean
  className?: string
}

export function SelectFilter({ value, onChange, allLabel, options, label, mono = false, className }: SelectFilterProps) {
  const items = [{ label: allLabel, value: ALL }, ...options]
  return (
    <Select items={items} value={value ?? ALL} onValueChange={(next) => onChange(next === ALL || next === null ? undefined : next)}>
      <SelectTrigger aria-label={label} className={cn('w-full sm:w-auto sm:min-w-36', className)}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {items.map((item) => (
          <SelectItem key={item.value} value={item.value} className={cn(mono && item.value !== ALL && 'font-mono')}>
            {item.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/** The row that holds the filters, with an optional note on the right. */
export function FilterBar({ children, aside, className }: { children: React.ReactNode; aside?: React.ReactNode; className?: string }) {
  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      {children}
      {aside ? <div className="ml-auto">{aside}</div> : null}
    </div>
  )
}
