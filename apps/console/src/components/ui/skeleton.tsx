import { cn } from '~/lib/utils'

// Skeletons keep the layout while data loads; the console never shows a full-page spinner.
function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="skeleton" className={cn('h-3 animate-pulse rounded-xs bg-skeleton', className)} {...props} />
}

export { Skeleton }
