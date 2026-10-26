import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { type Member } from '@tigerbeetle-platform/api-client'

import { graphql } from '~/graphql/gen'
import { type MemberFieldsFragment } from '~/graphql/gen/graphql'
import { request } from '~/lib/graphql/client'
import { present } from '~/lib/graphql/present'
import { activeOrganization } from '~/lib/organization'

export const MemberFields = graphql(`
  fragment MemberFields on Member {
    subject
    email
    emailVerified
    name
    status
    role
    addedAt
    addedBy
    ledgers {
      ledger
      role
      grantedAt
      grantedBy
    }
    invitation {
      id
      sentAt
      expiresAt
      expired
    }
  }
`)

const MembersQuery = graphql(`
  query Members {
    members {
      ...MemberFields
    }
  }
`)

const InviteMemberMutation = graphql(`
  mutation InviteMember($input: InviteMemberInput!) {
    inviteMember(input: $input) {
      ...MemberFields
    }
  }
`)

const SetMemberRoleMutation = graphql(`
  mutation SetMemberRole($input: SetMemberRoleInput!) {
    setMemberRole(input: $input)
  }
`)

const RemoveMemberMutation = graphql(`
  mutation RemoveMember($input: MemberInput!) {
    removeMember(input: $input)
  }
`)

const SetLedgerRoleMutation = graphql(`
  mutation SetLedgerRole($input: SetLedgerRoleInput!) {
    setLedgerRole(input: $input)
  }
`)

const RevokeLedgerRoleMutation = graphql(`
  mutation RevokeLedgerRole($input: RevokeLedgerRoleInput!) {
    revokeLedgerRole(input: $input)
  }
`)

const ResendInvitationMutation = graphql(`
  mutation ResendInvitation($input: InvitationInput!) {
    resendInvitation(input: $input)
  }
`)

const DeleteInvitationMutation = graphql(`
  mutation DeleteInvitation($input: InvitationInput!) {
    deleteInvitation(input: $input)
  }
`)

/**
 * A member as the REST types declare it. GraphQL carries the statuses and roles as strings (an
 * enum value cannot hold the dash of setup-pending); the API only ever sends the REST values.
 */
function member(fields: MemberFieldsFragment): Member {
  return present(fields) as Member
}

/** Members are per organization: the key carries it, so switching never shows another's. */
export const memberKeys = {
  all: () => ['members', activeOrganization() ?? ''] as const
}

export const memberQueries = {
  list: () =>
    queryOptions({
      queryFn: async ({ signal }): Promise<Member[]> => {
        const data = await request(MembersQuery, {}, { signal })
        return data.members.map(member)
      },
      queryKey: memberKeys.all(),
      staleTime: 30_000
    })
}

/** The member with the subject, or undefined. */
export function findMember(members: readonly Member[] | undefined, subject: string): Member | undefined {
  return members?.find((m) => m.subject === subject)
}

function useInvalidating<TInput, TResult>(mutationFn: (input: TInput) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: memberKeys.all() }),
        // The caller's own roles may have changed: leaving, or stepping down.
        queryClient.invalidateQueries({ queryKey: ['session'] })
      ])
    }
  })
}

export interface InviteInput {
  email: string
  role: string
  ledgers: { ledger: string; role: string }[]
}

export function useInviteMember() {
  return useInvalidating(async (input: InviteInput): Promise<Member> => {
    const { inviteMember } = await request(InviteMemberMutation, {
      input: { ledgers: input.ledgers, person: { email: input.email }, role: input.role }
    })
    return member(inviteMember)
  })
}

export function useSetMemberRole() {
  return useInvalidating(async (input: { subject: string; role: string }) => {
    await request(SetMemberRoleMutation, { input })
  })
}

export function useRemoveMember() {
  return useInvalidating(async (subject: string) => {
    await request(RemoveMemberMutation, { input: { subject } })
  })
}

export function useSetLedgerRole() {
  return useInvalidating(async (input: { subject: string; ledger: string; role: string }) => {
    await request(SetLedgerRoleMutation, { input })
  })
}

export function useRevokeLedgerRole() {
  return useInvalidating(async (input: { subject: string; ledger: string }) => {
    await request(RevokeLedgerRoleMutation, { input })
  })
}

export function useResendInvitation() {
  return useInvalidating(async (id: string) => {
    await request(ResendInvitationMutation, { input: { id } })
  })
}

export function useDeleteInvitation() {
  return useInvalidating(async (id: string) => {
    await request(DeleteInvitationMutation, { input: { id } })
  })
}
