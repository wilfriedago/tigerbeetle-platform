import { ApiError, HttpStatusError, problemTypeOf } from '@tigerbeetle-platform/api-client'
import { afterEach, describe, expect, it, vi } from 'vite-plus/test'

import { TypedDocumentString } from '~/graphql/gen/graphql'

import { configureGraphQL, notFound, request } from './client'
import { GraphQLProblemError } from './errors'
import { present } from './present'

const Doc = new TypedDocumentString<{ ledger: { name: string } | null }, { name: string }>(
  'query Ledger($name: String!) { ledger(name: $name) { name } }'
)

function answer(body: unknown, init: ResponseInit = {}): Response {
  return Response.json(body, { headers: { 'X-Request-Id': 'srv-1' }, ...init })
}

afterEach(() => {
  vi.unstubAllGlobals()
  configureGraphQL('/api')
})

describe('request', () => {
  it('posts the document and variables with a request ID and resolves with the data', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async () => answer({ data: { ledger: { name: 'eu' } } }))
    vi.stubGlobal('fetch', fetch)
    configureGraphQL('http://api.test/')
    await expect(request(Doc, { name: 'eu' })).resolves.toEqual({ ledger: { name: 'eu' } })
    const [url, init] = fetch.mock.calls[0] ?? []
    expect(url).toBe('http://api.test/graphql')
    expect(init?.method).toBe('POST')
    expect(JSON.parse(init?.body as string)).toEqual({ query: Doc.toString(), variables: { name: 'eu' } })
    expect(new Headers(init?.headers).get('X-Request-Id')).toMatch(/^[0-9a-f]{32}$/)
  })

  it('rejects with the first error as a problem, which is an ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof globalThis.fetch>(async () =>
        answer({
          data: null,
          errors: [
            {
              extensions: {
                code: 'CONFLICT',
                reason: 'request-in-flight',
                requestId: 'req-7',
                retryAfter: 1,
                status: 409,
                title: 'Request in flight',
                type: problemTypeOf('request-in-flight')
              },
              message: 'still running',
              path: ['createTransaction']
            }
          ]
        })
      )
    )
    const error = await request(Doc, { name: 'eu' }).catch((error_: unknown) => error_)
    expect(error).toBeInstanceOf(GraphQLProblemError)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({
      code: 'CONFLICT',
      path: ['createTransaction'],
      requestId: 'req-7',
      retryAfter: 1,
      status: 409,
      type: problemTypeOf('request-in-flight')
    })
  })

  it('reads an error without problem members, such as a validation error, as a 400', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof globalThis.fetch>(async () =>
        answer({ errors: [{ extensions: { code: 'GRAPHQL_VALIDATION_FAILED' }, message: 'Cannot query field "x"' }] }, { status: 422 })
      )
    )
    const error = await request(Doc, { name: 'eu' }).catch((error_: unknown) => error_)
    expect(error).toMatchObject({ code: 'GRAPHQL_VALIDATION_FAILED', problem: { detail: 'Cannot query field "x"', status: 400 }, requestId: 'srv-1' })
  })

  it('rejects the problem the API answers for a body that is no GraphQL request with ApiError', async () => {
    const problem = { requestId: 'srv-2', status: 413, title: 'Request Entity Too Large', type: 'about:blank' }
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof globalThis.fetch>(async () => Response.json(problem, { headers: { 'Content-Type': 'application/problem+json' }, status: 413 }))
    )
    const error = await request(Doc, { name: 'eu' }).catch((error_: unknown) => error_)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ problem, requestId: 'srv-2', status: 413 })
  })

  it('rejects an answer that is not GraphQL with HttpStatusError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof globalThis.fetch>(async () => new Response('<html>', { status: 502, statusText: 'Bad Gateway' }))
    )
    await expect(request(Doc, { name: 'eu' })).rejects.toBeInstanceOf(HttpStatusError)
  })
})

describe('present', () => {
  it('drops null members deeply and keeps everything else', () => {
    expect(present({ a: null, b: [{ c: null, d: 0 }], e: '' })).toEqual({ b: [{ d: 0 }], e: '' })
  })
})

describe('notFound', () => {
  it('is the 404 problem the routes turn into their not-found view', () => {
    expect(notFound('ledger eu')).toMatchObject({ status: 404, type: problemTypeOf('not-found') })
  })
})
