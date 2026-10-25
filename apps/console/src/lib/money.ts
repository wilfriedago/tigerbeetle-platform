/**
 * Amounts and assets (ADR-0004, ADR-0005).
 *
 * JSON amounts are strings holding an integer in the asset's smallest unit; balances may be
 * negative (world). The scale is a label: `EUR/2` with "125000" means 1,250.00. Everything here
 * uses bigint and string arithmetic, never JavaScript numbers, which lose precision above 2^53.
 */

export interface Asset {
  code: string
  scale: number
}

/** The largest scale the API accepts (MaxAssetScale in apps/api/internal/kernel/ledger/asset.go). */
export const MAX_ASSET_SCALE = 18

/** 2^128 - 1, the largest amount TigerBeetle stores. */
export const MAX_AMOUNT = (1n << 128n) - 1n

const ASSET_PATTERN = /^(?<code>[A-Z][A-Z0-9]{1,15})(?:\/(?<scale>\d{1,2}))?$/
const RAW_PATTERN = /^-?(?:0|[1-9]\d*)$/
const MAJOR_PATTERN = /^(?<int>\d{1,3}(?:,\d{3})*|\d+)?(?:\.(?<frac>\d*))?$/

/** Parses `CODE/SCALE`. A missing scale means zero decimals, exactly like the API. */
export function parseAsset(value: string): Asset | undefined {
  const match = ASSET_PATTERN.exec(value.trim())
  if (!match?.groups?.code) {
    return undefined
  }
  const scale = match.groups.scale === undefined ? 0 : Number(match.groups.scale)
  if (scale > MAX_ASSET_SCALE) {
    return undefined
  }
  return { code: match.groups.code, scale }
}

/** The canonical form, always with the scale: `EUR/2`, `XOF/0`. */
export function formatAsset(asset: Asset): string {
  return `${asset.code}/${asset.scale}`
}

/** Reads the scale of a canonical asset string; unknown shapes count as scale 0. */
export function assetScale(asset: string): number {
  return parseAsset(asset)?.scale ?? 0
}

export function assetCode(asset: string): string {
  return parseAsset(asset)?.code ?? asset
}

/** Whether a string is a valid amount integer (optionally negative, no leading zeros). */
export function isRawAmount(value: string): boolean {
  return RAW_PATTERN.test(value)
}

function groupThousands(digits: string): string {
  let out = ''
  for (let i = 0; i < digits.length; i += 1) {
    if (i > 0 && (digits.length - i) % 3 === 0) {
      out += ','
    }
    out += digits[i]
  }
  return out
}

/**
 * Formats an amount integer with the asset scale: ("125000", 2) is "1,250.00",
 * ("-481230000", 2) is "-4,812,300.00", ("250000", 0) is "250,000". Invalid input is returned
 * unchanged so a bad value is visible rather than silently rewritten.
 */
export function formatAmount(raw: string, scale: number, options: { grouping?: boolean } = {}): string {
  if (!isRawAmount(raw)) {
    return raw
  }
  const grouping = options.grouping ?? true
  const negative = raw.startsWith('-')
  const digits = negative ? raw.slice(1) : raw
  const padded = digits.padStart(scale + 1, '0')
  const intPart = padded.slice(0, padded.length - scale)
  const fracPart = scale > 0 ? padded.slice(padded.length - scale) : ''
  const sign = negative && digits !== '0' ? '-' : ''
  const intOut = grouping ? groupThousands(intPart) : intPart
  return fracPart ? `${sign}${intOut}.${fracPart}` : `${sign}${intOut}`
}

export type ParseResult = { ok: true; raw: string } | { ok: false; error: string }

/**
 * Converts what a person types in major units into the integer the API expects:
 * ("1,250.00", 2) is "125000", ("12.5", 2) is "1250". Rejects signs, more decimals than the
 * scale allows, and values above 2^128 - 1. Zero is allowed here; callers decide whether zero
 * makes sense (holds and captures reject it, ADR-0010).
 */
export function parseMajorUnits(input: string, scale: number): ParseResult {
  const value = input.trim()
  if (value === '') {
    return { error: 'Enter an amount.', ok: false }
  }
  if (value.startsWith('-')) {
    return { error: 'Amounts are positive; the direction comes from source and destination.', ok: false }
  }
  const match = MAJOR_PATTERN.exec(value)
  const intRaw = match?.groups?.int ?? ''
  const fracRaw = match?.groups?.frac ?? ''
  if (!match || (intRaw === '' && fracRaw === '')) {
    return { error: 'Use digits, an optional decimal point and optional thousands commas.', ok: false }
  }
  if (fracRaw.length > scale) {
    return {
      error: scale === 0 ? 'This asset has no decimal places.' : `This asset has ${scale} decimal place${scale === 1 ? '' : 's'} at most.`,
      ok: false
    }
  }
  const digits = `${intRaw.replaceAll(',', '')}${fracRaw.padEnd(scale, '0')}`
  const amount = BigInt(digits === '' ? '0' : digits)
  if (amount > MAX_AMOUNT) {
    return { error: 'The amount is larger than TigerBeetle can store (2^128 - 1).', ok: false }
  }
  return { ok: true, raw: amount.toString() }
}

/** Sums amount integers with bigint; non-integer inputs count as zero. */
export function sumAmounts(values: readonly string[]): string {
  let total = 0n
  for (const value of values) {
    if (isRawAmount(value)) {
      total += BigInt(value)
    }
  }
  return total.toString()
}

/** a - b with bigint. */
export function subtractAmounts(a: string, b: string): string {
  return (BigInt(isRawAmount(a) ? a : '0') - BigInt(isRawAmount(b) ? b : '0')).toString()
}

/** -1, 0 or 1. */
export function compareAmounts(a: string, b: string): number {
  const x = BigInt(isRawAmount(a) ? a : '0')
  const y = BigInt(isRawAmount(b) ? b : '0')
  if (x === y) {
    return 0
  }
  return x < y ? -1 : 1
}

export function isNegative(raw: string): boolean {
  return isRawAmount(raw) && raw.startsWith('-') && raw !== '-0'
}

export function isZero(raw: string): boolean {
  return isRawAmount(raw) && BigInt(raw) === 0n
}

/** The share of `part` in `whole` as a percentage between 0 and 100, for bars only. */
export function percentOf(part: string, whole: string): number {
  const p = BigInt(isRawAmount(part) ? part : '0')
  const w = BigInt(isRawAmount(whole) ? whole : '0')
  if (w <= 0n || p <= 0n) {
    return 0
  }
  if (p >= w) {
    return 100
  }
  // Basis points keep two decimals of precision without converting the amounts to numbers.
  return Number((p * 10_000n) / w) / 100
}

/** Signed difference formatted with an explicit plus sign, for "balance effect" previews. */
export function formatSignedAmount(raw: string, scale: number): string {
  const formatted = formatAmount(raw, scale)
  if (!isRawAmount(raw) || isZero(raw) || raw.startsWith('-')) {
    return formatted
  }
  return `+${formatted}`
}
