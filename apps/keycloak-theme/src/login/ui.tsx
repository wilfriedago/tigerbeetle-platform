import { useState } from 'react'

import { CircleAlertIcon, CircleCheckIcon, EyeIcon, EyeOffIcon, InfoIcon, TriangleAlertIcon } from 'lucide-react'

// The form parts of the sign-in pages, with the classes of the console's components (10 Buttons,
// 11 Form controls, 12 Feedback in .design/ds.lib.pen), so the pages look like the console.

export function cn(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(' ')
}

const buttonBase =
  'inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-md border border-transparent px-4 text-body font-medium whitespace-nowrap transition-colors outline-none select-none focus-visible:ring-3 focus-visible:ring-focus-ring disabled:pointer-events-none disabled:opacity-45'

export const primaryButton = cn(buttonBase, 'bg-primary text-primary-foreground hover:bg-primary-hover active:bg-primary-pressed')

export const secondaryButton = cn(buttonBase, 'border-border-strong bg-surface text-foreground hover:bg-surface-hover active:bg-surface-pressed')

export const linkClass = 'text-small text-primary-text outline-none hover:underline hover:underline-offset-4 focus-visible:underline'

export const inputClass =
  'h-10 w-full min-w-0 rounded-md border border-input bg-surface px-3 text-body text-foreground transition-[color,border-color,box-shadow] outline-none placeholder:text-subtle-foreground hover:border-subtle-foreground focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-focus-ring read-only:border-border read-only:bg-surface-sunken read-only:text-muted-foreground aria-invalid:border-danger-fg aria-invalid:focus-visible:ring-danger-border'

interface FieldProps {
  id: string
  label: React.ReactNode
  error?: string | undefined
  hint?: React.ReactNode
  aside?: React.ReactNode
  children: React.ReactNode
}

/** A label, its control, and the error or hint under it. */
export function Field({ id, label, error, hint, aside, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-2">
        <label htmlFor={id} className="text-small font-medium text-foreground">
          {label}
        </label>
        {aside}
      </div>
      {children}
      {error ? (
        <span id={`${id}-error`} className="text-caption text-danger-fg" aria-live="polite">
          {error}
        </span>
      ) : hint ? (
        <span className="text-caption text-muted-foreground">{hint}</span>
      ) : null}
    </div>
  )
}

/** A password input with a button that shows what was typed. */
export function PasswordInput({ id, invalid, ...props }: React.ComponentProps<'input'> & { id: string; invalid?: boolean }) {
  const [shown, setShown] = useState(false)
  return (
    <div className="relative">
      <input
        id={id}
        type={shown ? 'text' : 'password'}
        className={cn(inputClass, 'pr-10')}
        aria-invalid={invalid || undefined}
        aria-describedby={invalid ? `${id}-error` : undefined}
        {...props}
      />
      <button
        type="button"
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-md text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-focus-ring"
        aria-label={shown ? 'Hide password' : 'Show password'}
        aria-controls={id}
        onClick={() => setShown(!shown)}
      >
        {shown ? <EyeOffIcon className="size-4" /> : <EyeIcon className="size-4" />}
      </button>
    </div>
  )
}

const alertStyles = {
  error: { className: 'border-danger-border bg-danger-bg text-danger-fg', Icon: CircleAlertIcon },
  info: { className: 'border-primary-border bg-primary-subtle text-primary-text', Icon: InfoIcon },
  success: { className: 'border-success-border bg-success-bg text-success-fg', Icon: CircleCheckIcon },
  warning: { className: 'border-warning-border bg-warning-bg text-warning-fg', Icon: TriangleAlertIcon }
} as const

/** Keycloak's message for the page, in the colour of its kind. Keycloak sanitizes the HTML. */
export function Alert({ type, html }: { type: keyof typeof alertStyles; html: string }) {
  const { className, Icon } = alertStyles[type]
  return (
    <div role={type === 'error' ? 'alert' : 'status'} className={cn('flex gap-2 rounded-md border px-3 py-2.5 text-small', className)}>
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <span className="text-foreground" dangerouslySetInnerHTML={{ __html: html }} />
    </div>
  )
}
