import { configureClient } from '@tigerbeetle-platform/api-client'

import { configureGraphQL } from './graphql/client'

/**
 * Points the API clients at the platform API, the only server the console talks to (ADR-0003):
 * GraphQL for the slices (ADR-0023), the generated REST client for the system pages. In
 * development Vite proxies /api to the API; in production a reverse proxy does. main.tsx calls it
 * before the first render.
 */
export function configureApi(): void {
  const baseUrl = import.meta.env.VITE_API_URL ?? '/api'
  configureClient({ baseUrl })
  configureGraphQL(baseUrl)
}
