import { useId } from 'react'

import { createFormHook, createFormHookContexts } from '@tanstack/react-form'

import { Button } from '~/components/ui/button'
import { Field, FieldDescription, FieldError, FieldHeader, FieldHint, FieldLabel } from '~/components/ui/field'
import { Input } from '~/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import { Textarea } from '~/components/ui/textarea'

import { cn } from '~/lib/utils'

/**
 * TanStack Form wired to the design system. Forms are built with `useAppForm`; fields render
 * through the components below, which own the label, help text, error display and accessibility
 * (11 Form controls: every field has a visible label, help states the rule the API enforces).
 */
export const { fieldContext, formContext, useFieldContext, useFormContext } = createFormHookContexts()

/** Errors are shown once a field was touched or the form was submitted. */
export function useFieldErrors(): string[] {
  const field = useFieldContext<unknown>()
  const submitted = field.form.state.submissionAttempts > 0
  if (!field.state.meta.isTouched && !submitted) {
    return []
  }
  return field.state.meta.errors
    .map((error) => (typeof error === 'string' ? error : ((error as { message?: string } | undefined)?.message ?? '')))
    .filter(Boolean)
}

interface FieldFrameProps {
  label: React.ReactNode
  hint?: React.ReactNode
  description?: React.ReactNode
  className?: string | undefined
  children: (props: { id: string; invalid: boolean; describedBy: string | undefined }) => React.ReactNode
}

/** Label row, control, help or error: the anatomy shared by every field. */
export function FieldFrame({ label, hint, description, className, children }: FieldFrameProps) {
  const id = useId()
  const errors = useFieldErrors()
  const invalid = errors.length > 0
  const describedBy = invalid || description ? `${id}-help` : undefined
  return (
    <Field data-invalid={invalid || undefined} className={className}>
      <FieldHeader>
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {hint ? <FieldHint>{hint}</FieldHint> : null}
      </FieldHeader>
      {children({ describedBy, id, invalid })}
      {invalid ? (
        <FieldError id={`${id}-help`} errors={errors} />
      ) : description ? (
        <FieldDescription id={`${id}-help`}>{description}</FieldDescription>
      ) : null}
    </Field>
  )
}

type InputProps = Omit<React.ComponentProps<typeof Input>, 'value' | 'onChange' | 'onBlur' | 'name' | 'id'>

interface TextFieldProps extends InputProps {
  label: React.ReactNode
  hint?: React.ReactNode
  description?: React.ReactNode
  mono?: boolean
}

function TextField({ label, hint, description, mono = false, className, ...input }: TextFieldProps) {
  const field = useFieldContext<string>()
  return (
    <FieldFrame label={label} hint={hint} description={description} className={className}>
      {({ id, invalid, describedBy }) => (
        <Input
          id={id}
          name={field.name}
          value={field.state.value}
          onBlur={field.handleBlur}
          onChange={(event) => field.handleChange(event.target.value)}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          className={cn(mono && 'font-mono')}
          {...input}
        />
      )}
    </FieldFrame>
  )
}

interface TextareaFieldProps extends Omit<React.ComponentProps<typeof Textarea>, 'value' | 'onChange' | 'onBlur' | 'name' | 'id'> {
  label: React.ReactNode
  hint?: React.ReactNode
  description?: React.ReactNode
}

function TextareaField({ label, hint, description, className, ...textarea }: TextareaFieldProps) {
  const field = useFieldContext<string>()
  return (
    <FieldFrame label={label} hint={hint} description={description} className={className}>
      {({ id, invalid, describedBy }) => (
        <Textarea
          id={id}
          name={field.name}
          value={field.state.value}
          onBlur={field.handleBlur}
          onChange={(event) => field.handleChange(event.target.value)}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          {...textarea}
        />
      )}
    </FieldFrame>
  )
}

export interface Option {
  value: string
  label: React.ReactNode
}

interface SelectFieldProps {
  label: React.ReactNode
  hint?: React.ReactNode
  description?: React.ReactNode
  options: readonly Option[]
  placeholder?: string
  className?: string
  mono?: boolean
}

function SelectField({ label, hint, description, options, placeholder, className, mono = false }: SelectFieldProps) {
  const field = useFieldContext<string>()
  return (
    <FieldFrame label={label} hint={hint} description={description} className={className}>
      {({ id, invalid, describedBy }) => (
        <Select
          items={options}
          value={field.state.value}
          onValueChange={(value) => {
            field.handleChange(value ?? '')
            field.handleBlur()
          }}
        >
          <SelectTrigger id={id} aria-invalid={invalid || undefined} aria-describedby={describedBy} className={cn(mono && 'font-mono')}>
            <SelectValue placeholder={placeholder} />
          </SelectTrigger>
          <SelectContent>
            {options.map((option) => (
              <SelectItem key={option.value} value={option.value} className={cn(mono && 'font-mono')}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </FieldFrame>
  )
}

/** The submit button follows the form: disabled while invalid, a spinner while submitting. */
function SubmitButton({ children, className, size }: { children: React.ReactNode; className?: string; size?: 'default' | 'lg' }) {
  const form = useFormContext()
  return (
    <form.Subscribe selector={(state) => ({ canSubmit: state.canSubmit, isSubmitting: state.isSubmitting })}>
      {({ canSubmit, isSubmitting }) => (
        <Button type="submit" disabled={!canSubmit} loading={isSubmitting} className={className} size={size ?? 'default'}>
          {children}
        </Button>
      )}
    </form.Subscribe>
  )
}

export const { useAppForm, withForm } = createFormHook({
  fieldComponents: { SelectField, TextareaField, TextField },
  fieldContext,
  formComponents: { SubmitButton },
  formContext
})
