import { activeOrganization } from './organization'

/**
 * "Copy as curl" (docs/console.md, New transaction): the exact request the console sends, written
 * against the API's public address rather than the development proxy.
 */
const PUBLIC_API_URL = (import.meta.env.VITE_API_PUBLIC_URL ?? 'http://localhost:8080').replace(/\/$/, '')

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", String.raw`'\''`)}'`
}

interface CurlRequest {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE'
  /** The API path, such as "/v1/ledgers/eu-payments/transactions". */
  path: string
  body?: unknown
  idempotencyKey?: string
  /** The organization the request acts for; the current one when left out (ADR-0027). */
  organization?: string
}

export function curlCommand({ method, path, body, idempotencyKey, organization = activeOrganization() }: CurlRequest): string {
  const lines = [`curl -X ${method} ${shellQuote(`${PUBLIC_API_URL}${path}`)}`]
  // The console's own requests use its session cookie; a script brings an access token or an API
  // key (ADR-0026, ADR-0029). Double quotes let the shell expand $TOKEN.
  lines.push(`  -H "Authorization: Bearer $TOKEN"`)
  if (organization) {
    lines.push(`  -H ${shellQuote(`X-Organization: ${organization}`)}`)
  }
  if (body !== undefined) {
    lines.push(`  -H 'Content-Type: application/json'`)
  }
  if (idempotencyKey) {
    lines.push(`  -H ${shellQuote(`Idempotency-Key: ${idempotencyKey}`)}`)
  }
  if (body !== undefined) {
    lines.push(`  -d ${shellQuote(JSON.stringify(body))}`)
  }
  return lines.join(' \\\n')
}

/** An API path built from segments, each one escaped the way the API client escapes it. */
export function apiPath(...segments: string[]): string {
  return `/v1/${segments.map((segment) => encodeURIComponent(segment)).join('/')}`
}
