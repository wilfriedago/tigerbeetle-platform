import { activeOrganization } from './organization'

/**
 * The ledger last opened in this browser, per organization, so "/" can land there
 * (docs/console.md, Sitemap). Ledger names are unique only within an organization (ADR-0027).
 */
function key(): string {
  return `tbp.last-ledger.${activeOrganization() ?? ''}`
}

export function getLastLedger(): string | undefined {
  try {
    return globalThis.localStorage?.getItem(key()) ?? undefined
  } catch {
    return undefined
  }
}

export function setLastLedger(ledger: string): void {
  try {
    globalThis.localStorage?.setItem(key(), ledger)
  } catch {
    // Private mode: the console still works, "/" just opens the ledger list.
  }
}

export function forgetLastLedger(): void {
  try {
    globalThis.localStorage?.removeItem(key())
  } catch {
    // Nothing to forget.
  }
}
