import * as React from 'react'

import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '~/lib/utils'

// 12 Feedback, Alerts: inline, full width of their container, used to explain a ledger rule where
// it applies. Icon in the status colour, a semibold title, a muted message and an optional link
// action on the right.
const alertVariants = cva(
  "group/alert relative grid w-full grid-cols-[1fr_auto] gap-x-3 gap-y-0.5 rounded-md border px-3.5 py-3 text-left has-[>svg]:grid-cols-[auto_1fr_auto] *:[svg]:row-span-2 *:[svg]:mt-0.5 *:[svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        info: 'border-primary-border bg-primary-subtle *:[svg]:text-primary-text',
        success: 'border-success-border bg-success-bg *:[svg]:text-success-fg',
        warning: 'border-warning-border bg-warning-bg *:[svg]:text-warning-fg',
        danger: 'border-danger-border bg-danger-bg *:[svg]:text-danger-fg'
      }
    },
    defaultVariants: {
      variant: 'info'
    }
  }
)

function Alert({ className, variant, ...props }: React.ComponentProps<'div'> & VariantProps<typeof alertVariants>) {
  return <div data-slot="alert" role={variant === 'danger' ? 'alert' : 'status'} className={cn(alertVariants({ variant }), className)} {...props} />
}

function AlertTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div data-slot="alert-title" className={cn('text-body font-semibold text-foreground group-has-[>svg]/alert:col-start-2', className)} {...props} />
  )
}

function AlertDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-description"
      className={cn(
        'text-small text-pretty text-muted-foreground group-has-[>svg]/alert:col-start-2 [&_a]:text-primary-text [&_a]:underline-offset-3 [&_a]:hover:underline [&_p:not(:last-child)]:mb-2',
        className
      )}
      {...props}
    />
  )
}

/** A link or button placed on the right of the title row. */
function AlertAction({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-action"
      className={cn('col-start-3 row-span-2 row-start-1 self-start text-small font-medium text-primary-text', className)}
      {...props}
    />
  )
}

export { Alert, AlertAction, AlertDescription, AlertTitle }
