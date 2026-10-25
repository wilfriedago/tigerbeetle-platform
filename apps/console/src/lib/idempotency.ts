import { useState } from 'react'

import { ApiError, randomRequestId } from '@tigerbeetle-platform/api-client'

/** A new Idempotency-Key, "idem_" plus 32 random hex characters split like a UUID. */
export function newIdempotencyKey(): string {
  const hex = randomRequestId()
  return `idem_${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export interface IdempotencyKey {
  /** The key the next submission will send. */
  key: string
  /**
   * The key for a body: the current one while the body is the one last sent (a retry replays the
   * same write), a new one as soon as the body changes (ADR-0007). A key that was never sent is
   * kept for the first body.
   */
  keyFor: (body: unknown) => string
  /** Forces a new key, for "retry with a new key" after a decline. */
  renew: () => void
}

/**
 * The Idempotency-Key of a form (docs/console.md, Writes): created when the form opens, reused
 * on retry, renewed when the request changes.
 */
export function useIdempotencyKey(): IdempotencyKey {
  const [state, setState] = useState(() => ({ body: undefined as string | undefined, key: newIdempotencyKey() }))
  const keyFor = (body: unknown) => {
    const serialized = JSON.stringify(body)
    if (state.body === undefined || state.body === serialized) {
      if (state.body === undefined) {
        setState({ body: serialized, key: state.key })
      }
      return state.key
    }
    const key = newIdempotencyKey()
    setState({ body: serialized, key })
    return key
  }
  return {
    key: state.key,
    keyFor,
    renew: () => setState({ body: undefined, key: newIdempotencyKey() })
  }
}

/**
 * Whether a failed write already has its final answer under its key (ADR-0007): a declined or
 * rejected request is stored with its key, so sending the key again only replays that answer and
 * the next attempt needs a new key. Network failures, 5xx answers, `request-in-flight` and
 * `write-pending` are not final: retrying with the same key is the safe move.
 */
export function keyIsSpent(error: unknown): boolean {
  if (!(error instanceof ApiError) || error.status >= 500) {
    return false
  }
  const { type } = error
  return !type.endsWith('/request-in-flight') && !type.endsWith('/write-pending')
}
