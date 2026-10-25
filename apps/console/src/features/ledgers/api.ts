import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  type AssetList,
  type CreateLedgerRequest,
  type Ledger,
  type LedgerList,
  type LedgerStats,
  type Metadata,
  queryParams,
  type SearchResults
} from '@tigerbeetle-platform/api-client'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

/**
 * Query keys. Everything inside a ledger starts with ["ledger", name], so a write can refresh
 * every view of that ledger with one invalidation.
 */
export const ledgerKeys = {
  all: ['ledgers'] as const,
  list: (cursor: string | undefined) => ['ledgers', 'list', cursor ?? ''] as const,
  scope: (ledger: string) => ['ledger', ledger] as const
}

export const LedgerFields = graphql(`
  fragment LedgerFields on Ledger {
    name
    metadata
    createdAt
    assetCodes
    accountCount
  }
`)

const LedgersQuery = graphql(`
  query Ledgers($first: Int, $after: String) {
    ledgers(first: $first, after: $after) {
      nodes {
        ...LedgerFields
      }
      pageInfo {
        endCursor
      }
    }
  }
`)

const LedgerQuery = graphql(`
  query Ledger($name: String!) {
    ledger(name: $name) {
      ...LedgerFields
    }
  }
`)

const LedgerStatsQuery = graphql(`
  query LedgerStats($name: String!) {
    ledger(name: $name) {
      stats {
        accounts
        accountsToday
        assets {
          ...AssetSummaryFields
        }
        holds {
          open
          captured
          voided
          expired
          reserved {
            asset
            amount
          }
        }
        declined {
          total
          reasons {
            reason
            count
          }
        }
      }
    }
  }
`)

export const AssetSummaryFields = graphql(`
  fragment AssetSummaryFields on AssetSummary {
    asset
    tbLedger
    createdAt
    accounts
    issued
  }
`)

const LedgerAssetsQuery = graphql(`
  query LedgerAssets($name: String!) {
    ledger(name: $name) {
      assets {
        ...AssetSummaryFields
      }
    }
  }
`)

const SearchQuery = graphql(`
  query Search($name: String!, $q: String!) {
    ledger(name: $name) {
      search(q: $q) {
        query
        accounts {
          address
          assets
        }
        transactions {
          id
          reference
          committedAt
        }
        holds {
          id
          status
        }
        requests {
          key
          operation
          status
        }
      }
    }
  }
`)

const CreateLedgerMutation = graphql(`
  mutation CreateLedger($input: CreateLedgerInput!) {
    createLedger(input: $input) {
      ...LedgerFields
    }
  }
`)

const ReplaceLedgerMetadataMutation = graphql(`
  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {
    replaceLedgerMetadata(input: $input) {
      ...LedgerFields
    }
  }
`)

interface LedgerFieldsShape {
  name: string
  metadata: Record<string, string>
  createdAt: string
  assetCodes: string[]
  accountCount: number
}

/** The GraphQL ledger in the REST shape the pages take: assets and accounts by their REST names. */
function ledgerOf(l: LedgerFieldsShape): Ledger {
  return { accounts: l.accountCount, assets: l.assetCodes, createdAt: l.createdAt, metadata: l.metadata, name: l.name }
}

export const ledgerQueries = {
  list: (cursor?: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<LedgerList> => {
        const { ledgers } = await request(LedgersQuery, queryParams({ after: cursor, first: 100 }), { signal })
        return present({ data: ledgers.nodes.map(ledgerOf), next: ledgers.pageInfo.endCursor })
      },
      queryKey: ledgerKeys.list(cursor)
    }),
  detail: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Ledger> => {
        const data = await request(LedgerQuery, { name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return ledgerOf(data.ledger)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'summary'] as const
    }),
  stats: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<LedgerStats> => {
        const data = await request(LedgerStatsQuery, { name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present(data.ledger.stats)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'stats'] as const
    }),
  assets: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<AssetList> => {
        const data = await request(LedgerAssetsQuery, { name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return { data: present(data.ledger.assets) }
      },
      queryKey: [...ledgerKeys.scope(ledger), 'assets'] as const,
      staleTime: 30_000
    }),
  search: (ledger: string, q: string) =>
    queryOptions({
      enabled: q.trim().length > 0,
      queryFn: async ({ signal }): Promise<SearchResults> => {
        const data = await request(SearchQuery, { name: ledger, q: q.trim() }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present(data.ledger.search)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'search', q.trim()] as const
    })
}

export function useCreateLedger() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ name, body }: { name: string; body: CreateLedgerRequest }): Promise<Ledger> => {
      const { createLedger } = await request(CreateLedgerMutation, { input: { name, ...(body.metadata ? { metadata: body.metadata } : {}) } })
      return ledgerOf(createLedger)
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
  })
}

export function useReplaceLedgerMetadata(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata): Promise<Ledger> => {
      const { replaceLedgerMetadata } = await request(ReplaceLedgerMetadataMutation, { input: { ledger, metadata } })
      return ledgerOf(replaceLedgerMetadata)
    },
    onSuccess: async () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ledgerKeys.scope(ledger) }),
        queryClient.invalidateQueries({ queryKey: ledgerKeys.all })
      ])
  })
}
