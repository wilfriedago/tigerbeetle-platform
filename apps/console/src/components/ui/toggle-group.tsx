import { Toggle as TogglePrimitive } from '@base-ui/react/toggle'
import { ToggleGroup as ToggleGroupPrimitive } from '@base-ui/react/toggle-group'

import { toggleVariants } from '~/components/ui/toggle'

import { cn } from '~/lib/utils'

// 13 Navigation, Segmented Control: a sunken track with a 2 px inset holding the options.
function ToggleGroup({ className, ...props }: ToggleGroupPrimitive.Props) {
  return (
    <ToggleGroupPrimitive
      data-slot="toggle-group"
      className={cn('group/toggle-group flex w-fit items-center gap-0.5 rounded-md border border-border bg-surface-sunken p-0.5', className)}
      {...props}
    />
  )
}

function ToggleGroupItem({ className, ...props }: TogglePrimitive.Props) {
  return (
    <TogglePrimitive
      data-slot="toggle-group-item"
      className={(state) => toggleVariants({ className: ['focus-visible:z-10', typeof className === 'function' ? className(state) : className] })}
      {...props}
    />
  )
}

export { ToggleGroup, ToggleGroupItem }
