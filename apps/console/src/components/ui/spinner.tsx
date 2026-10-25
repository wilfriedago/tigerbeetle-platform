import { LoaderCircleIcon } from 'lucide-react'

import { cn } from '~/lib/utils'

// Lucide loader-circle, the "Loading" icon of 04 Iconography, in the accent colour by default.
function Spinner({ className, ...props }: React.ComponentProps<'svg'>) {
  return (
    <LoaderCircleIcon
      data-slot="spinner"
      role="status"
      aria-label="Loading"
      className={cn('size-4 animate-spin text-primary', className)}
      {...props}
    />
  )
}

export { Spinner }
