import { parseAsset } from '~/lib/money'
import { cn } from '~/lib/utils'

// 20 Money and identity, Asset Badge: always the canonical CODE/SCALE form (ADR-0005), so a
// reader can interpret an amount without looking anything up. The scale is quieter than the code.
export function AssetBadge({ asset, className }: { asset: string; className?: string }) {
  const parsed = parseAsset(asset)
  return (
    <span
      className={cn(
        'inline-flex h-[18px] items-center rounded-sm border border-border bg-surface-sunken px-1.5 font-mono text-caption leading-none whitespace-nowrap',
        className
      )}
    >
      <span className="font-medium text-foreground">{parsed?.code ?? asset}</span>
      {parsed ? <span className="text-subtle-foreground">/{parsed.scale}</span> : null}
    </span>
  )
}
