import { cn } from '~/lib/utils'

function Label({ className, ...props }: React.ComponentProps<'label'>) {
  return (
    // The control is associated at the call site, through htmlFor or by nesting.
    // oxlint-disable-next-line jsx-a11y/label-has-associated-control
    <label
      data-slot="label"
      className={cn(
        'flex items-center gap-2 text-small leading-snug font-medium text-foreground select-none group-data-[disabled=true]:pointer-events-none group-data-[disabled=true]:opacity-45 peer-disabled:cursor-not-allowed peer-disabled:opacity-45',
        className
      )}
      {...props}
    />
  )
}

export { Label }
