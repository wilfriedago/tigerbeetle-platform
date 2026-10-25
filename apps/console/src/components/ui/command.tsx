import * as React from 'react'

import { Command as CommandPrimitive } from 'cmdk'
import { SearchIcon } from 'lucide-react'

import { Dialog, DialogContent, DialogDescription, DialogTitle } from '~/components/ui/dialog'
import { Kbd } from '~/components/ui/kbd'

import { KEY_GLYPHS } from '~/lib/key-glyphs'
import { cn } from '~/lib/utils'

// 14 Overlays, Command palette: 640 px at level 2. A 48 px mono search row, grouped results with
// 36 px items (mono label, hint, enter key on the active item) and a sunken footer with the
// keyboard hints.

function Command({ className, ...props }: React.ComponentProps<typeof CommandPrimitive>) {
  return (
    <CommandPrimitive
      data-slot="command"
      className={cn('flex size-full flex-col overflow-hidden bg-popover text-popover-foreground', className)}
      {...props}
    />
  )
}

function CommandDialog({
  title = 'Search',
  description = 'Search IDs, addresses, references and Idempotency-Keys.',
  children,
  className,
  ...props
}: Omit<React.ComponentProps<typeof Dialog>, 'children'> & {
  title?: string
  description?: string
  className?: string
  children: React.ReactNode
}) {
  return (
    <Dialog {...props}>
      <DialogContent className={cn('top-[120px] translate-y-0 p-0 sm:max-w-[640px]', className)} showCloseButton={false}>
        <DialogTitle className="sr-only">{title}</DialogTitle>
        <DialogDescription className="sr-only">{description}</DialogDescription>
        {children}
      </DialogContent>
    </Dialog>
  )
}

function CommandInput({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Input>) {
  return (
    <div data-slot="command-input-wrapper" className="flex h-12 items-center gap-2.5 border-b border-border px-4">
      <SearchIcon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      <CommandPrimitive.Input
        data-slot="command-input"
        className={cn(
          'h-full w-full bg-transparent font-mono text-body text-foreground outline-hidden placeholder:font-sans placeholder:text-subtle-foreground disabled:cursor-not-allowed disabled:opacity-45',
          className
        )}
        {...props}
      />
      <Kbd>esc</Kbd>
    </div>
  )
}

function CommandList({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.List>) {
  return (
    <CommandPrimitive.List
      data-slot="command-list"
      className={cn('no-scrollbar max-h-[400px] scroll-py-2 overflow-x-hidden overflow-y-auto p-2 outline-none', className)}
      {...props}
    />
  )
}

function CommandEmpty({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Empty>) {
  return (
    <CommandPrimitive.Empty data-slot="command-empty" className={cn('py-8 text-center text-small text-muted-foreground', className)} {...props} />
  )
}

function CommandGroup({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Group>) {
  return (
    <CommandPrimitive.Group
      data-slot="command-group"
      className={cn(
        'overflow-hidden text-foreground **:[[cmdk-group-heading]]:px-2 **:[[cmdk-group-heading]]:pt-2 **:[[cmdk-group-heading]]:pb-1 **:[[cmdk-group-heading]]:text-caption **:[[cmdk-group-heading]]:font-medium **:[[cmdk-group-heading]]:text-subtle-foreground',
        className
      )}
      {...props}
    />
  )
}

function CommandSeparator({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Separator>) {
  return <CommandPrimitive.Separator data-slot="command-separator" className={cn('-mx-2 my-1 h-px bg-border', className)} {...props} />
}

function CommandItem({ className, children, ...props }: React.ComponentProps<typeof CommandPrimitive.Item>) {
  return (
    <CommandPrimitive.Item
      data-slot="command-item"
      className={cn(
        "group/command-item relative flex h-9 cursor-default items-center gap-2.5 rounded-md px-2.5 text-small outline-hidden select-none data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-45 data-selected:bg-surface-selected [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='text-'])]:text-muted-foreground [&_svg:not([class*='size-'])]:size-4 data-selected:[&_svg:not([class*='text-'])]:text-primary-text",
        className
      )}
      {...props}
    >
      {children}
    </CommandPrimitive.Item>
  )
}

/** The hint on the right of an item, plus the enter key on the selected item. */
function CommandShortcut({ className, children, ...props }: React.ComponentProps<'span'>) {
  return (
    <span
      data-slot="command-shortcut"
      className={cn('ml-auto flex items-center gap-2 font-sans text-caption text-subtle-foreground', className)}
      {...props}
    >
      {children}
      <Kbd className="hidden group-data-selected/command-item:inline-flex">{KEY_GLYPHS.enter}</Kbd>
    </span>
  )
}

/** The sunken footer with keyboard hints. */
function CommandFooter({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="command-footer"
      className={cn(
        'flex h-9 items-center gap-4 border-t border-border bg-surface-sunken px-4 text-caption text-muted-foreground [&>span]:flex [&>span]:items-center [&>span]:gap-1.5',
        className
      )}
      {...props}
    />
  )
}

export {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandFooter,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut
}
