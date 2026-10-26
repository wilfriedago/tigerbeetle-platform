import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type Organization, type OrganizationList, queryParams } from '@tigerbeetle-platform/api-client'

import { graphql } from '~/graphql/gen'
import { type OrganizationFieldsFragment } from '~/graphql/gen/graphql'
import { request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'

// The staff's organizations (ADR-0027). These calls act for no organization: the staff routes
// clear X-Organization before they load, since staff are members of none.

export const OrganizationFields = graphql(`
  fragment OrganizationFields on Organization {
    id
    keycloakId
    alias
    name
    status
    suspensionReason
    createdAt
    createdBy
    owners {
      subject
      email
      name
      status
      since
    }
    members
    ledgers
  }
`)

const OrganizationsQuery = graphql(`
  query Organizations($first: Int, $after: String) {
    organizations(first: $first, after: $after) {
      nodes {
        ...OrganizationFields
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
`)

const OrganizationQuery = graphql(`
  query Organization($alias: String!) {
    organization(alias: $alias) {
      ...OrganizationFields
    }
  }
`)

const CreateOrganizationMutation = graphql(`
  mutation CreateOrganization($input: CreateOrganizationInput!) {
    createOrganization(input: $input) {
      ...OrganizationFields
    }
  }
`)

const UpdateOrganizationMutation = graphql(`
  mutation UpdateOrganization($input: UpdateOrganizationInput!) {
    updateOrganization(input: $input) {
      ...OrganizationFields
    }
  }
`)

/**
 * An organization as the REST types declare it; GraphQL carries its statuses as strings, and the
 * API only ever sends the REST values.
 */
function organization(fields: OrganizationFieldsFragment): Organization {
  return present(fields) as Organization
}

export const staffKeys = {
  all: ['staff', 'organizations'] as const,
  list: (cursor: string | undefined) => ['staff', 'organizations', 'list', cursor ?? ''] as const,
  one: (alias: string) => ['staff', 'organizations', 'one', alias] as const
}

export const staffQueries = {
  list: (cursor?: string) =>
    queryOptions({
      placeholderData: keepPreviousData,
      queryFn: async ({ signal }): Promise<OrganizationList> => {
        const { organizations } = await request(OrganizationsQuery, queryParams({ after: cursor, first: 50 }), { signal })
        return {
          data: organizations.nodes.map(organization),
          ...(organizations.pageInfo.hasNextPage && organizations.pageInfo.endCursor ? { next: organizations.pageInfo.endCursor } : {})
        }
      },
      queryKey: staffKeys.list(cursor),
      staleTime: 30_000
    }),
  one: (alias: string) =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Organization | null> => {
        const data = await request(OrganizationQuery, { alias }, { signal })
        return data.organization ? organization(data.organization) : null
      },
      queryKey: staffKeys.one(alias),
      staleTime: 30_000
    })
}

export interface NewOrganization {
  alias: string
  name: string
  ownerEmail: string
}

export function useCreateOrganization() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: NewOrganization): Promise<Organization> => {
      const { createOrganization } = await request(CreateOrganizationMutation, {
        input: { alias: input.alias, name: input.name, owner: { email: input.ownerEmail } }
      })
      return organization(createOrganization)
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: staffKeys.all })
  })
}

export function useUpdateOrganization() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: { alias: string; name?: string; status?: 'active' | 'suspended'; reason?: string }): Promise<Organization> => {
      const { updateOrganization } = await request(UpdateOrganizationMutation, { input })
      return organization(updateOrganization)
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: staffKeys.all })
  })
}
