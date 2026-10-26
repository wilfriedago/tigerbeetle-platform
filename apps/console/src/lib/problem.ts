import { ApiError, HttpStatusError } from '@tigerbeetle-platform/api-client'

/**
 * One shape for every failure the console shows: an RFC 9457 problem from the API, a non-problem
 * HTTP status, a network failure or a bug. The Problem Panel renders it (12 Feedback).
 */
export interface ProblemView {
  title: string
  status?: number
  type?: string
  detail?: string
  requestId?: string
  /** Seconds the API asked to wait before a retry: the Retry-After of 409 request-in-flight and 503 write-pending. */
  retryAfter?: number
  /** Problem extension members, such as the per-posting results of a declined transaction. */
  extensions: Record<string, unknown>
}

const KNOWN_MEMBERS = new Set(['type', 'title', 'status', 'detail', 'instance', 'requestId'])

export function toProblem(error: unknown): ProblemView {
  if (error instanceof ApiError) {
    const extensions: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(error.problem)) {
      if (!KNOWN_MEMBERS.has(key)) {
        extensions[key] = value
      }
    }
    const view: ProblemView = { extensions, status: error.status, title: error.problem.title, type: error.problem.type }
    if (error.problem.detail) {
      view.detail = error.problem.detail
    }
    const requestId = error.problem.requestId ?? error.requestId
    if (requestId) {
      view.requestId = requestId
    }
    if (error.retryAfter !== undefined) {
      view.retryAfter = error.retryAfter
    }
    return view
  }
  if (error instanceof HttpStatusError) {
    const view: ProblemView = { detail: error.message, extensions: {}, status: error.status, title: 'Unexpected response' }
    if (error.requestId) {
      view.requestId = error.requestId
    }
    return view
  }
  if (isUnreachable(error)) {
    return { detail: 'The API did not answer. Check that it is running and reachable.', extensions: {}, title: 'API unreachable' }
  }
  return { detail: error instanceof Error ? error.message : String(error), extensions: {}, title: 'Something went wrong' }
}

/**
 * Whether the API never answered: fetch rejects with a TypeError when the network fails, and with
 * a TimeoutError DOMException when the client's timeout ends an attempt.
 */
function isUnreachable(error: unknown): boolean {
  return error instanceof TypeError || (error instanceof DOMException && error.name === 'TimeoutError')
}

/** The problem type of an error, when the API sent one: problemTypeOf('exceeds-credits') or "about:blank". */
export function problemType(error: unknown): string | undefined {
  return error instanceof ApiError ? error.problem.type : undefined
}

export function isNotFound(error: unknown): boolean {
  return (error instanceof ApiError || error instanceof HttpStatusError) && error.status === 404
}
