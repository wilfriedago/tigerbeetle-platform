import { type Problem } from '../gen/model'

/**
 * The base of every problem type the API names: the entry of the problem in the catalogue,
 * docs/problems.md, the same in every deployment (ADR-0025). The reason follows it.
 */
export const PROBLEM_TYPE_BASE = 'https://github.com/wilfriedago/tigerbeetle-platform/blob/develop/docs/problems.md#'

/** The problem type of a reason: problemTypeOf('exceeds-credits'). */
export function problemTypeOf(reason: string): string {
  return PROBLEM_TYPE_BASE + reason
}

/** The reason of a problem type the API names, such as "exceeds-credits"; undefined for about:blank or any other type. */
export function problemReason(type: string | undefined): string | undefined {
  return type?.startsWith(PROBLEM_TYPE_BASE) ? type.slice(PROBLEM_TYPE_BASE.length) || undefined : undefined
}

/** Thrown when the API answers with an RFC 9457 problem. */
export class ApiError extends Error {
  readonly problem: Problem
  readonly status: number
  readonly requestId: string | undefined
  /** Seconds to wait before retrying, from the Retry-After of 409 request-in-flight and 503 write-pending. */
  readonly retryAfter: number | undefined

  constructor(problem: Problem, requestId?: string, retryAfter?: number) {
    super(problem.detail ? `${problem.title}: ${problem.detail}` : problem.title)
    this.name = 'ApiError'
    this.problem = problem
    this.status = problem.status
    this.requestId = requestId
    this.retryAfter = retryAfter
  }

  /** The problem type, an absolute URI such as problemTypeOf('exceeds-credits'), or "about:blank". */
  get type(): string {
    return this.problem.type
  }

  /** The reason the problem type names, such as "exceeds-credits"; undefined for about:blank. */
  get reason(): string | undefined {
    return problemReason(this.problem.type)
  }
}

/** Thrown when the API answers with a non-2xx status that is not a problem document. */
export class HttpStatusError extends Error {
  readonly status: number
  readonly requestId: string | undefined

  constructor(status: number, statusText: string, requestId: string | undefined) {
    super(`unexpected status ${status} ${statusText}`.trim())
    this.name = 'HttpStatusError'
    this.status = status
    this.requestId = requestId
  }
}

/**
 * The seconds a Retry-After header asks to wait, in either form RFC 9110 allows: a number of
 * seconds, or an HTTP date, counted from `now`. A date in the past is 0.
 */
export function parseRetryAfter(value: string | null, now: number = Date.now()): number | undefined {
  const text = value?.trim() ?? ''
  if (/^\d+$/.test(text)) {
    return Number(text)
  }
  // Every HTTP date form starts with the day name (Date.parse alone reads "-1" as a year) and is in
  // GMT, which the obsolete asctime form leaves unsaid.
  const date = /^[A-Za-z]{3,9},? /.test(text) ? Date.parse(text.endsWith('GMT') ? text : `${text} GMT`) : Number.NaN
  return Number.isNaN(date) ? undefined : Math.max(0, Math.ceil((date - now) / 1000))
}

/**
 * The problem in a problem+json body, or undefined when the body is not one. RFC 9457 makes
 * `type` optional and reads its absence as "about:blank".
 */
export function problemFrom(text: string): Problem | undefined {
  let body: unknown
  try {
    body = JSON.parse(text)
  } catch {
    return undefined
  }
  if (typeof body !== 'object' || body === null) {
    return undefined
  }
  const { title, status, type } = body as Record<string, unknown>
  if (typeof title !== 'string' || typeof status !== 'number') {
    return undefined
  }
  return { ...(body as Problem), type: typeof type === 'string' ? type : 'about:blank' }
}
