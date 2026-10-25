import { CircleCheckIcon, CircleXIcon, InfoIcon, LoaderCircleIcon, TriangleAlertIcon } from 'lucide-react'
import { Toaster as Sonner, type ToasterProps } from 'sonner'

// 12 Feedback, Toast: confirms a finished write and disappears after 5 s; failures stay until
// dismissed (pass duration: Infinity). Level 1: raised surface, hairline, 8 px radius, float
// shadow; the icon carries the status colour and the message is usually a mono ID.
function Toaster(props: ToasterProps) {
  return (
    <Sonner
      className="toaster group"
      position="bottom-right"
      duration={5000}
      closeButton
      icons={{
        error: <CircleXIcon className="size-4 text-danger-fg" />,
        info: <InfoIcon className="size-4 text-primary-text" />,
        loading: <LoaderCircleIcon className="size-4 animate-spin text-primary" />,
        success: <CircleCheckIcon className="size-4 text-success-fg" />,
        warning: <TriangleAlertIcon className="size-4 text-warning-fg" />
      }}
      toastOptions={{
        classNames: {
          closeButton:
            'order-last ml-auto !static !transform-none size-6 shrink-0 rounded-sm !border-0 !bg-transparent text-muted-foreground hover:!bg-surface-hover [&>svg]:size-3.5',
          content: 'flex min-w-0 flex-1 flex-col gap-0.5',
          description: 'font-mono text-caption text-muted-foreground',
          icon: 'mt-0.5 size-4 shrink-0 self-start',
          title: 'text-body font-semibold text-foreground',
          toast: 'flex w-[380px] items-start gap-3 rounded-lg border border-border bg-popover py-3 pr-3 pl-3.5 text-popover-foreground shadow-float'
        },
        unstyled: true
      }}
      {...props}
    />
  )
}

export { Toaster }
