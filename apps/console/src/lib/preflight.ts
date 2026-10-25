import { type PostingLike } from './net-effect'

/**
 * A console-side dry run of a linked chain (docs/console.md, New transaction): it applies the
 * postings in order to the balances read beforehand and reports what TigerBeetle would refuse.
 * Advisory only: balances can move between the read and the commit, and TigerBeetle decides.
 *
 * TigerBeetle applies the transfers of a chain one after the other, so a later posting can spend
 * what an earlier one credited. The limits are the account flags of ADR-0006:
 * - debits_must_not_exceed_credits: debits_pending + debits_posted <= credits_posted, that is
 *   balance - reserved >= 0 after the debit;
 * - credits_must_not_exceed_debits: credits_pending + credits_posted <= debits_posted, that is
 *   -balance - incoming >= 0 after the credit.
 */
export type BalanceLimit = 'none' | 'debits_must_not_exceed_credits' | 'credits_must_not_exceed_debits'

export interface AccountState {
  /** Whether the TigerBeetle account exists; a missing one is created by the write with the rule's flags. */
  exists: boolean
  balance: string
  reserved: string
  incoming: string
  limit: BalanceLimit
}

export interface Effect {
  address: string
  asset: string
  before: string
  after: string
  delta: string
  exists: boolean
}

export interface Failure {
  /** Zero-based posting index. */
  index: number
  address: string
  asset: string
  reason: 'exceeds-credits' | 'exceeds-debits'
  /** How much is missing, in the smallest unit. */
  shortBy: string
}

export interface Simulation {
  effects: Effect[]
  failures: Failure[]
  /** Per posting: what the source can spend at that point of the chain, or undefined when it has no limit or is unknown. */
  spendable: (string | undefined)[]
}

interface Running {
  address: string
  asset: string
  exists: boolean
  limit: BalanceLimit
  before: bigint
  balance: bigint
  reserved: bigint
  incoming: bigint
}

const RAW = /^\d+$/

export function simulate(postings: readonly PostingLike[], stateOf: (address: string, asset: string) => AccountState | undefined): Simulation {
  const running = new Map<string, Running | null>()
  const get = (address: string, asset: string): Running | null => {
    const key = `${asset}\u0000${address}`
    if (!running.has(key)) {
      const state = stateOf(address, asset)
      running.set(
        key,
        state
          ? {
              address,
              asset,
              balance: BigInt(state.balance),
              before: BigInt(state.balance),
              exists: state.exists,
              incoming: BigInt(state.incoming),
              limit: state.limit,
              reserved: BigInt(state.reserved)
            }
          : null
      )
    }
    return running.get(key) ?? null
  }

  const failures: Failure[] = []
  const spendable: (string | undefined)[] = []
  for (const [index, posting] of postings.entries()) {
    if (!RAW.test(posting.amount) || posting.source === '' || posting.destination === '') {
      spendable.push(undefined)
      continue
    }
    const amount = BigInt(posting.amount)
    const source = get(posting.source, posting.asset)
    const destination = get(posting.destination, posting.asset)
    if (source) {
      const limited = source.limit === 'debits_must_not_exceed_credits'
      spendable.push(limited ? (source.balance - source.reserved).toString() : undefined)
      source.balance -= amount
      const headroom = source.balance - source.reserved
      if (limited && headroom < 0n) {
        failures.push({ address: source.address, asset: source.asset, index, reason: 'exceeds-credits', shortBy: (-headroom).toString() })
      }
    } else {
      spendable.push(undefined)
    }
    if (destination) {
      destination.balance += amount
      const headroom = -destination.balance - destination.incoming
      if (destination.limit === 'credits_must_not_exceed_debits' && headroom < 0n) {
        failures.push({ address: destination.address, asset: destination.asset, index, reason: 'exceeds-debits', shortBy: (-headroom).toString() })
      }
    }
  }

  const effects: Effect[] = []
  for (const entry of running.values()) {
    if (entry) {
      effects.push({
        address: entry.address,
        after: entry.balance.toString(),
        asset: entry.asset,
        before: entry.before.toString(),
        delta: (entry.balance - entry.before).toString(),
        exists: entry.exists
      })
    }
  }
  return { effects, failures, spendable }
}
