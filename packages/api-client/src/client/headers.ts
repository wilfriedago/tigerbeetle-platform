/** Header carried on every request and echoed back by the API. */
export const REQUEST_ID_HEADER = 'X-Request-Id'

/** Header that names the organization a call acts for (ADR-0027). */
export const ORGANIZATION_HEADER = 'X-Organization'

/** Header that makes a write that moves money safe to retry (ADR-0007). */
export const IDEMPOTENCY_KEY_HEADER = 'Idempotency-Key'

/**
 * Random request ID that also works outside secure contexts (plain http on a LAN address),
 * where crypto.randomUUID is not available but getRandomValues is.
 */
export function randomRequestId(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

/**
 * The headers of one call: those the generated function sets (Content-Type, Idempotency-Key) and
 * the caller's, plus a request ID unless the caller chose one, and the media types the API answers
 * with.
 */
export function requestHeaders(init: HeadersInit | undefined, requestId: string): Headers {
  const headers = new Headers(init)
  if (!headers.has(REQUEST_ID_HEADER)) {
    headers.set(REQUEST_ID_HEADER, requestId)
  }
  headers.set('Accept', 'application/json, application/problem+json')
  return headers
}
