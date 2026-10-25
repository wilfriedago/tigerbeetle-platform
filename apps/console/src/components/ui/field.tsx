import { useMemo } from 'react'

import { CircleXIcon } from 'lucide-react'

import { Label } from '~/components/ui/label'
import { Separator } from '~/components/ui/separator'

import { cn, tv, type VariantProps } from '~/lib/utils'

// 11 Form controls, Field and Field/Error: a visible label (13 px medium) with an optional hint on
// the right, the control, then help text that states the rule the API enforces, in the same words
// as the problem it would return. An error replaces the help with a danger icon and message.

function FieldSet({ className, ...props }: React.ComponentProps<'fieldset'>) {
  return (
    <fieldset
      data-slot="field-set"
      className={cn('flex flex-col gap-4 has-[>[data-slot=checkbox-group]]:gap-3 has-[>[data-slot=radio-group]]:gap-3', className)}
      {...props}
    />
  )
}

function FieldLegend({ className, variant = 'legend', ...props }: React.ComponentProps<'legend'> & { variant?: 'legend' | 'label' }) {
  return (
    <legend
      data-slot="field-legend"
      data-variant={variant}
      className={cn('mb-1.5 font-medium text-foreground data-[variant=label]:text-small data-[variant=legend]:text-body', className)}
      {...props}
    />
  )
}

function FieldGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="field-group"
      className={cn('group/field-group @container/field-group flex w-full flex-col gap-4 *:data-[slot=field-group]:gap-4', className)}
      {...props}
    />
  )
}

const fieldVariants = tv({
  base: 'group/field flex w-full gap-1.5',
  variants: {
    orientation: {
      vertical: 'flex-col *:w-full [&>.sr-only]:w-auto',
      horizontal:
        'flex-row items-center gap-2 has-[>[data-slot=field-content]]:items-start *:data-[slot=field-label]:flex-auto has-[>[data-slot=field-content]]:[&>[role=checkbox],[role=radio]]:mt-0.5'
    }
  },
  defaultVariants: {
    orientation: 'vertical'
  }
})

function Field({ className, orientation = 'vertical', ...props }: React.ComponentProps<'div'> & VariantProps<typeof fieldVariants>) {
  return <div role="group" data-slot="field" data-orientation={orientation} className={fieldVariants({ orientation, className })} {...props} />
}

function FieldContent({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="field-content" className={cn('group/field-content flex flex-1 flex-col gap-0.5 leading-snug', className)} {...props} />
}

/** The label row: the label on the left and an optional hint such as "Optional" on the right. */
function FieldHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="field-header" className={cn('flex items-center justify-between gap-2', className)} {...props} />
}

function FieldHint({ className, ...props }: React.ComponentProps<'span'>) {
  return <span data-slot="field-hint" className={cn('text-caption text-subtle-foreground', className)} {...props} />
}

function FieldLabel({ className, ...props }: React.ComponentProps<typeof Label>) {
  return (
    <Label
      data-slot="field-label"
      className={cn('group/field-label peer/field-label flex w-fit gap-2 leading-snug group-data-[disabled=true]/field:opacity-45', className)}
      {...props}
    />
  )
}

function FieldTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="field-label"
      className={cn('flex w-fit items-center gap-2 text-small font-medium text-foreground group-data-[disabled=true]/field:opacity-45', className)}
      {...props}
    />
  )
}

function FieldDescription({ className, ...props }: React.ComponentProps<'p'>) {
  return (
    <p
      data-slot="field-description"
      className={cn(
        'text-left text-caption leading-[1.4] font-normal text-muted-foreground [[data-variant=legend]+&]:-mt-1.5',
        '[&>a]:text-primary-text [&>a]:underline-offset-4 [&>a:hover]:underline',
        'group-has-data-[slot=field-error]/field:hidden',
        className
      )}
      {...props}
    />
  )
}

function FieldSeparator({ children, className, ...props }: React.ComponentProps<'div'> & { children?: React.ReactNode }) {
  return (
    <div data-slot="field-separator" data-content={Boolean(children)} className={cn('relative -my-2 h-5 text-small', className)} {...props}>
      <Separator className="absolute inset-0 top-1/2" />
      {children ? (
        <span className="relative mx-auto block w-fit bg-background px-2 text-muted-foreground" data-slot="field-separator-content">
          {children}
        </span>
      ) : null}
    </div>
  )
}

type FieldErrorProps = React.ComponentProps<'div'> & {
  /** Errors from a form library. Strings and objects with a message are both accepted. */
  errors?: readonly (string | { message?: string } | undefined | null)[]
}

function FieldError({ className, children, errors, ...props }: FieldErrorProps) {
  const content = useMemo(() => {
    if (children) {
      return children
    }
    const messages = [
      ...new Set((errors ?? []).map((error) => (typeof error === 'string' ? error : error?.message)).filter((message) => Boolean(message)))
    ]
    if (messages.length === 0) {
      return null
    }
    if (messages.length === 1) {
      return messages[0]
    }
    return (
      <ul className="ml-4 flex list-disc flex-col gap-1">
        {messages.map((message) => (
          <li key={message}>{message}</li>
        ))}
      </ul>
    )
  }, [children, errors])

  if (!content) {
    return null
  }

  return (
    <div role="alert" data-slot="field-error" className={cn('flex gap-1.5 text-caption leading-[1.4] text-danger-fg', className)} {...props}>
      <CircleXIcon aria-hidden="true" className="mt-px size-3.5 shrink-0" />
      <div>{content}</div>
    </div>
  )
}

export {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldHeader,
  FieldHint,
  FieldLabel,
  FieldLegend,
  FieldSeparator,
  FieldSet,
  FieldTitle
}
