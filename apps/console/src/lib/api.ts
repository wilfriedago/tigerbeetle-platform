import { configureClient } from '@tigerbeetle-platform/api-client'

import { configureSession } from './auth/session'
import { configureGraphQL, setGraphQLOrganization } from './graphql/client'

/**
 * Points the API clients at the platform API, the only server the console talks to (ADR-0003):
 * GraphQL for the slices (ADR-0023), the generated REST client for the system pages. In
 * development Vite proxies /api to the API; in production a reverse proxy does. main.tsx calls it
 * before the first render.
 */
export function configureApi(): void {
  configureClient({ baseUrl: baseUrl() })
  configureGraphQL(baseUrl())
  configureSession(baseUrl())
}

function baseUrl(): string {
  return import.meta.env.VITE_API_URL ?? '/api'
}

/** Makes every later call act for the organization (ADR-0027); the $org route calls it. */
export function actForOrganization(alias: string | undefined): void {
  configureClient({ baseUrl: baseUrl(), ...(alias ? { organization: alias } : {}) })
  setGraphQLOrganization(alias)
}
