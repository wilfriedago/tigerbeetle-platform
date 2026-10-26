import { clientConfig } from './config'
import { ApiError, HttpStatusError, parseRetryAfter, problemFrom } from './errors'
import { REQUEST_ID_HEADER, requestHeaders } from './headers'

const IDEMPOTENT_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])
const RETRY_STATUSES = new Set([502, 503, 504])
/** The longest a retry waits for the Retry-After of a 503. */
const MAX_RETRY_AFTER_MS = 10_000

interface Policy {
  /** Statuses outside 2xx whose body is the operation's answer, returned like a success. */
  answers: readonly number[]
  /** Overrides the configured retries. */
  retries?: number
}

/**
 * The function every generated operation calls (orval's `override.mutator`, ADR-0019). It
 * resolves with the decoded body, and rejects with ApiError for a problem, HttpStatusError for any
 * other failed status, and the error of fetch for a network failure, a timeout or an abort.
 */
export async function fetcher<T>(url: string, init: RequestInit = {}): Promise<T> {
  return send<T>(url, init, { answers: [] })
}

/**
 * The mutator of GET /readyz. Its 503 is a not-ready problem, rejected as ApiError with the
 * dependencies in its extension members, and never retried: the caller wants the state now.
 */
export async function readinessFetcher<T>(url: string, init: RequestInit = {}): Promise<T> {
  return send<T>(url, init, { answers: [], retries: 0 })
}

async function send<T>(url: string, init: RequestInit, policy: Policy): Promise<T> {
  const config = clientConfig()
  const method = (init.method ?? 'GET').toUpperCase()
  // Writes never retry: retrying is the caller's decision, made safe by the Idempotency-Key it keeps.
  const retries = IDEMPOTENT_METHODS.has(method) ? (policy.retries ?? config.retries) : 0
  // One request ID for the call, so the logs show its retries together.
  const headers = requestHeaders(init.headers, config.requestId())
  const target = config.baseUrl.replace(/\/+$/, '') + url
  for (let attempt = 0; ; attempt++) {
    const timeout = AbortSignal.timeout(config.timeoutMs)
    const signal = init.signal ? AbortSignal.any([init.signal, timeout]) : timeout
    let response: Response
    try {
      response = await config.fetch(target, { ...init, headers, method, signal })
    } catch (error) {
      // fetch rejects with a TypeError when the network fails; a timeout or an abort ends the call.
      if (attempt < retries && error instanceof TypeError) {
        await wait(backoff(attempt), init.signal)
        continue
      }
      throw error
    }
    if (response.ok || policy.answers.includes(response.status)) {
      return decode<T>(response)
    }
    if (attempt < retries && RETRY_STATUSES.has(response.status)) {
      await response.body?.cancel()
      await wait(retryDelay(response, attempt), init.signal)
      continue
    }
    throw await failure(response)
  }
}

/** Reads the body once; an empty body, such as a 204's, is undefined. */
async function decode<T>(response: Response): Promise<T> {
  const text = await response.text()
  return (text === '' ? undefined : JSON.parse(text)) as T
}

async function failure(response: Response): Promise<Error> {
  const requestId = response.headers.get(REQUEST_ID_HEADER) ?? undefined
  const text = await response.text()
  const problem = isProblem(response.headers.get('Content-Type')) ? problemFrom(text) : undefined
  if (problem) {
    return new ApiError(problem, requestId, parseRetryAfter(response.headers.get('Retry-After')))
  }
  return new HttpStatusError(response.status, response.statusText, requestId)
}

function isProblem(contentType: string | null): boolean {
  return contentType?.split(';')[0]?.trim().toLowerCase() === 'application/problem+json'
}

/** 300 ms, then 600 ms: the delays ky used. */
function backoff(attempt: number): number {
  return 300 * 2 ** attempt
}

/** A 503 that says when to come back is retried then, within MAX_RETRY_AFTER_MS. */
function retryDelay(response: Response, attempt: number): number {
  const seconds = response.status === 503 ? parseRetryAfter(response.headers.get('Retry-After')) : undefined
  return seconds === undefined ? backoff(attempt) : Math.min(seconds * 1000, MAX_RETRY_AFTER_MS)
}

/** Waits ms milliseconds, or rejects with the abort reason as soon as the caller aborts. */
async function wait(ms: number, signal: AbortSignal | null | undefined): Promise<void> {
  signal?.throwIfAborted()
  // Browsers have no promise-based timer that also listens to a signal, so this one wraps both.
  // oxlint-disable-next-line promise/avoid-new
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(done, ms)
    signal?.addEventListener('abort', abort, { once: true })
    function done() {
      signal?.removeEventListener('abort', abort)
      resolve()
    }
    function abort() {
      clearTimeout(timer)
      reject(signal?.reason)
    }
  })
}
