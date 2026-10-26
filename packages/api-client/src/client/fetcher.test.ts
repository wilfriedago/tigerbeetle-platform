import { afterEach, beforeEach, describe, expect, it, vi } from 'vite-plus/test'

import { configureClient } from './config'
import { ApiError, HttpStatusError, problemTypeOf } from './errors'
import { fetcher, readinessFetcher } from './fetcher'

interface Call {
  url: string
  init: RequestInit
}

/** A fetch that answers from a list, in order, and records every call. */
function fakeFetch(...answers: (Response | Error)[]) {
  const calls: Call[] = []
  const fetch = async (input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> => {
    calls.push({ init, url: input instanceof Request ? input.url : input.toString() })
    const answer = answers.shift()
    if (answer === undefined) {
      throw new Error('the test gave no answer for this call')
    }
    if (answer instanceof Error) {
      throw answer
    }
    return answer
  }
  return { calls, fetch }
}

/** A fetch that never answers and rejects when its signal aborts, as fetch does. */
async function hangingFetch(_input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const { signal } = init
  return new Promise((_resolve, reject) => {
    signal?.addEventListener('abort', () => reject(signal.reason), { once: true })
  })
}

function json(status: number, body: unknown): Response {
  return Response.json(body, { status })
}

function problem(status: number, type: string, headers: Record<string, string> = {}): Response {
  return Response.json(
    { status, title: type, type: problemTypeOf(type) },
    { headers: { 'Content-Type': 'application/problem+json', 'X-Request-Id': 'srv-1', ...headers }, status }
  )
}

/** The error a promise rejects with; started now, awaited later, so timers can run in between. */
async function rejection(promise: Promise<unknown>): Promise<unknown> {
  try {
    await promise
  } catch (error) {
    return error
  }
  throw new Error('the call resolved')
}

function header(call: Call | undefined, name: string): string | null {
  return new Headers(call?.init.headers).get(name)
}

let ids = 0
function configure(fetch: typeof globalThis.fetch, options: { retries?: number; timeoutMs?: number } = {}) {
  configureClient({ baseUrl: 'http://api.test/', fetch, requestId: () => `req-${++ids}`, ...options })
}

beforeEach(() => {
  ids = 0
})

afterEach(() => {
  vi.useRealTimers()
  configureClient({})
})

describe('fetcher', () => {
  it('sends the call to the base URL with a request ID and resolves with the body', async () => {
    const { calls, fetch } = fakeFetch(json(201, { id: 'tx-1' }))
    configure(fetch)
    const body = JSON.stringify({ postings: [] })
    await expect(
      fetcher('/v1/ledgers/eu/transactions', { body, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'idem_1' }, method: 'POST' })
    ).resolves.toEqual({ id: 'tx-1' })
    expect(calls).toHaveLength(1)
    expect(calls[0]?.url).toBe('http://api.test/v1/ledgers/eu/transactions')
    expect(calls[0]?.init.method).toBe('POST')
    expect(calls[0]?.init.body).toBe(body)
    expect(header(calls[0], 'X-Request-Id')).toBe('req-1')
    expect(header(calls[0], 'Idempotency-Key')).toBe('idem_1')
    expect(header(calls[0], 'Accept')).toBe('application/json, application/problem+json')
  })

  it('resolves an empty answer, such as a 204, with undefined', async () => {
    const { fetch } = fakeFetch(new Response(null, { status: 204 }))
    configure(fetch)
    await expect(fetcher('/v1/ledgers/eu/rules/x', { method: 'DELETE' })).resolves.toBeUndefined()
  })

  it('rejects a problem with ApiError, its request ID and Retry-After', async () => {
    const { fetch } = fakeFetch(problem(409, 'request-in-flight', { 'Retry-After': '2' }))
    configure(fetch)
    const error = await rejection(fetcher('/v1/ledgers/eu/transactions', { method: 'POST' }))
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ requestId: 'srv-1', retryAfter: 2, status: 409, type: problemTypeOf('request-in-flight') })
  })

  it('rejects any other failed status with HttpStatusError', async () => {
    const { fetch } = fakeFetch(
      new Response('<html>bad gateway</html>', { headers: { 'Content-Type': 'text/html' }, status: 502, statusText: 'Bad Gateway' }),
      new Response('not json', { headers: { 'Content-Type': 'application/problem+json' }, status: 400 })
    )
    configure(fetch, { retries: 0 })
    await expect(fetcher('/v1/ledgers')).rejects.toEqual(new HttpStatusError(502, 'Bad Gateway', undefined))
    await expect(fetcher('/v1/ledgers')).rejects.toBeInstanceOf(HttpStatusError)
  })

  it('retries a GET after 502, 503, 504 and a network error, with the same request ID', async () => {
    vi.useFakeTimers()
    const { calls, fetch } = fakeFetch(problem(503, 'dependency-unavailable'), new TypeError('Failed to fetch'), json(200, { data: [] }))
    configure(fetch)
    const answer = fetcher('/v1/ledgers')
    await vi.runAllTimersAsync()
    await expect(answer).resolves.toEqual({ data: [] })
    expect(calls.map((call) => header(call, 'X-Request-Id'))).toEqual(['req-1', 'req-1', 'req-1'])
  })

  it('gives up after the configured retries', async () => {
    vi.useFakeTimers()
    const { calls, fetch } = fakeFetch(json(504, {}), json(502, {}), json(504, {}))
    configure(fetch, { retries: 2 })
    const answer = rejection(fetcher('/v1/ledgers'))
    await vi.runAllTimersAsync()
    expect(await answer).toBeInstanceOf(HttpStatusError)
    expect(calls).toHaveLength(3)
  })

  it('waits for the Retry-After of a 503, at most 10 seconds', async () => {
    vi.useFakeTimers()
    const { calls, fetch } = fakeFetch(problem(503, 'dependency-unavailable', { 'Retry-After': '60' }), json(200, {}))
    configure(fetch)
    const answer = fetcher('/v1/ledgers')
    await vi.advanceTimersByTimeAsync(9999)
    expect(calls).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1)
    await expect(answer).resolves.toEqual({})
    expect(calls).toHaveLength(2)
  })

  it.each([
    ['a write', 'POST', problem(503, 'write-pending', { 'Retry-After': '1' })],
    ['a 409', 'GET', problem(409, 'request-in-flight', { 'Retry-After': '1' })],
    ['a 500', 'GET', problem(500, 'internal')]
  ])('never retries %s', async (_name, method, response) => {
    const { calls, fetch } = fakeFetch(response)
    configure(fetch)
    await expect(fetcher('/v1/ledgers/eu/transactions', { method })).rejects.toBeInstanceOf(ApiError)
    expect(calls).toHaveLength(1)
  })

  it('rejects with a TimeoutError when an attempt takes too long, without retrying', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(hangingFetch)
    configure(fetch, { timeoutMs: 10 })
    await expect(fetcher('/v1/ledgers')).rejects.toMatchObject({ name: 'TimeoutError' })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('stops when the caller aborts, during a call or while waiting to retry', async () => {
    configure(hangingFetch)
    const during = new AbortController()
    const call = fetcher('/v1/ledgers', { signal: during.signal })
    during.abort()
    await expect(call).rejects.toMatchObject({ name: 'AbortError' })

    vi.useFakeTimers()
    const { calls, fetch } = fakeFetch(json(503, {}), json(200, {}))
    configure(fetch)
    const waiting = new AbortController()
    const retry = rejection(fetcher('/v1/ledgers', { signal: waiting.signal }))
    await vi.advanceTimersByTimeAsync(100)
    waiting.abort()
    expect(await retry).toMatchObject({ name: 'AbortError' })
    expect(calls).toHaveLength(1)
  })
})

describe('readinessFetcher', () => {
  it('rejects the not-ready problem of a 503 without retrying', async () => {
    const dependencies = { postgres: { latencyMs: 1, status: 'up' }, tigerbeetle: { error: 'unreachable', latencyMs: 2, status: 'down' } }
    const { calls, fetch } = fakeFetch(
      Response.json(
        { dependencies, status: 503, title: 'Not ready', type: problemTypeOf('not-ready') },
        { headers: { 'Content-Type': 'application/problem+json' }, status: 503 }
      )
    )
    configure(fetch)
    const error = await rejection(readinessFetcher('/readyz'))
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ problem: { dependencies }, reason: 'not-ready', status: 503 })
    expect(calls).toHaveLength(1)
  })

  it('resolves the body of a 200', async () => {
    const { fetch } = fakeFetch(json(200, { status: 'ready' }))
    configure(fetch)
    await expect(readinessFetcher('/readyz')).resolves.toEqual({ status: 'ready' })
  })
})
