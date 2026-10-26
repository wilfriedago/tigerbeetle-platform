import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type APIKey, type MachineClient } from '@tigerbeetle-platform/api-client'

import { graphql } from '~/graphql/gen'
import { type ApiKeyFieldsFragment, type MachineClientFieldsFragment } from '~/graphql/gen/graphql'
import { request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'
import { activeOrganization } from '~/lib/organization'

// The organization's API keys and machine clients (ADR-0029). GraphQL carries statuses and roles
// as strings; the API only sends the REST values, so the results keep the REST types.

export const ApiKeyFields = graphql(`
  fragment ApiKeyFields on APIKey {
    id
    name
    status
    expiresAt
    lastUsedAt
    revokedAt
    createdAt
    createdBy
    ledgers {
      ledger
      role
      grantedAt
      grantedBy
    }
  }
`)

export const MachineClientFields = graphql(`
  fragment MachineClientFields on MachineClient {
    clientId
    name
    subject
    status
    secretSetAt
    lastTokenAt
    deletedAt
    createdAt
    createdBy
    ledgers {
      ledger
      role
      grantedAt
      grantedBy
    }
  }
`)

const ApiKeysQuery = graphql(`
  query ApiKeys {
    apiKeys {
      ...ApiKeyFields
    }
  }
`)

const MachineClientsQuery = graphql(`
  query MachineClients {
    machineClients {
      ...MachineClientFields
    }
  }
`)

const CreateApiKeyMutation = graphql(`
  mutation CreateApiKey($input: CreateAPIKeyInput!) {
    createApiKey(input: $input) {
      key {
        ...ApiKeyFields
      }
      secret
    }
  }
`)

const RevokeApiKeyMutation = graphql(`
  mutation RevokeApiKey($input: APIKeyInput!) {
    revokeApiKey(input: $input)
  }
`)

const CreateMachineClientMutation = graphql(`
  mutation CreateMachineClient($input: CreateMachineClientInput!) {
    createMachineClient(input: $input) {
      client {
        ...MachineClientFields
      }
      secret
      tokenUrl
    }
  }
`)

const RegenerateSecretMutation = graphql(`
  mutation RegenerateMachineClientSecret($input: MachineClientInput!) {
    regenerateMachineClientSecret(input: $input) {
      secret
    }
  }
`)

const DeleteMachineClientMutation = graphql(`
  mutation DeleteMachineClient($input: MachineClientInput!) {
    deleteMachineClient(input: $input)
  }
`)

const SetCredentialRoleMutation = graphql(`
  mutation SetCredentialRole($input: SetCredentialRoleInput!) {
    setCredentialRole(input: $input)
  }
`)

const RevokeCredentialRoleMutation = graphql(`
  mutation RevokeCredentialRole($input: RevokeCredentialRoleInput!) {
    revokeCredentialRole(input: $input)
  }
`)

function apiKey(fields: ApiKeyFieldsFragment): APIKey {
  return present(fields) as APIKey
}

function machineClient(fields: MachineClientFieldsFragment): MachineClient {
  return present(fields) as MachineClient
}

export const credentialKeys = {
  all: () => ['credentials', activeOrganization() ?? ''] as const,
  apiKeys: () => ['credentials', activeOrganization() ?? '', 'api-keys'] as const,
  machineClients: () => ['credentials', activeOrganization() ?? '', 'machine-clients'] as const
}

export const credentialQueries = {
  apiKeys: () =>
    queryOptions({
      queryFn: async ({ signal }): Promise<APIKey[]> => {
        const data = await request(ApiKeysQuery, {}, { signal })
        return data.apiKeys.map(apiKey)
      },
      queryKey: credentialKeys.apiKeys(),
      staleTime: 30_000
    }),
  machineClients: () =>
    queryOptions({
      queryFn: async ({ signal }): Promise<MachineClient[]> => {
        const data = await request(MachineClientsQuery, {}, { signal })
        return data.machineClients.map(machineClient)
      },
      queryKey: credentialKeys.machineClients(),
      staleTime: 30_000
    })
}

function useCredentialMutation<TInput, TResult>(mutationFn: (input: TInput) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({ mutationFn, onSuccess: async () => queryClient.invalidateQueries({ queryKey: credentialKeys.all() }) })
}

export interface LedgerRoleInput {
  ledger: string
  role: string
}

export function useCreateApiKey() {
  return useCredentialMutation(async (input: { name: string; expiresAt?: string; ledgers: LedgerRoleInput[] }) => {
    const { createApiKey } = await request(CreateApiKeyMutation, { input })
    return { key: apiKey(createApiKey.key), secret: createApiKey.secret }
  })
}

export function useRevokeApiKey() {
  return useCredentialMutation(async (id: string) => {
    await request(RevokeApiKeyMutation, { input: { id } })
  })
}

export function useCreateMachineClient() {
  return useCredentialMutation(async (input: { name: string; ledgers: LedgerRoleInput[] }) => {
    const { createMachineClient } = await request(CreateMachineClientMutation, { input })
    return { client: machineClient(createMachineClient.client), secret: createMachineClient.secret, tokenUrl: createMachineClient.tokenUrl }
  })
}

export function useRegenerateSecret() {
  return useCredentialMutation(async (clientId: string) => {
    const { regenerateMachineClientSecret } = await request(RegenerateSecretMutation, { input: { clientId } })
    return regenerateMachineClientSecret.secret
  })
}

export function useDeleteMachineClient() {
  return useCredentialMutation(async (clientId: string) => {
    await request(DeleteMachineClientMutation, { input: { clientId } })
  })
}

export type CredentialKind = 'api-key' | 'machine-client'

export function useSetCredentialRole() {
  return useCredentialMutation(async (input: { kind: CredentialKind; id: string; ledger: string; role: string }) => {
    await request(SetCredentialRoleMutation, { input })
  })
}

export function useRevokeCredentialRole() {
  return useCredentialMutation(async (input: { kind: CredentialKind; id: string; ledger: string }) => {
    await request(RevokeCredentialRoleMutation, { input })
  })
}
