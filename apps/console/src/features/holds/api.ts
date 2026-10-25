import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  type CaptureHoldRequest,
  type CreateHoldRequest,
  type Hold,
  type HoldList,
  type HoldStatus,
  queryParams
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

export interface HoldListParams {
  status?: HoldStatus | undefined
  address?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const HoldFields = graphql(`
  fragment HoldFields on Hold {
    id
    ledger
    source
    destination
    asset
    amount
    capturedAmount
    status
    timeoutSeconds
    createdAt
    timestamp
    expiresAt
    resolvedAt
    reference
    eventType {
      name
      code
    }
    metadata
    idempotencyKey
    pendingTransferId
    resolutionTransferId
    tbLedger
  }
`)

const HoldsQuery = graphql(`
  query Holds($name: String!, $first: Int, $after: String, $status: HoldStatus, $address: String) {
    ledger(name: $name) {
      holds(first: $first, after: $after, status: $status, address: $address) {
        nodes {
          ...HoldFields
        }
        pageInfo {
          endCursor
        }
      }
      holdCounts {
        open
        captured
        voided
        expired
        reserved {
          asset
          amount
        }
      }
    }
  }
`)

const HoldQuery = graphql(`
  query Hold($name: String!, $id: U128!) {
    ledger(name: $name) {
      hold(id: $id) {
        ...HoldFields
      }
    }
  }
`)

const CreateHoldMutation = graphql(`
  mutation CreateHold($input: CreateHoldInput!) {
    createHold(input: $input) {
      ...HoldFields
    }
  }
`)

const CaptureHoldMutation = graphql(`
  mutation CaptureHold($input: CaptureHoldInput!) {
    captureHold(input: $input) {
      ...HoldFields
    }
  }
`)

const VoidHoldMutation = graphql(`
  mutation VoidHold($input: VoidHoldInput!) {
    voidHold(input: $input) {
      ...HoldFields
    }
  }
`)

export const holdQueries = {
  list: (ledger: string, params: HoldListParams) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<HoldList> => {
        const variables = queryParams({ address: params.address, after: params.cursor, first: params.limit, status: params.status })
        const data = await request(HoldsQuery, { name: ledger, ...variables }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        const { holds, holdCounts } = data.ledger
        return present({ counts: holdCounts, data: holds.nodes, next: holds.pageInfo.endCursor })
      },
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'list', params] as const,
      // Expiry is derived from the clock, so an open list goes stale on its own.
      refetchInterval: 30_000
    }),
  detail: (ledger: string, id: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Hold> => {
        const data = await request(HoldQuery, { id, name: ledger }, { signal })
        if (!data.ledger?.hold) {
          throw notFound(`hold ${id}`)
        }
        return present(data.ledger.hold)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'holds', 'detail', id] as const
    })
}

export function useCreateHold(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CreateHoldRequest; idempotencyKey: string }): Promise<Hold> => {
      const { createHold } = await request(CreateHoldMutation, { input: { ...body, idempotencyKey, ledger } })
      return present(createHold)
    },
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useCaptureHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CaptureHoldRequest; idempotencyKey: string }): Promise<Hold> => {
      const { captureHold } = await request(CaptureHoldMutation, { input: { ...body, id, idempotencyKey, ledger } })
      return present(captureHold)
    },
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}

export function useVoidHold(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ idempotencyKey }: { idempotencyKey: string }): Promise<Hold> => {
      const { voidHold } = await request(VoidHoldMutation, { input: { id, idempotencyKey, ledger } })
      return present(voidHold)
    },
    onSettled: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
  })
}
