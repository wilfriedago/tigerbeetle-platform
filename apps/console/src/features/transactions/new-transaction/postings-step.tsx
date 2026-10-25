import { PlusIcon, XIcon } from 'lucide-react'

import { withForm } from '~/components/form/form'
import { useFormattedAmount } from '~/components/ledger/amount'
import { AmountField } from '~/components/ledger/amount-field'
import { Hint } from '~/components/ledger/hint'
import { Button } from '~/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'

import { AccountField } from '~/features/accounts/components/account-field'
import { type AccountStates } from '~/features/accounts/use-account-states'

import { type Simulation } from '~/lib/preflight'

import { addressError, amountError, assetError, emptyPosting, MAX_POSTINGS } from './draft'
import { transactionFormOptions } from './form-options'

const LIMIT_TEXT = {
  credits_must_not_exceed_debits: 'credits cannot exceed debits',
  debits_must_not_exceed_credits: 'cannot go negative',
  none: 'no limit'
} as const

function SpendNote({ amount, asset, index }: { amount: string; asset: string; index: number }) {
  const text = useFormattedAmount(amount, asset)
  if (index === 0) {
    return <>{text} available.</>
  }
  return (
    <>
      {text} available after {index === 1 ? 'posting 1' : `postings 1 to ${index}`}.
    </>
  )
}

function BalanceNote({ amount, asset }: { amount: string; asset: string }) {
  return <>Balance {useFormattedAmount(amount, asset)}.</>
}

/** The note under a source: what it can spend at this point of the chain, or why it has no limit. */
function sourceNote(address: string, asset: string, index: number, states: AccountStates, simulation: Simulation): React.ReactNode {
  if (!address) {
    return 'Debited by this posting.'
  }
  const state = asset ? states.stateOf(address, asset) : undefined
  if (!state) {
    return undefined
  }
  const spendable = simulation.spendable[index]
  if (state.limit === 'none') {
    return address === 'world' ? 'world has no limit.' : 'No balance limit on this account.'
  }
  if (!state.exists) {
    const rule = states.ruleOf(address)
    return `New account, rule ${rule?.pattern ?? '*'} (${LIMIT_TEXT[state.limit]}).`
  }
  return spendable === undefined ? undefined : <SpendNote amount={spendable} asset={asset} index={index} />
}

function destinationNote(address: string, asset: string, states: AccountStates): React.ReactNode {
  if (!address) {
    return 'Credited by this posting.'
  }
  const state = asset ? states.stateOf(address, asset) : undefined
  if (!state) {
    return undefined
  }
  if (!state.exists) {
    const rule = states.ruleOf(address)
    return `New account, rule ${rule?.pattern ?? '*'} (${LIMIT_TEXT[state.limit]}).`
  }
  return <BalanceNote amount={state.balance} asset={asset} />
}

const NO_SIMULATION: Simulation = { effects: [], failures: [], spendable: [] }

export const PostingsStep = withForm({
  ...transactionFormOptions,
  props: {
    assets: [] as readonly string[],
    ledger: '',
    simulation: NO_SIMULATION,
    states: undefined as unknown as AccountStates
  },
  render({ form, ledger, assets, states, simulation }) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-card-title">Postings</CardTitle>
          <CardDescription className="text-small">
            Source is debited, destination is credited. Pick accounts from the list or type a new address.
          </CardDescription>
          <CardAction>
            <form.Subscribe selector={(state) => state.values.postings.length}>
              {(count) => (
                <span className="font-mono text-caption text-muted-foreground">
                  {count} of {MAX_POSTINGS}
                </span>
              )}
            </form.Subscribe>
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 p-5">
          <form.Field name="postings" mode="array">
            {(postings) => (
              <>
                {postings.state.value.map((posting, index) => (
                  // Postings are positional: the index is the leg of the chain.
                  // oxlint-disable-next-line react/no-array-index-key
                  <fieldset key={index} className="flex flex-col gap-3 rounded-md border border-border bg-surface-sunken/50 p-4">
                    <div className="flex items-center justify-between">
                      <legend className="text-small font-medium">Posting {index + 1}</legend>
                      {postings.state.value.length > 1 ? (
                        <Button variant="ghost" size="icon-xs" aria-label={`Remove posting ${index + 1}`} onClick={() => postings.removeValue(index)}>
                          <XIcon />
                        </Button>
                      ) : null}
                    </div>
                    <div className="grid gap-3 md:grid-cols-[1fr_1fr_minmax(0,1.1fr)]">
                      <form.AppField name={`postings[${index}].source`} validators={{ onChange: ({ value }) => addressError(value, 'source') }}>
                        {() => (
                          <AccountField
                            ledger={ledger}
                            label="Source"
                            asset={posting.asset}
                            description={sourceNote(posting.source, posting.asset, index, states, simulation)}
                          />
                        )}
                      </form.AppField>
                      <form.AppField
                        name={`postings[${index}].destination`}
                        validators={{
                          onChange: ({ value, fieldApi }) =>
                            addressError(value, 'destination') ??
                            (value === fieldApi.form.getFieldValue(`postings[${index}].source`)
                              ? 'Pick another account: TigerBeetle needs two different accounts.'
                              : undefined),
                          onChangeListenTo: [`postings[${index}].source`]
                        }}
                      >
                        {() => (
                          <AccountField
                            ledger={ledger}
                            label="Destination"
                            asset={posting.asset}
                            description={destinationNote(posting.destination, posting.asset, states)}
                          />
                        )}
                      </form.AppField>
                      <form.Field name={`postings[${index}].asset`} validators={{ onChange: ({ value }) => assetError(value) }}>
                        {(assetField) => (
                          <form.AppField
                            name={`postings[${index}].amount`}
                            validators={{
                              onChange: ({ value, fieldApi }) => amountError(value, fieldApi.form.getFieldValue(`postings[${index}].asset`)),
                              onChangeListenTo: [`postings[${index}].asset`]
                            }}
                          >
                            {() => (
                              <AmountField
                                asset={assetField.state.value}
                                onAssetChange={(asset) => assetField.handleChange(asset)}
                                assets={assets}
                                assetInvalid={assetField.state.meta.isTouched && assetField.state.meta.errors.length > 0}
                              />
                            )}
                          </form.AppField>
                        )}
                      </form.Field>
                    </div>
                  </fieldset>
                ))}
                <div>
                  <Button
                    variant="ghost"
                    disabled={postings.state.value.length >= MAX_POSTINGS}
                    onClick={() => {
                      const last = postings.state.value.at(-1)
                      // A new leg usually continues the chain: from the last destination, in the same asset.
                      postings.pushValue(emptyPosting(last?.asset ?? '', last?.destination ?? ''))
                    }}
                  >
                    <PlusIcon />
                    Add posting
                  </Button>
                </div>
              </>
            )}
          </form.Field>
          <Hint>Available balances already count the earlier postings of this chain: TigerBeetle applies them in order.</Hint>
        </CardContent>
      </Card>
    )
  }
})
