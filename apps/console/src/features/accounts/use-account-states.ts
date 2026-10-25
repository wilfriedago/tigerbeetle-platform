import { useQueries } from '@tanstack/react-query'

import { type Account, type Rule } from '@tigerbeetle-platform/api-client'

import { settingsQueries } from '~/features/settings/api'

import { type AccountState } from '~/lib/preflight'
import { isNotFound } from '~/lib/problem'

import { accountQueries } from './api'

const ADDRESS = /^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*$/

export interface AccountStates {
  /** Balances and limit of (address, asset) as the write would find them; undefined while unknown. */
  stateOf: (address: string, asset: string) => AccountState | undefined
  /** The account, when the address exists. */
  accountOf: (address: string) => Account | undefined
  /** The rule that creates the address's accounts in assets it does not have yet. */
  ruleOf: (address: string) => Rule | undefined
  loading: boolean
}

/**
 * Reads what a write is about to touch: each address once (GET account) and the rule that would
 * create its missing accounts (GET rules/match). Both are cached, so editing the form does not
 * refetch what is already known.
 */
export function useAccountStates(ledger: string, addresses: readonly string[]): AccountStates {
  const unique = [...new Set(addresses.filter((address) => ADDRESS.test(address)))]
  const accounts = useQueries({ queries: unique.map((address) => accountQueries.detail(ledger, address)) })
  const matches = useQueries({ queries: unique.map((address) => settingsQueries.match(ledger, address)) })
  const index = new Map(unique.map((address, i) => [address, i]))

  const accountOf = (address: string) => {
    const i = index.get(address)
    return i === undefined ? undefined : accounts[i]?.data
  }
  const ruleOf = (address: string) => {
    const i = index.get(address)
    return i === undefined ? undefined : matches[i]?.data?.rule
  }
  const stateOf = (address: string, asset: string): AccountState | undefined => {
    const i = index.get(address)
    const account = i === undefined ? undefined : accounts[i]
    if (!account) {
      return undefined
    }
    if (account.data) {
      const balance = account.data.balances.find((entry) => entry.asset === asset)
      if (balance) {
        return { balance: balance.balance, exists: true, incoming: balance.incoming, limit: balance.limit, reserved: balance.reserved }
      }
    } else if (!isNotFound(account.error)) {
      return undefined
    }
    // No account in this asset yet: the write creates it with the matching rule's flags (ADR-0006).
    const rule = ruleOf(address)
    return rule ? { balance: '0', exists: false, incoming: '0', limit: rule.limit, reserved: '0' } : undefined
  }
  return {
    accountOf,
    loading:
      accounts.some((query) => query.isPending && query.fetchStatus !== 'idle') ||
      matches.some((query) => query.isPending && query.fetchStatus !== 'idle'),
    ruleOf,
    stateOf
  }
}
