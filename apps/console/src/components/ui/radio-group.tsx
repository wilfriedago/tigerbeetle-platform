import { Radio as RadioPrimitive } from '@base-ui/react/radio'
import { RadioGroup as RadioGroupPrimitive } from '@base-ui/react/radio-group'

import { cn } from '~/lib/utils'

function RadioGroup({ className, ...props }: RadioGroupPrimitive.Props) {
  return <RadioGroupPrimitive data-slot="radio-group" className={cn('grid w-full gap-2', className)} {...props} />
}

// 16 px dot; the selected state fills with the accent and shows a 6 px inner dot.
function RadioGroupItem({ className, ...props }: RadioPrimitive.Root.Props) {
  return (
    <RadioPrimitive.Root
      data-slot="radio-group-item"
      className={cn(
        'group/radio-group-item peer relative flex aspect-square size-4 shrink-0 items-center justify-center rounded-full border border-input bg-surface outline-none after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:ring-3 focus-visible:ring-focus-ring data-disabled:cursor-not-allowed data-disabled:opacity-45 aria-invalid:border-danger-fg data-checked:border-primary data-checked:bg-primary',
        className
      )}
      {...props}
    >
      <RadioPrimitive.Indicator data-slot="radio-group-indicator" className="size-1.5 rounded-full bg-primary-foreground" />
    </RadioPrimitive.Root>
  )
}

export { RadioGroup, RadioGroupItem }
