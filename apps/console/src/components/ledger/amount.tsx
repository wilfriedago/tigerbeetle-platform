import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip'

import { assetCode, assetScale, formatAmount } from '~/lib/money'
import { usePreferences } from '~/lib/preferences'
import { cn } from '~/lib/utils'

// 20 Money and identity, Amount: the integer string formatted with the asset scale, the asset code
// after it in a muted tone, the raw integer one hover away so nobody has to trust the formatting.
// Never coloured by sign, never rounded, never animated (02 Typography, Numbers).

interface AmountProps {
  /** Integer string in the smallest unit; may be negative for balances. */
  value: string
  /** Canonical asset, for example "EUR/2". */
  asset: string
  /** Hide the asset code, for columns that already name the asset in their header. */
  hideAsset?: boolean
  className?: string
}

/** The amount as text in the chosen unit (Preferences: major units or raw integers), without the code. */
export function useAmountText(value: string, asset: string): string {
  const { amounts } = usePreferences()
  const scale = assetScale(asset)
  return amounts === 'raw' ? value : formatAmount(value, scale)
}

/** Amount for tables and lists: mono 13 px, right-aligned by its cell. */
export function Amount({ value, asset, hideAsset = false, className }: AmountProps) {
  const text = useAmountText(value, asset)
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span className={cn('inline-flex items-baseline gap-1 font-mono text-small whitespace-nowrap text-foreground tabular-nums', className)} />
        }
      >
        <span>{text}</span>
        {hideAsset ? null : <span className="text-[11px] text-subtle-foreground">{assetCode(asset)}</span>}
      </TooltipTrigger>
      <TooltipContent>
        <span className="font-mono">{value}</span>
        <span className="opacity-70">({asset}, smallest unit)</span>
      </TooltipContent>
    </Tooltip>
  )
}

/** Amount/Display for balances and headline figures: sans 30 px semibold, asset in mono. */
export function AmountDisplay({ value, asset, className, size = 'lg' }: AmountProps & { size?: 'md' | 'lg' }) {
  const text = useAmountText(value, asset)
  return (
    <Tooltip>
      <TooltipTrigger render={<span className={cn('inline-flex items-baseline gap-2 whitespace-nowrap', className)} />}>
        <span className={cn('tabular-nums', size === 'lg' ? 'text-amount-display leading-[1.1]' : 'text-page-title')}>{text}</span>
        <span className={cn('font-mono text-muted-foreground', size === 'lg' ? 'text-body' : 'text-small')}>{assetCode(asset)}</span>
      </TooltipTrigger>
      <TooltipContent>
        <span className="font-mono">{value}</span>
        <span className="opacity-70">({asset}, smallest unit)</span>
      </TooltipContent>
    </Tooltip>
  )
}

/** Formats an amount as text, for places where a component does not fit (labels, toasts). */
export function useFormattedAmount(value: string, asset: string): string {
  return `${useAmountText(value, asset)} ${assetCode(asset)}`
}
