import { cn } from '~/lib/utils'

// 15 Data display, Description Item: record headers and drawers. 160 px muted term, the value on
// the right, a subtle hairline between rows; `mono` for machine values.

export function DescriptionList({ className, ...props }: React.ComponentProps<'dl'>) {
  return <dl className={cn('flex flex-col', className)} {...props} />
}

interface DescriptionItemProps {
  term: React.ReactNode
  children: React.ReactNode
  mono?: boolean
  className?: string
  /** Width of the term column; the design uses 160 px in drawers and 120 px in side cards. */
  termWidth?: 'sm' | 'md'
}

export function DescriptionItem({ term, children, mono = false, termWidth = 'md', className }: DescriptionItemProps) {
  return (
    <div className={cn('flex items-baseline gap-4 border-b border-border-subtle py-2.5 last:border-b-0', className)}>
      <dt className={cn('shrink-0 text-small text-muted-foreground', termWidth === 'md' ? 'w-40' : 'w-[120px]')}>{term}</dt>
      <dd className={cn('min-w-0 flex-1 text-small break-words text-foreground', mono && 'font-mono')}>{children}</dd>
    </div>
  )
}
