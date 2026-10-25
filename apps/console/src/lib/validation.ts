import { parseAsset, parseMajorUnits } from './money'

// The input rules of the API, checked in the console so a form explains them before a request is
// sent (apps/api/internal/kernel/ledger). The API checks them again.

/** The Address schema of the API: colon-separated segments of letters, digits, dash and underscore. */
export const ADDRESS_PATTERN = /^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*$/

export function addressError(value: string, label: string): string | undefined {
  if (value === '') {
    return `Pick the ${label} account.`
  }
  if (value.length > 255 || !ADDRESS_PATTERN.test(value)) {
    return 'Use segments of letters, digits, dash or underscore, separated by colons.'
  }
  return undefined
}

export function assetError(value: string): string | undefined {
  if (value === '') {
    return 'Pick an asset.'
  }
  return parseAsset(value) ? undefined : 'Use CODE/SCALE, such as EUR/2.'
}

export function amountError(value: string, asset: string): string | undefined {
  const parsed = parseMajorUnits(value, parseAsset(asset)?.scale ?? 0)
  if (!parsed.ok) {
    return parsed.error
  }
  return parsed.raw === '0' ? 'TigerBeetle moves amounts greater than zero.' : undefined
}
