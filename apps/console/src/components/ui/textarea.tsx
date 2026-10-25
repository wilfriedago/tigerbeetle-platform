

import { cn } from '~/lib/utils'

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'flex field-sizing-content min-h-22 w-full rounded-md border border-input bg-surface px-2.5 py-2 text-body text-foreground transition-[color,border-color,box-shadow] outline-none placeholder:text-subtle-foreground hover:border-subtle-foreground focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-focus-ring disabled:cursor-not-allowed disabled:border-border disabled:bg-surface-sunken disabled:text-subtle-foreground aria-invalid:border-danger-fg aria-invalid:focus-visible:ring-danger-border',
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
