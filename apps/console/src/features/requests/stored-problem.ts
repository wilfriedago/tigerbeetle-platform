import { ApiError, type Problem, type WriteRequestDetail } from '@tigerbeetle-platform/api-client'

const TITLES: Record<string, string> = {
  capture: 'Capture declined',
  hold: 'Hold declined',
  revert: 'Revert declined',
  transaction: 'Transaction declined',
  void: 'Void declined'
}

/**
 * The answer stored under a key, as the problem the API would send again on a retry (ADR-0007).
 * Records keep the domain problem (reason, detail, failing posting), not the HTTP document, so the
 * type and title are rebuilt here the way the HTTP adapter renders them.
 */
export function storedProblem(record: WriteRequestDetail): ApiError | undefined {
  if (record.status !== 'declined' && record.status !== 'failed') {
    return undefined
  }
  const response = (record.response ?? {}) as Record<string, unknown>
  const reason = typeof response.reason === 'string' ? response.reason : undefined
  const problem: Problem = {
    ...response,
    status: record.responseStatus ?? (typeof response.status === 'number' ? response.status : 422),
    title: typeof response.title === 'string' ? response.title : (TITLES[record.operation] ?? 'Write declined'),
    type: typeof response.type === 'string' ? response.type : (record.problemType ?? (reason ? `/problems/${reason}` : 'about:blank'))
  }
  if (typeof response.detail === 'string') {
    problem.detail = response.detail
  }
  return new ApiError(problem, record.requestId)
}
