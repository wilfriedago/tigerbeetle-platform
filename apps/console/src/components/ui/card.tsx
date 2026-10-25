import { cn } from '~/lib/utils'

// 15 Data display, Card: groups one idea. Surface, 1 px border, 8 px radius. The header carries
// a 14 px semibold title, a 12 px description and actions, separated from the body by a subtle
// hairline. Cards are not wrappers for everything.
function Card({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card"
      className={cn('group/card flex flex-col overflow-hidden rounded-lg border border-border bg-card text-card-foreground', className)}
      {...props}
    />
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        'group/card-header grid auto-rows-min items-center gap-x-3 gap-y-0.5 border-b border-border-subtle px-4 py-3.5 has-data-[slot=card-action]:grid-cols-[1fr_auto]',
        className
      )}
      {...props}
    />
  )
}

function CardTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="card-title" className={cn('text-body leading-snug font-semibold text-foreground', className)} {...props} />
}

function CardDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="card-description" className={cn('text-caption text-muted-foreground', className)} {...props} />
}

function CardAction({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-action"
      className={cn('col-start-2 row-span-2 row-start-1 flex items-center gap-2 justify-self-end', className)}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="card-content" className={cn('p-4', className)} {...props} />
}

function CardFooter({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div data-slot="card-footer" className={cn('flex items-center gap-2 border-t border-border bg-surface-sunken px-5 py-3', className)} {...props} />
  )
}

export { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle }
