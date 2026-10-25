import { createClient } from '@tigerbeetle-platform/api-client'

/**
 * The one API client of the console (ADR-0003: the console only talks to the platform API). In
 * development Vite proxies /api to the API; in production a reverse proxy does.
 */
export const api = createClient({
  baseUrl: import.meta.env.VITE_API_URL ?? '/api'
})
