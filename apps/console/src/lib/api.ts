import { configureClient } from '@tigerbeetle-platform/api-client'

/**
 * Points the generated API client at the platform API, the only server the console talks to
 * (ADR-0003). In development Vite proxies /api to the API; in production a reverse proxy does.
 * main.tsx calls it before the first render.
 */
export function configureApi(): void {
  configureClient({ baseUrl: import.meta.env.VITE_API_URL ?? '/api' })
}
