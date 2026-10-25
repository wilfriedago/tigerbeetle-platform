import { FieldFrame, useFieldContext } from '~/components/form/form'
import { InputGroup, InputGroupInput } from '~/components/ui/input-group'

import { parseAsset, parseMajorUnits } from '~/lib/money'

import { AssetCombobox } from './asset-combobox'

// 20 Money and identity, Amount Field: typed in major units with the asset beside it, and shown as
// the integer that will be sent ("12.50 EUR/2" is sent as "1250"). bigint arithmetic only (ADR-0005).

interface AmountFieldProps {
  label?: React.ReactNode
  asset: string
  onAssetChange: (asset: string) => void
  assets: readonly string[]
  assetInvalid?: boolean
  /** Extra text after "Sent as ...". */
  note?: React.ReactNode
  className?: string
}

/** Binds the amount (major units, as typed) of a TanStack Form field; the asset is passed in. */
export function AmountField({ label = 'Amount', asset, onAssetChange, assets, assetInvalid = false, note, className }: AmountFieldProps) {
  const field = useFieldContext<string>()
  const scale = parseAsset(asset)?.scale
  const parsed = scale === undefined ? undefined : parseMajorUnits(field.state.value, scale)
  const description =
    parsed?.ok && field.state.value.trim() !== '' ? (
      <>
        Sent as <span className="font-mono text-foreground">"{parsed.raw}"</span> in the smallest unit.{note ? <> {note}</> : null}
      </>
    ) : (
      (note ?? (scale === undefined ? 'Pick the asset first.' : `Up to ${scale} decimal place${scale === 1 ? '' : 's'}.`))
    )
  return (
    <FieldFrame label={label} description={description} className={className}>
      {({ id, invalid, describedBy }) => (
        <div className="flex min-w-0">
          <InputGroup className="rounded-r-none">
            <InputGroupInput
              id={id}
              inputMode="decimal"
              autoComplete="off"
              placeholder={scale ? `0.${'0'.repeat(scale)}` : '0'}
              value={field.state.value}
              onChange={(event) => field.handleChange(event.target.value)}
              onBlur={field.handleBlur}
              aria-invalid={invalid || undefined}
              aria-describedby={describedBy}
              className="text-right font-mono tabular-nums"
            />
          </InputGroup>
          <AssetCombobox
            value={asset}
            onChange={onAssetChange}
            assets={assets}
            invalid={assetInvalid}
            className="-ml-px w-28 shrink-0 rounded-l-none"
          />
        </div>
      )}
    </FieldFrame>
  )
}
