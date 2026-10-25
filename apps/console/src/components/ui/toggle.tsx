import { Toggle as TogglePrimitive } from '@base-ui/react/toggle'

import { tv, type VariantProps } from '~/lib/utils'

// The option of a Segmented Control (13 Navigation): 26 px, 13 px label with a 14 px icon. The
// pressed option sits on the surface with a hairline; the others are muted on the sunken track.
const toggleVariants = tv({
  base: "group/toggle inline-flex h-6.5 items-center justify-center gap-1.5 rounded-sm border border-transparent px-2.5 text-small whitespace-nowrap text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-focus-ring disabled:pointer-events-none disabled:opacity-45 data-pressed:border-border data-pressed:bg-surface data-pressed:font-medium data-pressed:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3.5",
  variants: {
    variant: {
      default: ''
    }
  },
  defaultVariants: {
    variant: 'default'
  }
})

function Toggle({ className, variant = 'default', ...props }: TogglePrimitive.Props & VariantProps<typeof toggleVariants>) {
  return (
    <TogglePrimitive
      data-slot="toggle"
      className={(state) => toggleVariants({ variant, className: typeof className === 'function' ? className(state) : className })}
      {...props}
    />
  )
}

export { Toggle, toggleVariants }
