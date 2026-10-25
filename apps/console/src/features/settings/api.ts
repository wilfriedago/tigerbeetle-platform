import { type QueryClient, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import {
  type CreateEventTypeRequest,
  type EventType,
  type EventTypeList,
  type PutRuleRequest,
  type Rule,
  type RuleList,
  type RuleMatch
} from '@tigerbeetle-platform/api-client'

import { ledgerKeys } from '~/features/ledgers/api'

import { graphql } from '~/graphql/gen'
import { notFound, request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

export const RuleFields = graphql(`
  fragment RuleFields on Rule {
    pattern
    limit
    history
    code
    builtIn
    segments
    createdAt
    updatedAt
  }
`)

export const EventTypeFields = graphql(`
  fragment EventTypeFields on EventType {
    name
    code
    builtIn
    createdAt
    transactions
  }
`)

const RulesQuery = graphql(`
  query Rules($name: String!) {
    ledger(name: $name) {
      rules {
        ...RuleFields
      }
    }
  }
`)

const RuleMatchQuery = graphql(`
  query RuleMatch($name: String!, $address: String!) {
    ledger(name: $name) {
      ruleMatch(address: $address) {
        address
        rule {
          ...RuleFields
        }
        candidates {
          matches
          rule {
            ...RuleFields
          }
        }
        existing
      }
    }
  }
`)

const EventTypesQuery = graphql(`
  query EventTypes($name: String!) {
    ledger(name: $name) {
      eventTypes {
        ...EventTypeFields
      }
    }
  }
`)

const PutRuleMutation = graphql(`
  mutation PutRule($input: PutRuleInput!) {
    putRule(input: $input) {
      ...RuleFields
    }
  }
`)

const DeleteRuleMutation = graphql(`
  mutation DeleteRule($input: DeleteRuleInput!) {
    deleteRule(input: $input)
  }
`)

const CreateEventTypeMutation = graphql(`
  mutation CreateEventType($input: CreateEventTypeInput!) {
    createEventType(input: $input) {
      ...EventTypeFields
    }
  }
`)

export const settingsQueries = {
  rules: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<RuleList> => {
        const data = await request(RulesQuery, { name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return { data: present(data.ledger.rules) }
      },
      queryKey: [...ledgerKeys.scope(ledger), 'rules'] as const,
      staleTime: 60_000
    }),
  match: (ledger: string, address: string) =>
    queryOptions({
      enabled: address.length > 0,
      queryFn: async ({ signal }): Promise<RuleMatch> => {
        const data = await request(RuleMatchQuery, { address, name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return present(data.ledger.ruleMatch)
      },
      queryKey: [...ledgerKeys.scope(ledger), 'rules', 'match', address] as const,
      staleTime: 60_000
    }),
  eventTypes: (ledger: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<EventTypeList> => {
        const data = await request(EventTypesQuery, { name: ledger }, { signal })
        if (!data.ledger) {
          throw notFound(`ledger ${ledger}`)
        }
        return { data: present(data.ledger.eventTypes) }
      },
      queryKey: [...ledgerKeys.scope(ledger), 'event-types'] as const,
      staleTime: 60_000
    })
}

async function invalidateRules(queryClient: QueryClient, ledger: string) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'rules'] }),
    queryClient.invalidateQueries({ queryKey: ledgerKeys.writeTargets(ledger) })
  ])
}

export function usePutRule(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ pattern, body }: { pattern: string; body: PutRuleRequest }): Promise<Rule> => {
      const { putRule } = await request(PutRuleMutation, {
        input: { code: body.code, ledger, limit: body.limit, pattern, ...(body.history === undefined ? {} : { history: body.history }) }
      })
      return present(putRule)
    },
    onSuccess: async () => invalidateRules(queryClient, ledger)
  })
}

export function useDeleteRule(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (pattern: string) => {
      await request(DeleteRuleMutation, { input: { ledger, pattern } })
    },
    onSuccess: async () => invalidateRules(queryClient, ledger)
  })
}

export function useCreateEventType(ledger: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateEventTypeRequest): Promise<EventType> => {
      const { createEventType } = await request(CreateEventTypeMutation, {
        input: { ledger, name: body.name, ...(body.code === undefined ? {} : { code: body.code }) }
      })
      return present(createEventType)
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: [...ledgerKeys.scope(ledger), 'event-types'] })
  })
}
