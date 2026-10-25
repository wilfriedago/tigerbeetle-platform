/** The prefixes summed last in this browser, newest first (a convenience; nothing depends on it). */
const KEY = 'tbp.recent-prefixes'
const MAX = 5

export function getRecentPrefixes(ledger: string): string[] {
  try {
    const all = JSON.parse(globalThis.localStorage?.getItem(KEY) ?? '{}') as Record<string, unknown>
    const list = all[ledger]
    return Array.isArray(list) ? list.filter((item): item is string => typeof item === 'string') : []
  } catch {
    return []
  }
}

export function rememberPrefix(ledger: string, prefix: string): void {
  try {
    const all = JSON.parse(globalThis.localStorage?.getItem(KEY) ?? '{}') as Record<string, string[]>
    all[ledger] = [prefix, ...(all[ledger] ?? []).filter((item) => item !== prefix)].slice(0, MAX)
    globalThis.localStorage?.setItem(KEY, JSON.stringify(all))
  } catch {
    // Storage unavailable: the page works without the shortcut list.
  }
}
