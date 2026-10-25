import { useState } from 'react'

import { randomRequestId } from '@tigerbeetle-platform/api-client'

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
