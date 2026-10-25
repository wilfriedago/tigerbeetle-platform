import { QueryClient } from '@tanstack/react-query'

import { ApiError, HttpStatusError } from '@tigerbeetle-platform/api-client'

/** Client errors (4xx) are answers, not outages: retrying them only repeats the answer. */
function shouldRetry(failureCount: number, error: unknown): boolean {
  if ((error instanceof ApiError || error instanceof HttpStatusError) && error.status < 500) {
    return false
  }
  return failureCount < 2
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
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
