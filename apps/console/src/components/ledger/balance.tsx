import { isNegative, percentOf, subtractAmounts } from '~/lib/money'
import { cn } from '~/lib/utils'

import { Amount, AmountDisplay } from './amount'
import { AssetBadge } from './asset-badge'
import { IdText } from './id-text'

// 21 Ledger patterns, Balances. Balance is credits minus debits; reserved is pending debits (open
// holds, ADR-0010); available is balance minus reserved. Incoming pending credits are shown but not
// added to available, because a hold can still be voided. Volumes are the totals TigerBeetle keeps
// per account.

interface BalanceBarProps {
  /** Credits minus debits, posted only. */
  balance: string
  /** Pending debits. */
  reserved: string
  className?: string
}

/** No axis and no totals: the share of the balance that is reserved. Negative balances get no bar. */
export function BalanceBar({ balance, reserved, className }: BalanceBarProps) {
  if (isNegative(balance)) {
    return null
  }
  const available = subtractAmounts(balance, reserved)
  const availablePct = percentOf(available, balance)
  const reservedPct = percentOf(reserved, balance)
  return (
    <div className={cn('flex h-1.5 w-full gap-0.5 overflow-hidden rounded-full bg-bar-track', className)} aria-hidden="true">
      {availablePct > 0 ? <div className="h-full rounded-l-full bg-bar-available" style={{ width: `${availablePct}%` }} /> : null}
      {reservedPct > 0 ? <div className="h-full bg-bar-reserved" style={{ width: `${reservedPct}%` }} /> : null}
    </div>
  )
}

export interface AssetBalance {
  asset: string
  balance: string
  reserved: string
  incoming: string
  credits: string
  debits: string
  accountId?: string
}

function LegendRow({ swatch, label, value, asset }: { swatch: string; label: React.ReactNode; value: string; asset: string }) {
  return (
    <div className="flex items-center gap-2">
      <span aria-hidden="true" className={cn('size-2 shrink-0 rounded-full', swatch)} />
      <span className="flex-1 text-small text-muted-foreground">{label}</span>
      <Amount value={value} asset={asset} />
    </div>
  )
}

interface BalanceCardProps {
  balance: AssetBalance
  /** "Reserved by 2 holds" when the count is known. */
  holds?: number | undefined
  className?: string
}

export function BalanceCard({ balance, holds, className }: BalanceCardProps) {
  const available = subtractAmounts(balance.balance, balance.reserved)
  let reservedLabel = 'Reserved'
  if (holds !== undefined) {
    reservedLabel = holds === 0 ? 'Reserved, no holds' : `Reserved by ${holds} hold${holds === 1 ? '' : 's'}`
  }
  return (
    <div className={cn('flex flex-col gap-4 rounded-lg border border-border bg-surface p-5', className)}>
      <div className="flex items-center gap-2">
        <AssetBadge asset={balance.asset} />
        <span className="flex-1 text-caption text-muted-foreground">Balance</span>
        {balance.accountId ? <IdText value={balance.accountId} label="Copy account ID" /> : null}
      </div>
      <AmountDisplay value={balance.balance} asset={balance.asset} />
      <BalanceBar balance={balance.balance} reserved={balance.reserved} />
      <div className="flex flex-col gap-2">
        <LegendRow swatch="bg-bar-available" label="Available" value={available} asset={balance.asset} />
        <LegendRow swatch="bg-bar-reserved" label={reservedLabel} value={balance.reserved} asset={balance.asset} />
        <LegendRow swatch="border border-border-strong bg-bar-track" label="Incoming, pending" value={balance.incoming} asset={balance.asset} />
      </div>
      <div className="flex flex-col gap-2 border-t border-border-subtle pt-3">
        <div className="flex items-center gap-2">
          <span className="flex-1 text-small text-muted-foreground">Total credits</span>
          <Amount value={balance.credits} asset={balance.asset} />
        </div>
        <div className="flex items-center gap-2">
          <span className="flex-1 text-small text-muted-foreground">Total debits</span>
          <Amount value={balance.debits} asset={balance.asset} />
        </div>
      </div>
    </div>
  )
}
