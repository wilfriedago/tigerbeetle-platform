import { queryOptions } from '@tanstack/react-query'

import { getWriteRequest, listWriteRequests, type Operation, queryParams, type RequestStatus } from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

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
      queryFn: async ({ signal }) => listWriteRequests(ledger, queryParams(params), { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'list', params] as const,
      // Pending records change state without any action here.
      refetchInterval: 10_000
    }),
  detail: (ledger: string, key: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getWriteRequest(ledger, key, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'detail', key] as const,
      refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2000 : false)
    })
}
