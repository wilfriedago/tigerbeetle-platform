import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  createLedger,
  type CreateLedgerRequest,
  getLedger,
  getLedgerStats,
  listAssets,
  listLedgers,
  type Metadata,
  queryParams,
  replaceLedgerMetadata,
  search
} from '@tigerbeetle-platform/api-client'

/**
 * Query keys. Everything inside a ledger starts with ["ledger", name], so a write can refresh
 * every view of that ledger with one invalidation.
 */
export const ledgerKeys = {
  all: ['ledgers'] as const,
  list: (cursor: string | undefined) => ['ledgers', 'list', cursor ?? ''] as const,
  scope: (ledger: string) => ['ledger', ledger] as const
}

export const ledgerQueries = {
  list: (cursor?: string) =>
    queryOptions({
      queryFn: async ({ signal }) => listLedgers(queryParams({ cursor, limit: 100 }), { signal }),
      queryKey: ledgerKeys.list(cursor)
    }),
  detail: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getLedger(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'summary'] as const
    }),
  stats: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getLedgerStats(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'stats'] as const
    }),
  assets: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => listAssets(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'assets'] as const,
      staleTime: 30_000
    }),
  search: (ledger: string, q: string) =>
    queryOptions({
      enabled: q.trim().length > 0,
      queryFn: async ({ signal }) => search(ledger, { q: q.trim() }, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'search', q.trim()] as const
    })
}

export function useCreateLedger() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ name, body }: { name: string; body: CreateLedgerRequest }) => createLedger(name, body),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
  })
}

export function useReplaceLedgerMetadata(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata) => replaceLedgerMetadata(ledger, metadata),
    onSuccess: async () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) }),
        queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
      ])
  })
}
