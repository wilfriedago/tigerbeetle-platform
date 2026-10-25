import { useQueries } from '@tanstack/react-query'

import { type Account, type Rule } from '@tigerbeetle-platform/api-client'

import { type AccountState } from '~/lib/preflight'

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
 * Reads what a write is about to touch: one GraphQL request per address returns its account, when
 * it exists, and the rule that would create its missing accounts. Each answer is cached, so editing
 * the form does not refetch what is already known.
 */
export function useAccountStates(ledger: string, addresses: readonly string[]): AccountStates {
  const unique = [...new Set(addresses.filter((address) => ADDRESS.test(address)))]
  const targets = useQueries({ queries: unique.map((address) => accountQueries.writeTarget(ledger, address)) })
  const index = new Map(unique.map((address, i) => [address, i]))

  const targetOf = (address: string) => {
    const i = index.get(address)
    return i === undefined ? undefined : targets[i]?.data
  }
  const accountOf = (address: string) => targetOf(address)?.account
  const ruleOf = (address: string) => targetOf(address)?.rule
  const stateOf = (address: string, asset: string): AccountState | undefined => {
    const target = targetOf(address)
    if (!target) {
      return undefined
    }
    const balance = target.account?.balances.find((entry) => entry.asset === asset)
    if (balance) {
      return { balance: balance.balance, exists: true, incoming: balance.incoming, limit: balance.limit, reserved: balance.reserved }
    }
    // No account in this asset yet: the write creates it with the matching rule's flags (ADR-0006).
    return { balance: '0', exists: false, incoming: '0', limit: target.rule.limit, reserved: '0' }
  }
  return {
    accountOf,
    loading: targets.some((query) => query.isPending && query.fetchStatus !== 'idle'),
    ruleOf,
    stateOf
  }
}
