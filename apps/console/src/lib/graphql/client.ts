import {
  ApiError,
  HttpStatusError,
  ORGANIZATION_HEADER,
  problemFrom,
  problemTypeOf,
  randomRequestId,
  REQUEST_ID_HEADER
} from '@tigerbeetle-platform/api-client'

import { type TypedDocumentString } from '~/graphql/gen/graphql'

import { type GraphQLErrorEntry, GraphQLProblemError } from './errors'

/** Time allowed for one GraphQL request. */
const TIMEOUT_MS = 15_000

let endpoint = '/api/graphql'
let organization: string | undefined

/** Points the client at the API; the endpoint is POST {baseUrl}/graphql. */
export function configureGraphQL(baseUrl: string): void {
  endpoint = `${baseUrl.replace(/\/+$/, '')}/graphql`
}

/** Names the organization every request acts for, sent as X-Organization (ADR-0027). */
export function setGraphQLOrganization(alias: string | undefined): void {
  organization = alias
}

export interface RequestOptions {
  /** The signal TanStack Query passes, combined with the timeout. */
  signal?: AbortSignal
}

/**
 * Posts one GraphQL operation (ADR-0023). Every request carries a fresh X-Request-Id, and an
 * answer with any error rejects with GraphQLProblemError, built from the first error: data and
 * errors are never mixed in the UI. Nothing is retried here; TanStack Query decides that, as for
 * REST.
 */
export async function request<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
  variables: TVariables,
  options: RequestOptions = {}
): Promise<TResult> {
  const timeout = AbortSignal.timeout(TIMEOUT_MS)
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    [REQUEST_ID_HEADER]: randomRequestId()
  }
  if (organization) {
    headers[ORGANIZATION_HEADER] = organization
  }
  // The session cookie authenticates the request (ADR-0030); the console holds no token.
  const response = await globalThis.fetch(endpoint, {
    body: JSON.stringify({ query: document.toString(), variables }),
    credentials: 'same-origin',
    headers,
    method: 'POST',
    signal: options.signal ? AbortSignal.any([options.signal, timeout]) : timeout
  })
  const requestId = response.headers.get(REQUEST_ID_HEADER) ?? undefined
  const text = await response.text()
  let body: { data?: TResult | null; errors?: GraphQLErrorEntry[] } | undefined
  try {
    body = JSON.parse(text) as typeof body
  } catch {
    throw new HttpStatusError(response.status, response.statusText, requestId)
  }
  const first = body?.errors?.[0]
  if (first) {
    throw new GraphQLProblemError(first, requestId)
  }
  // A body the API could not read as a GraphQL request at all is answered with a REST problem.
  const problem = isProblem(response) ? problemFrom(text) : undefined
  if (problem) {
    throw new ApiError(problem, problem.requestId ?? requestId)
  }
  if (!response.ok || body?.data === undefined || body.data === null) {
    throw new HttpStatusError(response.status, response.statusText, requestId)
  }
  return body.data
}

/**
 * The error a single-object query answers when GraphQL returns null for it (plan section 8.5): the
 * 404 problem the routes already turn into their not-found view.
 */
export function notFound(what: string): ApiError {
  return new ApiError({ detail: `${what} does not exist`, status: 404, title: 'Not found', type: problemTypeOf('not-found') })
}

function isProblem(response: Response): boolean {
  return response.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() === 'application/problem+json'
}
