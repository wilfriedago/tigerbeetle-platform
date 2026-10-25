import { cn } from '~/lib/utils'

// 13 Navigation, Page Header: a 24 px title with a one-line description, actions on the right,
// aligned to the bottom. Machine names (ledgers, addresses) set `mono` on the title.

export function Page({ className, ...props }: React.ComponentProps<'div'>) {
  return <div className={cn('flex w-full max-w-[1440px] flex-col gap-6 px-4 pt-8 pb-16 sm:px-8', className)} {...props} />
}

interface PageHeaderProps {
  title: React.ReactNode
  /** Next to the title: an ID, a status badge. */
  titleAddon?: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  mono?: boolean
  className?: string
}

export function PageHeader({ title, titleAddon, description, actions, mono = false, className }: PageHeaderProps) {
  return (
    <header className={cn('flex flex-wrap items-end justify-between gap-x-6 gap-y-3', className)}>
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          <h1 className={cn('min-w-0 truncate text-page-title text-foreground', mono && 'font-mono font-medium tracking-tight')}>{title}</h1>
          {titleAddon}
        </div>
        {description ? <div className="text-small text-muted-foreground">{description}</div> : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  )
}

/** A section title inside a page, with an optional description on the same line. */
export function SectionTitle({
  title,
  description,
  className,
  actions
}: {
  title: React.ReactNode
  description?: React.ReactNode
  className?: string
  actions?: React.ReactNode
}) {
  return (
    <div className={cn('flex items-baseline gap-2', className)}>
      <h2 className="text-body font-semibold text-foreground">{title}</h2>
      {description ? <span className="text-caption text-muted-foreground">{description}</span> : null}
      {actions ? <div className="ml-auto flex items-center gap-2">{actions}</div> : null}
    </div>
  )
}
