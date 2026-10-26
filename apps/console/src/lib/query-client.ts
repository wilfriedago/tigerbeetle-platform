import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'

import { ApiError, HttpStatusError } from '@tigerbeetle-platform/api-client'

import { signIn } from './auth/session'

/** Client errors (4xx) are answers, not outages: retrying them only repeats the answer. */
function shouldRetry(failureCount: number, error: unknown): boolean {
  if ((error instanceof ApiError || error instanceof HttpStatusError) && error.status < 500) {
    return false
  }
  return failureCount < 2
}

/** A 401 means the session ended (ADR-0030): sign in again and come back to this page. */
function onError(error: unknown): void {
  if ((error instanceof ApiError || error instanceof HttpStatusError) && error.status === 401) {
    signIn()
  }
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    mutationCache: new MutationCache({ onError }),
    queryCache: new QueryCache({ onError }),
    defaultOptions: {
      mutations: {
        // Writes are retried by the person, with the same Idempotency-Key, never automatically.
        retry: false
      },
      queries: {
        retry: shouldRetry,
        // Balances move with every write; a short freshness window keeps lists cheap to revisit
        // without showing stale money for long.
        staleTime: 5000
      }
    }
  })
}
