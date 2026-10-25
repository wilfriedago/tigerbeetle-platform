import { FieldFrame, useFieldContext } from '~/components/form/form'

import { AccountCombobox } from './account-combobox'

interface AccountFieldProps {
  ledger: string
  label: React.ReactNode
  asset?: string
  /** The note under the field: what the account can spend, or the rule that will create it. */
  description?: React.ReactNode
  className?: string
}

/** An account picker bound to a TanStack Form field (use inside `form.AppField`). */
export function AccountField({ ledger, label, asset, description, className }: AccountFieldProps) {
  const field = useFieldContext<string>()
  return (
    <FieldFrame label={label} description={description} className={className}>
      {({ id, invalid, describedBy }) => (
        <AccountCombobox
          ledger={ledger}
          id={id}
          invalid={invalid}
          describedBy={describedBy}
          value={field.state.value}
          onChange={(address) => field.handleChange(address)}
          onBlur={field.handleBlur}
          {...(asset ? { asset } : {})}
        />
      )}
    </FieldFrame>
  )
}
