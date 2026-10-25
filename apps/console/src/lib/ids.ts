/**
 * Transaction, hold and account IDs are 128-bit time-ordered numbers rendered as 32 lowercase hex
 * characters (ADR-0004). Lists shorten them in the middle; the full value is always one copy away.
 */
export const ID_PATTERN = /^[0-9a-f]{32}$/

export function isId(value: string): boolean {
  return ID_PATTERN.test(value)
}

/** "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e" becomes "0192f3a1...7c2e". */
export function shortId(id: string, head = 8, tail = 4): string {
  if (id.length <= head + tail + 3) {
    return id
  }
  return `${id.slice(0, head)}...${id.slice(-tail)}`
}
