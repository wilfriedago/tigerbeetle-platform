import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  createEventType,
  type CreateEventTypeRequest,
  deleteRule,
  listEventTypes,
  listRules,
  matchRule,
  putRule,
  type PutRuleRequest
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

export const settingsQueries = {
  rules: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => listRules(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'rules'] as const,
      staleTime: 60_000
    }),
  match: (ledger: string, address: string) =>
    queryOptions({
      enabled: address.length > 0,
      queryFn: async ({ signal }) => matchRule(ledger, { address }, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'rules', 'match', address] as const,
      staleTime: 60_000
    }),
  eventTypes: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }) => listEventTypes(ledger, { signal }),
      queryKey: [...ledgerKeys.scope(ledger), 'event-types'] as const,
      staleTime: 60_000
    })
}

export function usePutRule(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ pattern, body }: { pattern: string; body: PutRuleRequest }) => putRule(ledger, pattern, body),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'rules'] })
  })
}

export function useDeleteRule(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (pattern: string) => deleteRule(ledger, pattern),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'rules'] })
  })
}

export function useCreateEventType(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateEventTypeRequest) => createEventType(ledger, body),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'event-types'] })
  })
}
