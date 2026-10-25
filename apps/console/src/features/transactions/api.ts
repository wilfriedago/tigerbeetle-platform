import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type CreateTransactionRequest, type Metadata, type RevertTransactionRequest } from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { api } from '~/lib/api'

export interface TransactionListParams {
  address?: string | undefined
  asset?: string | undefined
  reference?: string | undefined
  eventType?: string | undefined
  after?: string | undefined
  before?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const transactionQueries = {
  list: (ledger: string, params: TransactionListParams) =>
    queryOptions({
      queryFn: async ({ signal }) => api.transactions.list(ledger, params, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'list', params] as const
    }),
  detail: (ledger: string, id: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.transactions.get(ledger, id, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'detail', id] as const,
      // A committed transaction never changes, apart from its metadata and revertedBy.
      staleTime: 30_000
    })
}

/** Every write that moves money touches balances everywhere in the ledger. */
async function refreshLedger(queryClient: ReturnType<typeof useQueryClient>, ledger: string) {
  await queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
}

export function useCreateTransaction(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CreateTransactionRequest; idempotencyKey: string }) =>
      api.transactions.create(ledger, body, idempotencyKey),
    // Declines are recorded too (Requests, overview counts), so refresh either way.
    onSettled: async () => refreshLedger(queryClient, ledger)
  })
}

export function useRevertTransaction(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: RevertTransactionRequest; idempotencyKey: string }) =>
      api.transactions.revert(ledger, id, body, idempotencyKey),
    onSettled: async () => refreshLedger(queryClient, ledger)
  })
}

export function useReplaceTransactionMetadata(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata) => api.transactions.replaceMetadata(ledger, id, metadata),
    onSuccess: async (transaction) => {
      queryClient.setQueryData(transactionQueries.detail(ledger, id).queryKey, transaction)
      await queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'list'] })
    }
  })
}
