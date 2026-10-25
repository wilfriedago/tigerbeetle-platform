import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  type Account,
  type AccountList,
  type AggregateBalances,
  type BalanceHistory,
  type Metadata,
  queryParams,
  type Rule,
  type SegmentList
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

export interface AccountListParams {
  prefix?: string | undefined
  search?: string | undefined
  asset?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const AccountSummaryFields = graphql(`
  fragment AccountSummaryFields on AccountSummary {
    address
    createdAt
    metadata
    balances {
      asset
      accountId
      tbLedger
      balance
      available
      reserved
      incoming
      credits
      debits
      limit
      history
      code
      rule
      createdAt
    }
  }
`)

export const AccountFields = graphql(`
  fragment AccountFields on Account {
    address
    createdAt
    metadata
    balances {
      asset
      accountId
      tbLedger
      balance
      available
      reserved
      incoming
      credits
      debits
      limit
      history
      code
      rule
      createdAt
    }
    openHolds
  }
`)

const AccountsQuery = graphql(`
  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {
    ledger(name: $name) {
      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {
        nodes {
          ...AccountSummaryFields
        }
        pageInfo {
          endCursor
        }
      }
    }
  }
`)

const AccountQuery = graphql(`
  query Account($name: String!, $address: String!) {
    ledger(name: $name) {
      account(address: $address) {
        ...AccountFields
      }
    }
  }
`)

const AccountHistoryQuery = graphql(`
  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {
    ledger(name: $name) {
      accountHistory(address: $address, asset: $asset, limit: $limit) {
        address
        asset
        data {
          timestamp
          at
          balance
          reserved
          credits
          debits
        }
      }
    }
  }
`)

const SegmentsQuery = graphql(`
  query Segments($name: String!, $prefix: String) {
    ledger(name: $name) {
      segments(prefix: $prefix) {
        segment
        prefix
        accounts
        assets
        isAccount
        hasChildren
      }
    }
  }
`)

const AggregateBalancesQuery = graphql(`
  query AggregateBalances($name: String!, $address: String!) {
    ledger(name: $name) {
      aggregateBalances(address: $address) {
        prefix
        asOf
        accounts
        chunks
        assets {
          asset
          accounts
          balance
          available
          reserved
          incoming
        }
      }
    }
  }
`)

const WriteTargetQuery = graphql(`
  query WriteTarget($name: String!, $address: String!) {
    ledger(name: $name) {
      account(address: $address) {
        ...AccountFields
      }
      ruleMatch(address: $address) {
        rule {
          ...RuleFields
        }
      }
    }
  }
`)

const ReplaceAccountMetadataMutation = graphql(`
  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {
    replaceAccountMetadata(input: $input) {
      ...AccountFields
    }
  }
`)

/** What a write is about to touch at one address: its account, when it exists, and the rule that creates its missing accounts. */
export interface WriteTarget {
  account: Account | undefined
  rule: Rule
}

export const accountQueries = {
  list: (ledger: string, params: AccountListParams) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<AccountList> => {
        const variables = queryParams({
          after: params.cursor,
          asset: params.asset,
          first: params.limit,
          prefix: params.prefix,
          search: params.search
        })
        const data = await request(AccountsQuery, { name: ledger, ...variables }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present({ data: data.ledger.accounts.nodes, next: data.ledger.accounts.pageInfo.endCursor })
      },
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'list', params] as const
    }),
  detail: (ledger: string, address: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Account> => {
        const data = await request(AccountQuery, { address, name: ledger }, { signal })
        if (!data.ledger?.account) {
          throw notFound(`account ${address}`)
        }
        return present(data.ledger.account)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'detail', address] as const
    }),
  history: (ledger: string, address: string, asset: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<BalanceHistory> => {
        const data = await request(AccountHistoryQuery, { address, asset, limit: 100, name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present(data.ledger.accountHistory)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'accounts', 'history', address, asset] as const
    }),
  segments: (ledger: string, prefix: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<SegmentList> => {
        const data = await request(SegmentsQuery, { name: ledger, ...queryParams({ prefix }) }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        const trimmed = prefix.replace(/:$/, '')
        return { data: present(data.ledger.segments), ...(trimmed ? { prefix: trimmed } : {}) }
      },
      queryKey: [...ledgerKeys.scope(ledger), 'segments', prefix] as const
    }),
  aggregate: (ledger: string, prefix: string) =>
    queryOptions({
      enabled: prefix.length > 0,
      queryFn: async ({ signal }): Promise<AggregateBalances> => {
        const data = await request(AggregateBalancesQuery, { address: prefix, name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present(data.ledger.aggregateBalances)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'aggregate', prefix] as const,
      // A prefix sum reads every account under the prefix; it is computed on demand only.
      staleTime: 15_000
    }),
  /** The account of an address and the rule for its new accounts, in one request (plan section 17.2). */
  writeTarget: (ledger: string, address: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<WriteTarget> => {
        const data = await request(WriteTargetQuery, { address, name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return { account: data.ledger.account ? present(data.ledger.account) : undefined, rule: present(data.ledger.ruleMatch.rule) }
      },
      queryKey: [...ledgerKeys.writeTargets(ledger), address] as const
    })
}

export function useReplaceAccountMetadata(ledger: string, address: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (metadata: Metadata): Promise<Account> => {
      const { replaceAccountMetadata } = await request(ReplaceAccountMetadataMutation, { input: { address, ledger, metadata } })
      return present(replaceAccountMetadata)
    },
    onSuccess: async () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'accounts'] }),
        queryClient.invalidateQueries({ queryKey: ledgerKeys.writeTargets(ledger) })
      ])
  })
}
