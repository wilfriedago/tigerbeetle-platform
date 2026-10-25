import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  captureHold,
  type CaptureHoldRequest,
  createHold,
  type CreateHoldRequest,
  getHold,
  type HoldStatus,
  listHolds,
  queryParams,
  voidHold
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

export interface HoldListParams {
  status?: HoldStatus | undefined
  address?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const holdQueries = {
  list: (ledger: string, params: HoldListParams) =>
    queryOptions({
      queryFn: async ({ signal }) => listHolds(ledger, queryParams(params), { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'list', params] as const,
      // Expiry is derived from the clock, so an open list goes stale on its own.
      refetchInterval: 30_000
    }),
  detail: (ledger: string, id: string) =>
    queryOptions({
      queryFn: async ({ signal }) => getHold(ledger, id, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'detail', id] as const
    })
}

export function useCreateHold(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CreateHoldRequest; idempotencyKey: string }) =>
      createHold(ledger, body, { 'Idempotency-Key': idempotencyKey }),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useCaptureHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CaptureHoldRequest; idempotencyKey: string }) =>
      captureHold(ledger, id, { 'Idempotency-Key': idempotencyKey }, body),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useVoidHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ idempotencyKey }: { idempotencyKey: string }) => voidHold(ledger, id, { 'Idempotency-Key': idempotencyKey }),
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}
