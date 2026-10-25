import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type CreateLedgerRequest, type Metadata } from '@tigerbeetle-platform/api-client'

import { api } from '~/lib/api'

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
      queryFn: async ({ signal }) => api.ledgers.list({ cursor, limit: 100 }, { signal }),
      queryKey: ledgerKeys.list(cursor)
    }),
  detail: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.ledgers.get(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'summary'] as const
    }),
  stats: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.ledgers.stats(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'stats'] as const
    }),
  assets: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.ledgers.assets(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'assets'] as const,
      staleTime: 30_000
    }),
  search: (ledger: string, q: string) =>
    queryOptions({
      enabled: q.trim().length > 0,
      queryFn: async ({ signal }) => api.ledgers.search(ledger, q.trim(), { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'search', q.trim()] as const
    })
}

export function useCreateLedger() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ name, body }: { name: string; body: CreateLedgerRequest }) => api.ledgers.create(name, body),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
  })
}

export function useReplaceLedgerMetadata(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata) => api.ledgers.replaceMetadata(ledger, metadata),
    onSuccess: async () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) }),
        queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
      ])
  })
}
