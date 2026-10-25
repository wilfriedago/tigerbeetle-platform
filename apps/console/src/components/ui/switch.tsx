import { Switch as SwitchPrimitive } from '@base-ui/react/switch'

import { cn } from '~/lib/utils'

// 28 x 16 track with 2 px padding and a 12 px knob. Off uses border-strong, on uses the accent.
function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'peer group/switch relative inline-flex h-4 w-7 shrink-0 items-center rounded-full p-0.5 transition-colors outline-none after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:ring-3 focus-visible:ring-focus-ring data-checked:bg-primary data-unchecked:bg-border-strong data-disabled:cursor-not-allowed data-disabled:opacity-45',
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block size-3 rounded-full bg-surface transition-transform data-checked:translate-x-3 data-unchecked:translate-x-0"
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
