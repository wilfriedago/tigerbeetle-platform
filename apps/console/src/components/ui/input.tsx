import * as React from 'react'

import { Input as InputPrimitive } from '@base-ui/react/input'

import { cn } from '~/lib/utils'

// 11 Form controls: 32 px with a 1 px border-strong outline. Hover darkens the outline, focus uses
// the accent outline plus the ring, error keeps the value and switches the outline to danger,
// read-only shows a value the user cannot change on the sunken surface. Machine values
// (addresses, keys, amounts) add font-mono at the call site.
const inputClassName =
  'h-8 w-full min-w-0 rounded-md border border-input bg-surface px-2.5 text-body text-foreground transition-[color,border-color,box-shadow] outline-none placeholder:text-subtle-foreground hover:border-subtle-foreground focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-focus-ring disabled:pointer-events-none disabled:border-border disabled:bg-surface-sunken disabled:text-subtle-foreground read-only:border-border read-only:bg-surface-sunken read-only:text-muted-foreground read-only:hover:border-border read-only:focus-visible:border-border aria-invalid:border-danger-fg aria-invalid:focus-visible:ring-danger-border file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-small file:font-medium'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return <InputPrimitive type={type} data-slot="input" className={cn(inputClassName, className)} {...props} />
}

export { Input, inputClassName }
