/**
 * The effect of a chain of postings on each account, per asset: what it debits, what it credits,
 * and the net change of the balance (credits minus debits, ADR-0004). Used by the transaction page
 * and by the preview of a new transaction. bigint only.
 */
export interface PostingLike {
  source: string
  destination: string
  asset: string
  amount: string
}

export interface NetEffect {
  address: string
  asset: string
  debited: string
  credited: string
  /** credited - debited. */
  net: string
}

export function netEffect(postings: readonly PostingLike[]): NetEffect[] {
  const rows = new Map<string, { address: string; asset: string; debited: bigint; credited: bigint }>()
  const row = (address: string, asset: string) => {
    const key = `${asset}\u0000${address}`
    let found = rows.get(key)
    if (!found) {
      found = { address, asset, credited: 0n, debited: 0n }
      rows.set(key, found)
    }
    return found
  }
  for (const posting of postings) {
    if (!/^\d+$/.test(posting.amount)) {
      continue
    }
    const amount = BigInt(posting.amount)
    row(posting.source, posting.asset).debited += amount
    row(posting.destination, posting.asset).credited += amount
  }
  // First appearance order, which follows the chain: the reader sees the money move.
  return [...rows.values()].map((entry) => ({
    address: entry.address,
    asset: entry.asset,
    credited: entry.credited.toString(),
    debited: entry.debited.toString(),
    net: (entry.credited - entry.debited).toString()
  }))
}
