/** The ledger last opened in this browser, so "/" can land there (docs/console.md, Sitemap). */
const KEY = 'tbp.last-ledger'

export function getLastLedger(): string | undefined {
  try {
    return globalThis.localStorage?.getItem(KEY) ?? undefined
  } catch {
    return undefined
  }
}

export function setLastLedger(ledger: string): void {
  try {
    globalThis.localStorage?.setItem(KEY, ledger)
  } catch {
    // Private mode: the console still works, "/" just opens the ledger list.
  }
}

export function forgetLastLedger(): void {
  try {
    globalThis.localStorage?.removeItem(KEY)
  } catch {
    // Nothing to forget.
  }
}
