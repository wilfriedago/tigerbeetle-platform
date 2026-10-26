import { randomRequestId } from './headers'

/** What every call of the client shares. */
export interface ClientConfig {
  /**
   * Base URL of the API, such as "http://localhost:8080", or "/api" behind the console's dev
   * proxy. Empty means the origin of the page.
   */
  baseUrl: string
  /** The fetch every call goes through. */
  fetch: typeof globalThis.fetch
  /** Time allowed for each attempt, in milliseconds. */
  timeoutMs: number
  /** Further attempts of a GET, HEAD or OPTIONS after a network error or a 502, 503 or 504. */
  retries: number
  /** Generates the X-Request-Id of each call. */
  requestId: () => string
  /**
   * Returns the access token sent as `Authorization: Bearer` (ADR-0026), read before each call so
   * a refreshed token is used. Leave it out when a cookie session authenticates, as in the console.
   */
  accessToken?: () => string | undefined | Promise<string | undefined>
  /** The alias of the organization calls act for, sent as X-Organization (ADR-0027). */
  organization?: string
}

const defaults: ClientConfig = {
  baseUrl: '',
  // Looked up at call time, so a fetch replaced after start-up is the one used.
  fetch: async (input, init) => globalThis.fetch(input, init),
  requestId: randomRequestId,
  retries: 2,
  timeoutMs: 15_000
}

let current: ClientConfig = defaults

/**
 * Sets the configuration of every later call; options left out take their defaults. The console
 * calls it once at start-up, tests call it with their own fetch.
 */
export function configureClient(options: Partial<ClientConfig>): void {
  current = { ...defaults, ...options }
}

/** The configuration calls use now. */
export function clientConfig(): Readonly<ClientConfig> {
  return current
}
