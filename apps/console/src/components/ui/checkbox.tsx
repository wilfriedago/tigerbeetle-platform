import { Checkbox as CheckboxPrimitive } from '@base-ui/react/checkbox'
import { CheckIcon, MinusIcon } from 'lucide-react'

import { cn } from '~/lib/utils'

// 16 px box, 4 px radius. Checked and mixed use the accent; focus puts the ring around the box,
// not the label; disabled is 45 percent opacity.
function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        'peer relative flex size-4 shrink-0 items-center justify-center rounded-sm border border-input bg-surface text-primary-foreground transition-colors outline-none after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:ring-3 focus-visible:ring-focus-ring disabled:cursor-not-allowed data-disabled:opacity-45 aria-invalid:border-danger-fg data-checked:border-primary data-checked:bg-primary data-indeterminate:border-primary data-indeterminate:bg-primary',
        className
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="group/checkbox-indicator grid place-content-center text-current transition-none [&>svg]:size-3 [&>svg]:stroke-[2.5]"
        keepMounted={false}
        render={(indicatorProps, state) => <span {...indicatorProps}>{state.indeterminate ? <MinusIcon /> : <CheckIcon />}</span>}
      />
    </CheckboxPrimitive.Root>
  )
}

export { Checkbox }
