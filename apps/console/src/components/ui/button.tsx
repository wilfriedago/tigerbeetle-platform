import { Button as ButtonPrimitive } from '@base-ui/react/button'

import { tv, type VariantProps } from '~/lib/utils'

import { Spinner } from './spinner'

// 10 Buttons: one primary action per section, labels are verbs in sentence case. Hover and
// pressed shift the fill one step, focus adds the 3 px ring outside, disabled is 45 percent
// opacity, loading swaps the leading icon for a spinner and keeps the width.
const buttonVariants = tv({
  base: "group/button inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md border border-transparent font-medium whitespace-nowrap transition-colors outline-none select-none focus-visible:ring-3 focus-visible:ring-focus-ring disabled:pointer-events-none disabled:opacity-45 aria-disabled:pointer-events-none aria-disabled:opacity-45 aria-invalid:border-danger-fg [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  variants: {
    variant: {
      default: 'bg-primary text-primary-foreground hover:bg-primary-hover active:bg-primary-pressed',
      secondary:
        "border-border-strong bg-surface text-foreground hover:bg-surface-hover active:bg-surface-pressed aria-expanded:bg-surface-hover [&_svg:not([class*='text-'])]:text-muted-foreground",
      ghost:
        "text-foreground hover:bg-surface-hover active:bg-surface-pressed aria-expanded:bg-surface-hover [&_svg:not([class*='text-'])]:text-muted-foreground",
      destructive: 'bg-danger text-danger-on hover:bg-danger-hover active:bg-danger-hover focus-visible:ring-danger-border',
      link: 'text-primary-text hover:underline hover:underline-offset-4'
    },
    size: {
      default: 'h-8 px-3 text-body',
      sm: "h-7 gap-1 px-2.5 text-small [&_svg:not([class*='size-'])]:size-3.5",
      lg: 'h-10 px-4 text-body',
      icon: 'size-8',
      'icon-sm': 'size-7',
      'icon-xs': "size-6 rounded-sm [&_svg:not([class*='size-'])]:size-3.5"
    },
    // Loading disables the button without dimming it and hides the leading icon behind the spinner.
    loading: {
      true: 'disabled:opacity-100 [&>svg:first-child:not([data-slot=spinner])]:hidden'
    }
  },
  // A link sits inline with text, so it drops the control height and padding at every size.
  compoundVariants: [{ className: 'h-auto px-0', variant: 'link' }],
  defaultVariants: {
    size: 'default',
    variant: 'default'
  }
})

type ButtonProps = ButtonPrimitive.Props &
  VariantProps<typeof buttonVariants> & {
    /** Shows a spinner in place of the leading icon and blocks clicks without changing the width. */
    loading?: boolean
  }

function Button({
  className,
  variant = 'default',
  size = 'default',
  loading = false,
  disabled,
  children,
  render,
  nativeButton,
  ...props
}: ButtonProps) {
  return (
    <ButtonPrimitive
      // A button rendered as a link (render={<Link />}) keeps link semantics instead of button ones.
      // A render target that is itself a native button (a Base UI trigger) passes nativeButton.
      nativeButton={nativeButton ?? render === undefined}
      render={render}
      data-slot="button"
      data-loading={loading || undefined}
      aria-busy={loading || undefined}
      disabled={disabled === true || loading}
      // Base UI accepts className as a function of the button state; resolve it before merging.
      className={(state) => buttonVariants({ size, variant, loading, className: typeof className === 'function' ? className(state) : className })}
      {...props}
    >
      {loading ? <Spinner className="text-current" /> : null}
      {children}
    </ButtonPrimitive>
  )
}

export { Button, buttonVariants }
