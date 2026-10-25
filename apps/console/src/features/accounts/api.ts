import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  aggregateBalances,
  getAccount,
  getAccountHistory,
  listAccounts,
  listSegments,
  type Metadata,
  queryParams,
  replaceAccountMetadata
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

export interface AccountListParams {
  prefix?: string | undefined
  search?: string | undefined
  asset?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const accountQueries = {
  list: (ledger: string, params: AccountListParams) =>
    queryOptions({
      queryFn: async ({ signal }) => listAccounts(ledger, queryParams(params), { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'list', params] as const
    }),
  detail: (ledger: string, address: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getAccount(ledger, address, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'detail', address] as const
    }),
  history: (ledger: string, address: string, asset: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getAccountHistory(ledger, address, { asset, limit: 100 }, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'history', address, asset] as const
    }),
  segments: (ledger: string, prefix: string) =>
    queryOptions({
      queryFn: async ({ signal }) => listSegments(ledger, queryParams({ prefix }), { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'segments', prefix] as const
    }),
  aggregate: (ledger: string, prefix: string) =>
    queryOptions({
      enabled: prefix.length > 0,
      queryFn: async ({ signal }) => aggregateBalances(ledger, { address: prefix }, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'aggregate', prefix] as const,
      // A prefix sum reads every account under the prefix; it is computed on demand only.
      staleTime: 15_000
    })
}

export function useReplaceAccountMetadata(ledger: string, address: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata) => replaceAccountMetadata(ledger, address, metadata),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'accounts'] })
  })
}
