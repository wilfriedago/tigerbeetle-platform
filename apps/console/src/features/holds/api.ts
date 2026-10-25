import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type CaptureHoldRequest, type CreateHoldRequest, type HoldStatus } from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { api } from '~/lib/api'

export interface HoldListParams {
  status?: HoldStatus | undefined
  address?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const holdQueries = {
  list: (ledger: string, params: HoldListParams) =>
    queryOptions({
      queryFn: async ({ signal }) => api.holds.list(ledger, params, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'list', params] as const,
      // Expiry is derived from the clock, so an open list goes stale on its own.
      refetchInterval: 30_000
    }),
  detail: (ledger: string, id: string) =>
    queryOptions({
      queryFn: async ({ signal }) => api.holds.get(ledger, id, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'detail', id] as const
    })
}

export function useCreateHold(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CreateHoldRequest; idempotencyKey: string }) =>
      api.holds.create(ledger, body, idempotencyKey),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useCaptureHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CaptureHoldRequest; idempotencyKey: string }) =>
      api.holds.capture(ledger, id, body, idempotencyKey),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useVoidHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ idempotencyKey }: { idempotencyKey: string }) => api.holds.void(ledger, id, idempotencyKey),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}
