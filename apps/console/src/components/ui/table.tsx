import { cn } from '~/lib/utils'

// 15 Data display, table parts: tables are the main surface. Header cells are 12 px medium on the
// sunken surface; rows follow the density token (row-h). Text left, numbers right (TableHead and
// TableCell take align="right"), IDs and addresses in mono at the call site.

function Table({ className, framed = true, ...props }: React.ComponentProps<'table'> & { framed?: boolean }) {
  return (
    <div
      data-slot="table-container"
      data-framed={framed}
      className={cn('relative w-full overflow-x-auto', framed && 'rounded-lg border border-border bg-surface')}
    >
      <table data-slot="table" className={cn('w-full caption-bottom border-collapse text-small', className)} {...props} />
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
  return <thead data-slot="table-header" className={cn('bg-surface-sunken', className)} {...props} />
}

function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
  return <tbody data-slot="table-body" className={cn('[&_tr:last-child]:border-0', className)} {...props} />
}

function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn('border-t border-border bg-surface-sunken font-medium [&>tr]:last:border-b-0', className)}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<'tr'>) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        'border-b border-border-subtle transition-colors [tbody_&]:hover:bg-surface-hover/60 has-aria-expanded:bg-surface-hover data-[state=selected]:bg-surface-selected [thead_&]:border-border',
        className
      )}
      {...props}
    />
  )
}

type Align = 'left' | 'right' | 'center'

function TableHead({ className, align = 'left', ...props }: Omit<React.ComponentProps<'th'>, 'align'> & { align?: Align | undefined }) {
  return (
    <th
      data-slot="table-head"
      data-align={align}
      className={cn(
        'h-header-row px-3 align-middle text-caption font-medium whitespace-nowrap text-muted-foreground data-[align=center]:text-center data-[align=left]:text-left data-[align=right]:text-right',
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, align = 'left', ...props }: Omit<React.ComponentProps<'td'>, 'align'> & { align?: Align | undefined }) {
  return (
    <td
      data-slot="table-cell"
      data-align={align}
      className={cn(
        'h-row px-3 align-middle whitespace-nowrap text-foreground data-[align=center]:text-center data-[align=left]:text-left data-[align=right]:text-right',
        className
      )}
      {...props}
    />
  )
}

function TableCaption({ className, ...props }: React.ComponentProps<'caption'>) {
  return <caption data-slot="table-caption" className={cn('mt-3 text-caption text-muted-foreground', className)} {...props} />
}

export { Table, TableBody, TableCaption, TableCell, TableFooter, TableHead, TableHeader, TableRow }
