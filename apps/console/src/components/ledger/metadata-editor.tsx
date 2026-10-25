import { PlusIcon, XIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import { FieldError } from '~/components/ui/field'
import { Input } from '~/components/ui/input'

// 15 Data display, Metadata Row: edits one key/value pair; "Add field" appends one. Keys and values
// are machine values, so both are mono. The rules mirror ValidateMetadata in
// apps/api/internal/kernel/ledger/metadata.go.

export interface MetadataRow {
  key: string
  value: string
}

export const METADATA_KEY_PATTERN = /^[A-Za-z0-9_.:-]{1,64}$/
export const MAX_METADATA_ENTRIES = 64
export const MAX_METADATA_VALUE_LENGTH = 512

export function metadataToRows(metadata: Record<string, string> | undefined): MetadataRow[] {
  return Object.entries(metadata ?? {})
    .toSorted(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ key, value }))
}

/** Rows with an empty key are dropped: an unfinished row is not an entry. */
export function rowsToMetadata(rows: readonly MetadataRow[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const row of rows) {
    if (row.key.trim() !== '') {
      out[row.key.trim()] = row.value
    }
  }
  return out
}

/** The first problem with the rows, phrased like the API's own validation, or undefined. */
export function validateMetadataRows(rows: readonly MetadataRow[]): string | undefined {
  const seen = new Set<string>()
  const filled = rows.filter((row) => row.key.trim() !== '')
  if (filled.length > MAX_METADATA_ENTRIES) {
    return `Metadata has at most ${MAX_METADATA_ENTRIES} entries.`
  }
  for (const row of filled) {
    const key = row.key.trim()
    if (!METADATA_KEY_PATTERN.test(key)) {
      return `Key "${key}" must be 1 to 64 letters, digits or _ . : -.`
    }
    if (seen.has(key)) {
      return `Key "${key}" appears twice.`
    }
    if (row.value.length > MAX_METADATA_VALUE_LENGTH) {
      return `The value of "${key}" is longer than ${MAX_METADATA_VALUE_LENGTH} characters.`
    }
    seen.add(key)
  }
  return undefined
}

interface MetadataEditorProps {
  rows: readonly MetadataRow[]
  onChange: (rows: MetadataRow[]) => void
  error?: string | undefined
  /** Accessible name prefix for the inputs, such as "Ledger metadata". */
  label?: string
}

export function MetadataEditor({ rows, onChange, error, label = 'Metadata' }: MetadataEditorProps) {
  const update = (index: number, patch: Partial<MetadataRow>) => onChange(rows.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  return (
    <div className="flex flex-col gap-2">
      {rows.map((row, index) => (
        // Rows are positional and edited in place; a key can be empty while typing.
        // oxlint-disable-next-line react/no-array-index-key
        <div key={index} className="flex items-center gap-2">
          <Input
            aria-label={`${label} key ${index + 1}`}
            placeholder="key"
            value={row.key}
            onChange={(event) => update(index, { key: event.target.value })}
            className="w-[180px] shrink-0 font-mono"
          />
          <Input
            aria-label={`${label} value ${index + 1}`}
            placeholder="value"
            value={row.value}
            onChange={(event) => update(index, { value: event.target.value })}
            className="font-mono"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`Remove ${row.key || `row ${index + 1}`}`}
            onClick={() => onChange(rows.filter((_, i) => i !== index))}
          >
            <XIcon />
          </Button>
        </div>
      ))}
      <div>
        <Button
          type="button"
          variant="ghost"
          onClick={() => onChange([...rows, { key: '', value: '' }])}
          disabled={rows.length >= MAX_METADATA_ENTRIES}
        >
          <PlusIcon />
          Add field
        </Button>
      </div>
      {error ? <FieldError>{error}</FieldError> : null}
    </div>
  )
}
