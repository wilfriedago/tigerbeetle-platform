import { cn } from '~/lib/utils'

// 12 Feedback, Empty State: empty states teach the concept and offer the next action. Surface
// card with a 40 px icon box, a 16 px title, a short explanation and up to two actions.

function Empty({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="empty"
      className={cn(
        'flex w-full min-w-0 flex-col items-center justify-center gap-3 rounded-lg border border-border bg-surface px-6 py-10 text-center',
        className
      )}
      {...props}
    />
  )
}

function EmptyHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="empty-header" className={cn('flex max-w-[380px] flex-col items-center gap-3', className)} {...props} />
}

function EmptyMedia({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="empty-icon"
      className={cn(
        "flex size-10 shrink-0 items-center justify-center rounded-md border border-border bg-surface-sunken text-muted-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-5",
        className
      )}
      {...props}
    />
  )
}

function EmptyTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="empty-title" className={cn('text-card-title text-foreground', className)} {...props} />
}

function EmptyDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="empty-description"
      className={cn(
        'text-small leading-normal text-muted-foreground [&>a]:text-primary-text [&>a]:underline-offset-4 [&>a:hover]:underline',
        className
      )}
      {...props}
    />
  )
}

function EmptyContent({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="empty-content" className={cn('flex flex-wrap items-center justify-center gap-2', className)} {...props} />
}

export { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle }
