import { queryOptions } from '@tanstack/react-query'

import { type Operation, queryParams, type RequestStatus, type WriteRequestDetail, type WriteRequestList } from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

export interface RequestListParams {
  status?: RequestStatus | undefined
  operation?: Operation | undefined
  search?: string | undefined
  /** Only the writes of this principal: its subject, or an API key's ID. */
  actor?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const WriteRequestFields = graphql(`
  fragment WriteRequestFields on WriteRequest {
    key
    operation
    status
    createdAt
    resolvedAt
    targetId
    problemType
    responseStatus
    requestId
    bodyHash
    attempts
  }
`)

const WriteRequestsQuery = graphql(`
  query WriteRequests($name: String!, $first: Int, $after: String, $status: RequestStatus, $operation: Operation, $search: String, $actor: String) {
    ledger(name: $name) {
      requests(first: $first, after: $after, status: $status, operation: $operation, search: $search, actor: $actor) {
        nodes {
          ...WriteRequestFields
        }
        pageInfo {
          endCursor
        }
      }
    }
  }
`)

const WriteRequestQuery = graphql(`
  query WriteRequest($name: String!, $key: String!) {
    ledger(name: $name) {
      request(key: $key) {
        key
        operation
        status
        createdAt
        resolvedAt
        targetId
        problemType
        responseStatus
        requestId
        bodyHash
        attempts
        request
        response
        transferIds
      }
    }
  }
`)

export const requestQueries = {
  list: (ledger: string, params: RequestListParams) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<WriteRequestList> => {
        const variables = queryParams({
          actor: params.actor,
          after: params.cursor,
          first: params.limit,
          operation: params.operation,
          search: params.search,
          status: params.status
        })
        const data = await request(WriteRequestsQuery, { name: ledger, ...variables }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present({ data: data.ledger.requests.nodes, next: data.ledger.requests.pageInfo.endCursor })
      },
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'list', params] as const,
      // Pending records change state without any action here.
      refetchInterval: 10_000
    }),
  detail: (ledger: string, key: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<WriteRequestDetail> => {
        const data = await request(WriteRequestQuery, { key, name: ledger }, { signal })
        if (!data.ledger?.request) {
          throw notFound(`write request ${key}`)
        }
        // The stored bodies are shown as recorded, nulls included; only the record's own members
        // lose their nulls.
        const { request: body, response, ...record } = data.ledger.request
        return { ...present(record), request: body, ...(response ? { response } : {}) }
      },
      queryKey: [...ledgerKeys.scope(ledger), 'requests', 'detail', key] as const,
      refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2000 : false)
    })
}
