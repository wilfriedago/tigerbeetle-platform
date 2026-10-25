import { queryOptions } from '@tanstack/react-query'

import { api } from '~/lib/api'

/** How often the console asks the API for its health (docs/console.md, Top bar). */
export const HEALTH_INTERVAL_MS = 5000

export const systemQueries = {
  info: () =>
    queryOptions({
      queryFn: async ({ signal }) => api.getInfo({ signal }),
      queryKey: ['system', 'info'] as const,
      refetchInterval: HEALTH_INTERVAL_MS,
      // Health must reflect now, and failures are themselves the answer.
      retry: false,
      staleTime: 0
    }),
  reconciler: () =>
    queryOptions({
      queryFn: async ({ signal }) => api.getReconcilerStatus({ signal }),
      queryKey: ['system', 'reconciler'] as const,
      refetchInterval: HEALTH_INTERVAL_MS
    })
}
