import { type QueryClient, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  type CreateTransactionRequest,
  type Metadata,
  queryParams,
  type RevertTransactionRequest,
  type Transaction,
  type TransactionList
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

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

export const TransactionFields = graphql(`
  fragment TransactionFields on Transaction {
    id
    ledger
    postings {
      source
      destination
      asset
      amount
      transferId
      tbLedger
    }
    reference
    eventType {
      name
      code
    }
    businessTime
    metadata
    committedAt
    timestamp
    reverts
    revertedBy
    idempotencyKey
    requestId
  }
`)

const TransactionsQuery = graphql(`
  query Transactions(
    $name: String!
    $first: Int
    $after: String
    $address: String
    $asset: String
    $reference: String
    $eventType: String
    $committedAfter: DateTime
    $committedBefore: DateTime
  ) {
    ledger(name: $name) {
      transactions(
        first: $first
        after: $after
        address: $address
        asset: $asset
        reference: $reference
        eventType: $eventType
        committedAfter: $committedAfter
        committedBefore: $committedBefore
      ) {
        nodes {
          ...TransactionFields
        }
        pageInfo {
          endCursor
        }
      }
    }
  }
`)

const TransactionQuery = graphql(`
  query Transaction($name: String!, $id: U128!) {
    ledger(name: $name) {
      transaction(id: $id) {
        ...TransactionFields
      }
    }
  }
`)

const CreateTransactionMutation = graphql(`
  mutation CreateTransaction($input: CreateTransactionInput!) {
    createTransaction(input: $input) {
      ...TransactionFields
    }
  }
`)

const RevertTransactionMutation = graphql(`
  mutation RevertTransaction($input: RevertTransactionInput!) {
    revertTransaction(input: $input) {
      ...TransactionFields
    }
  }
`)

const ReplaceTransactionMetadataMutation = graphql(`
  mutation ReplaceTransactionMetadata($input: ReplaceTransactionMetadataInput!) {
    replaceTransactionMetadata(input: $input) {
      ...TransactionFields
    }
  }
`)

export const transactionQueries = {
  list: (ledger: string, params: TransactionListParams) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<TransactionList> => {
        const variables = queryParams({
          address: params.address,
          after: params.cursor,
          asset: params.asset,
          committedAfter: params.after,
          committedBefore: params.before,
          eventType: params.eventType,
          first: params.limit,
          reference: params.reference
        })
        const data = await request(TransactionsQuery, { name: ledger, ...variables }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present({ data: data.ledger.transactions.nodes, next: data.ledger.transactions.pageInfo.endCursor })
      },
      queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'list', params] as const
    }),
  detail: (ledger: string, id: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Transaction> => {
        const data = await request(TransactionQuery, { id, name: ledger }, { signal })
        if (!data.ledger?.transaction) {
          throw notFound(`transaction ${id}`)
        }
        return present(data.ledger.transaction)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'detail', id] as const,
      // A committed transaction never changes, apart from its metadata and revertedBy.
      staleTime: 30_000
    })
}

/** Every write that moves money touches balances everywhere in the ledger. */
async function refreshLedger(queryClient: QueryClient, ledger: string) {
  await queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) })
}

export function useCreateTransaction(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: CreateTransactionRequest; idempotencyKey: string }): Promise<Transaction> => {
      const { createTransaction } = await request(CreateTransactionMutation, { input: { ...body, idempotencyKey, ledger } })
      return present(createTransaction)
    },
    // Declines are recorded too (Requests, overview counts), so refresh either way.
    onSettled: async () => refreshLedger(queryClient, ledger)
  })
}

export function useRevertTransaction(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: { body: RevertTransactionRequest; idempotencyKey: string }): Promise<Transaction> => {
      const { revertTransaction } = await request(RevertTransactionMutation, { input: { ...body, id, idempotencyKey, ledger } })
      return present(revertTransaction)
    },
    onSettled: async () => refreshLedger(queryClient, ledger)
  })
}

export function useReplaceTransactionMetadata(ledger: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata): Promise<Transaction> => {
      const { replaceTransactionMetadata } = await request(ReplaceTransactionMetadataMutation, { input: { id, ledger, metadata } })
      return present(replaceTransactionMetadata)
    },
    onSuccess: async (transaction) => {
      queryClient.setQueryData(transactionQueries.detail(ledger, id).queryKey, transaction)
      await queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'transactions', 'list'] })
    }
  })
}
