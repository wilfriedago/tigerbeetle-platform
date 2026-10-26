import { useQuery } from '@tanstack/react-query'

import { PlusIcon, XIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'

import { ledgerQueries } from '~/features/ledgers/api'

import { LEDGER_ROLES } from './roles'

/** One ledger role being chosen; id keys the row until a ledger is picked. */
export interface LedgerRoleRow {
  id: number
  ledger: string
  role: string
}

/** The rows that are complete, as the API takes them. */
export function chosenLedgerRoles(rows: readonly LedgerRoleRow[]): { ledger: string; role: string }[] {
  return rows.filter((r) => r.ledger && r.role).map(({ ledger, role }) => ({ ledger, role }))
}

/**
 * A list of ledger and role pairs, as the invite dialog and the credential forms of
 * .design/auth.pen draw them: two selects per row, a remove button, and "Add a ledger role".
 */
export function LedgerRoleRows({ rows, onChange }: { rows: readonly LedgerRoleRow[]; onChange: (rows: LedgerRoleRow[]) => void }) {
  const ledgers = useQuery(ledgerQueries.list())
  const names = (ledgers.data?.data ?? []).map((l) => l.name)
  const update = (i: number, patch: Partial<LedgerRoleRow>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  return (
    <div className="flex flex-col gap-2">
      {rows.map((row, i) => (
        <div key={row.id} className="flex items-center gap-2">
          <Select
            items={names.map((n) => ({ label: n, value: n }))}
            value={row.ledger}
            onValueChange={(ledger) => update(i, { ledger: ledger ?? '' })}
          >
            <SelectTrigger className="flex-1 font-mono" aria-label="Ledger">
              <SelectValue placeholder="Ledger" />
            </SelectTrigger>
            <SelectContent>
              {names.map((n) => (
                <SelectItem key={n} value={n} className="font-mono">
                  {n}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            items={LEDGER_ROLES.map((r) => ({ label: r, value: r }))}
            value={row.role}
            onValueChange={(role) => update(i, { role: role ?? '' })}
          >
            <SelectTrigger className="w-36" aria-label="Role">
              <SelectValue placeholder="Role" />
            </SelectTrigger>
            <SelectContent>
              {LEDGER_ROLES.map((r) => (
                <SelectItem key={r} value={r}>
                  {r}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label="Remove this ledger role"
            onClick={() => onChange(rows.filter((_, j) => j !== i))}
          >
            <XIcon />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="link"
        className="self-start"
        onClick={() => onChange([...rows, { id: (rows.at(-1)?.id ?? 0) + 1, ledger: '', role: 'viewer' }])}
      >
        <PlusIcon />
        Add a ledger role
      </Button>
    </div>
  )
}
