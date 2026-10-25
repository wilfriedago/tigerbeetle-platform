import { mergeProps } from '@base-ui/react/merge-props'
import { useRender } from '@base-ui/react/use-render'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '~/lib/utils'

// 12 Feedback: status is always a dot plus a word, never colour alone. 12 px medium label, 6 px
// dot, 4 px radius, one tinted surface and border per status.
const badgeVariants = cva(
  'group/badge inline-flex h-5 w-fit shrink-0 items-center gap-1.5 overflow-hidden rounded-sm border px-2 text-caption leading-none font-medium whitespace-nowrap [&>svg]:pointer-events-none [&>svg]:size-3',
  {
    variants: {
      variant: {
        neutral: 'border-neutral-border bg-neutral-bg text-neutral-fg',
        muted: 'border-border-subtle bg-quiet-bg text-quiet-fg',
        accent: 'border-primary-border bg-primary-subtle text-primary-text',
        success: 'border-success-border bg-success-bg text-success-fg',
        warning: 'border-warning-border bg-warning-bg text-warning-fg',
        danger: 'border-danger-border bg-danger-bg text-danger-fg'
      }
    },
    defaultVariants: {
      variant: 'neutral'
    }
  }
)

type BadgeProps = useRender.ComponentProps<'span'> &
  VariantProps<typeof badgeVariants> & {
    /** The status dot. On by default; turn it off for counts and labels that are not a status. */
    dot?: boolean
  }

function Badge({ className, variant = 'neutral', dot = true, render, children, ...props }: BadgeProps) {
  return useRender({
    defaultTagName: 'span',
    props: mergeProps<'span'>(
      {
        children: (
          <>
            {dot ? <span aria-hidden="true" data-slot="badge-dot" className="size-1.5 shrink-0 rounded-full bg-current" /> : null}
            {children}
          </>
        ),
        className: cn(badgeVariants({ variant }), className)
      },
      props
    ),
    render,
    state: {
      slot: 'badge',
      variant
    }
  })
}

export { Badge, badgeVariants }
export type { BadgeProps }
