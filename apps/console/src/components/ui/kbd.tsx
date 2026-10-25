import { cn } from '~/lib/utils'

function Kbd({ className, ...props }: React.ComponentProps<'kbd'>) {
  return (
    <kbd
      data-slot="kbd"
      className={cn(
        "pointer-events-none inline-flex h-[18px] w-fit min-w-5 items-center justify-center gap-1 rounded-sm border border-border-strong bg-surface px-[5px] font-mono text-[11px] leading-none text-muted-foreground select-none in-data-[slot=tooltip-content]:border-transparent in-data-[slot=tooltip-content]:bg-white/15 in-data-[slot=tooltip-content]:text-tooltip-foreground [&_svg:not([class*='size-'])]:size-3",
        className
      )}
      {...props}
    />
  )
}

function KbdGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return <kbd data-slot="kbd-group" className={cn('inline-flex items-center gap-1', className)} {...props} />
}

export { Kbd, KbdGroup }
