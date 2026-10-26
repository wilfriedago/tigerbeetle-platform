import { useParams } from '@tanstack/react-router'

/**
 * The organization the console acts for (ADR-0027, ADR-0030). It is the $org of the URL, so two
 * tabs on two organizations never mix data; the $org route sets it before its pages load, and every
 * API call sends it as X-Organization.
 */
const KEY = 'tbp.last-organization'

let active: string | undefined

/** The organization of the current page, or undefined outside one (Preferences). */
export function activeOrganization(): string | undefined {
  return active
}

/** Records the organization of the page; the $org route calls it. */
export function setActiveOrganization(alias: string): void {
  active = alias
  try {
    globalThis.localStorage?.setItem(KEY, alias)
  } catch {
    // Private mode: "/" then opens the first organization instead of the last one.
  }
}

/** The organization opened last in this browser, so "/" can land there. */
export function getLastOrganization(): string | undefined {
  try {
    return globalThis.localStorage?.getItem(KEY) ?? undefined
  } catch {
    return undefined
  }
}

/**
 * The organization links point into: the $org of the URL, or on pages outside one (Preferences),
 * the one used last.
 */
export function useOrganization(): string {
  const params = useParams({ strict: false })
  return params.org ?? active ?? getLastOrganization() ?? ''
}
