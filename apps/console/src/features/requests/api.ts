import { queryOptions } from '@tanstack/react-query'

import { type Operation, type RequestStatus } from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { api } from '~/lib/api'

export interface RequestListParams {
  status?: RequestStatus | undefined
  operation?: Operation | undefined
  search?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const requestQueries = {
  list: (ledger: string, params: RequestListParams) =>
    queryOptions({
      queryFn: async ({ signal }) => api.requests.list(ledger, params, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'list', params] as const,
      // Pending records change state without any action here.
      refetchInterval: 10_000
    }),
  detail: (ledger: string, key: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.requests.get(ledger, key, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'detail', key] as const,
      refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2000 : false)
    })
}
