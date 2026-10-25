import * as React from 'react'

import { cva, type VariantProps } from 'class-variance-authority'

import { Button } from '~/components/ui/button'
import { Input } from '~/components/ui/input'
import { Textarea } from '~/components/ui/textarea'

import { cn } from '~/lib/utils'

// An Input with a leading icon, a suffix (an asset such as EUR/2) or a trailing button. The group
// owns the outline so hover, focus and error look exactly like Input.
function InputGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="input-group"
      role="group"
      className={cn(
        'group/input-group relative flex h-8 w-full min-w-0 items-center rounded-md border border-input bg-surface transition-[color,border-color,box-shadow] outline-none hover:border-subtle-foreground has-disabled:border-border has-disabled:bg-surface-sunken has-[[data-slot=input-group-control]:focus-visible]:border-primary has-[[data-slot=input-group-control]:focus-visible]:ring-3 has-[[data-slot=input-group-control]:focus-visible]:ring-focus-ring has-[[data-slot][aria-invalid=true]]:border-danger-fg has-[>[data-align=block-end]]:h-auto has-[>[data-align=block-end]]:flex-col has-[>[data-align=block-start]]:h-auto has-[>[data-align=block-start]]:flex-col has-[>textarea]:h-auto has-[>[data-align=inline-end]]:[&>input]:pr-1 has-[>[data-align=inline-start]]:[&>input]:pl-2 in-data-[slot=combobox-content]:focus-within:ring-0',
        className
      )}
      {...props}
    />
  )
}

const inputGroupAddonVariants = cva(
  "flex h-auto cursor-text items-center justify-center gap-2 text-small text-subtle-foreground select-none group-data-[disabled=true]/input-group:opacity-45 [&>svg:not([class*='size-'])]:size-4",
  {
    variants: {
      align: {
        'inline-start': 'order-first pl-2.5 has-[>button]:pl-1',
        'inline-end': 'order-last pr-2 has-[>button]:pr-1 has-[>kbd]:pr-1.5',
        'block-start': 'order-first w-full justify-start px-2.5 pt-2',
        'block-end': 'order-last w-full justify-start px-2.5 pb-2'
      }
    },
    defaultVariants: {
      align: 'inline-start'
    }
  }
)

function InputGroupAddon({
  className,
  align = 'inline-start',
  ...props
}: React.ComponentProps<'div'> & VariantProps<typeof inputGroupAddonVariants>) {
  return (
    // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions -- a mouse convenience; the input stays keyboard reachable on its own
    <div
      data-slot="input-group-addon"
      data-align={align}
      className={cn(inputGroupAddonVariants({ align }), className)}
      onClick={(event) => {
        if ((event.target as HTMLElement).closest('button')) {
          return
        }
        event.currentTarget.parentElement?.querySelector('input')?.focus()
      }}
      {...props}
    />
  )
}

function InputGroupButton({
  className,
  type = 'button',
  variant = 'ghost',
  size = 'icon-xs',
  ...props
}: Omit<React.ComponentProps<typeof Button>, 'type'> & { type?: 'button' | 'submit' | 'reset' }) {
  return <Button type={type} variant={variant} size={size} className={cn('shadow-none', className)} {...props} />
}

function InputGroupText({ className, ...props }: React.ComponentProps<'span'>) {
  return (
    <span
      className={cn(
        "flex items-center gap-2 font-mono text-caption text-subtle-foreground [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    />
  )
}

function InputGroupInput({ className, ...props }: React.ComponentProps<'input'>) {
  return (
    <Input
      data-slot="input-group-control"
      className={cn(
        'h-full flex-1 rounded-none border-0 bg-transparent shadow-none ring-0 hover:border-0 focus-visible:ring-0 disabled:bg-transparent read-only:bg-transparent',
        className
      )}
      {...props}
    />
  )
}

function InputGroupTextarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <Textarea
      data-slot="input-group-control"
      className={cn(
        'flex-1 resize-none rounded-none border-0 bg-transparent py-2 shadow-none ring-0 focus-visible:ring-0 disabled:bg-transparent',
        className
      )}
      {...props}
    />
  )
}

export { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput, InputGroupText, InputGroupTextarea }
