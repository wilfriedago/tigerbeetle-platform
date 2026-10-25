import { useState } from 'react'

import { Combobox, ComboboxContent, ComboboxEmpty, ComboboxInput, ComboboxItem, ComboboxList } from '~/components/ui/combobox'

import { formatAsset, parseAsset } from '~/lib/money'
import { cn } from '~/lib/utils'

// The asset next to an amount: the ledger's assets, or a new CODE/SCALE that the first write will
// register (ADR-0005). What is typed is kept in canonical form, so "EUR" becomes "EUR/0".

interface AssetComboboxProps {
  value: string
  onChange: (asset: string) => void
  onBlur?: () => void
  assets: readonly string[]
  id?: string
  invalid?: boolean
  className?: string
  'aria-label'?: string
}

export function AssetCombobox({
  value,
  onChange,
  onBlur,
  assets,
  id,
  invalid = false,
  className,
  'aria-label': ariaLabel = 'Asset'
}: AssetComboboxProps) {
  const [input, setInput] = useState(value)
  const typed = parseAsset(input.toUpperCase())
  const canonical = typed ? formatAsset(typed) : undefined
  const items = canonical && !assets.includes(canonical) ? [...assets, canonical] : [...assets]
  return (
    <Combobox<string>
      items={items}
      value={value || null}
      onValueChange={(next) => {
        onChange(next ?? '')
        setInput(next ?? '')
      }}
      inputValue={input}
      onInputValueChange={(next) => setInput(next)}
      onOpenChange={(open) => {
        if (!open) {
          setInput(value)
          onBlur?.()
        }
      }}
      filter={(item, query) => item.startsWith(query.toUpperCase()) || item === canonical}
      autoHighlight
    >
      <ComboboxInput
        id={id}
        aria-label={ariaLabel}
        aria-invalid={invalid || undefined}
        placeholder="EUR/2"
        spellCheck={false}
        autoComplete="off"
        className={cn('[&_input]:font-mono [&_input]:uppercase', className)}
      />
      <ComboboxContent className="min-w-40">
        <ComboboxEmpty>Type CODE/SCALE, such as EUR/2.</ComboboxEmpty>
        <ComboboxList>
          {(item: string) => (
            <ComboboxItem key={item} value={item} className="font-mono">
              {item}
              {assets.includes(item) ? null : <span className="font-sans text-caption text-muted-foreground">new asset</span>}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
