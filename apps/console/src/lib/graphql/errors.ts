import { ApiError, type Problem } from '@tigerbeetle-platform/api-client'

/** One entry of the errors list of a GraphQL answer. */
export interface GraphQLErrorEntry {
  message: string
  path?: (string | number)[]
  extensions?: Record<string, unknown>
}

/**
 * A GraphQL error, carrying the same problem members as a REST answer (plan section 9.2): code,
 * reason, type, title, status, detail, requestId, retryAfter, posting, results. It extends
 * ApiError, so everything that reads a REST problem (the problem panel, keyIsSpent, the retry
 * policy, the stored-problem view) reads it unchanged. Errors GraphQL raises itself, such as a
 * validation error, have no problem members and become a 400 problem with the message as detail.
 */
export class GraphQLProblemError extends ApiError {
  /** The problem kind: INVALID, NOT_FOUND, REJECTED, UNAVAILABLE and so on. */
  readonly code: string
  /** The path of the field that failed. */
  readonly path: (string | number)[] | undefined

  constructor(entry: GraphQLErrorEntry, requestId?: string) {
    const ext = entry.extensions ?? {}
    super(problemOf(entry, ext), stringOf(ext.requestId) ?? requestId, numberOf(ext.retryAfter))
    this.name = 'GraphQLProblemError'
    this.code = stringOf(ext.code) ?? 'INVALID'
    this.path = entry.path
  }
}

function problemOf(entry: GraphQLErrorEntry, ext: Record<string, unknown>): Problem {
  const status = numberOf(ext.status)
  const title = stringOf(ext.title)
  if (status === undefined || title === undefined) {
    // A parse or validation error of the operation itself: a bug in the console's documents.
    return { detail: entry.message, status: 400, title: 'Invalid GraphQL request', type: 'about:blank' }
  }
  const problem: Problem = { status, title, type: stringOf(ext.type) ?? 'about:blank' }
  const detail = stringOf(ext.detail)
  if (detail !== undefined) {
    problem.detail = detail
  }
  const requestId = stringOf(ext.requestId)
  if (requestId !== undefined) {
    problem.requestId = requestId
  }
  const posting = numberOf(ext.posting)
  if (posting !== undefined) {
    problem.posting = posting
  }
  if (Array.isArray(ext.results) && ext.results.every((r) => typeof r === 'string')) {
    problem.results = ext.results
  }
  return problem
}

function stringOf(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

function numberOf(value: unknown): number | undefined {
  return typeof value === 'number' ? value : undefined
}
