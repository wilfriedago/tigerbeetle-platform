/* eslint-disable */
/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
import type { DocumentTypeDecoration } from '@graphql-typed-document-node/core';
export type ApiKeyInput = {
  id: string;
};

/** The balance limit flag of the TigerBeetle accounts a rule creates. */
export type BalanceLimit =
  | 'credits_must_not_exceed_debits'
  | 'debits_must_not_exceed_credits'
  | 'none';

export type CaptureHoldInput = {
  /** The amount to capture; the full hold when absent. Must not be zero. */
  amount?: string | null | undefined;
  id: string;
  idempotencyKey: string;
  ledger: string;
};

export type CreateApiKeyInput = {
  /** Absent for a key that does not expire. */
  expiresAt?: string | null | undefined;
  /** The ledger roles the key acts with; at least one. */
  ledgers: Array<LedgerRoleInput>;
  /** Lowercase letters, digits and dashes. */
  name: string;
};

export type CreateEventTypeInput = {
  /** 2 to 65535; the next free code when absent. */
  code?: number | null | undefined;
  ledger: string;
  /** Lowercase, starting with a letter, such as card.authorized. */
  name: string;
};

export type CreateHoldInput = {
  amount: string;
  asset: string;
  destination: string;
  eventType?: string | null | undefined;
  idempotencyKey: string;
  ledger: string;
  metadata?: Record<string, string> | null | undefined;
  reference?: string | null | undefined;
  source: string;
  /** Releases the hold after this many seconds on the cluster clock; 0 or absent means never. */
  timeoutSeconds?: number | null | undefined;
};

export type CreateLedgerInput = {
  metadata?: Record<string, string> | null | undefined;
  /** Lowercase letters, digits, dash and underscore, 1 to 63 characters. */
  name: string;
};

export type CreateMachineClientInput = {
  /** The ledger roles its tokens act with; at least one. */
  ledgers: Array<LedgerRoleInput>;
  /** The client ID becomes <organization>-<name>. */
  name: string;
};

export type CreateOrganizationInput = {
  /** Lowercase letters, digits and dashes, 1 to 63 characters. It cannot change. */
  alias: string;
  name: string;
  owner: NewMemberInput;
};

export type CreateTransactionInput = {
  /** The client's business time, stored in user_data_64. Not the committed time. */
  businessTime?: string | null | undefined;
  /** An event type name of the ledger; the default event type when absent. */
  eventType?: string | null | undefined;
  /** Makes the write safe to retry: the same key and input replay the first answer (ADR-0007). */
  idempotencyKey: string;
  ledger: string;
  metadata?: Record<string, string> | null | undefined;
  /** 1 to 64 postings, applied atomically. */
  postings: Array<PostingInput>;
  reference?: string | null | undefined;
};

export type DeleteRuleInput = {
  ledger: string;
  pattern: string;
};

/** The life cycle of a hold. expired is derived from the cluster clock. */
export type HoldStatus =
  | 'captured'
  | 'expired'
  | 'open'
  | 'voided';

export type InvitationInput = {
  id: string;
};

export type InviteMemberInput = {
  /** Roles on single ledgers, granted with the organization role. */
  ledgers: Array<LedgerRoleInput>;
  person: NewMemberInput;
  /** member, admin or owner; member when absent. Only an owner can invite an owner. */
  role?: string | null | undefined;
};

export type LedgerRoleInput = {
  ledger: string;
  /** viewer, operator or admin. */
  role: string;
};

export type MachineClientInput = {
  clientId: string;
};

export type MemberInput = {
  subject: string;
};

export type NewMemberInput = {
  email: string;
  /** Used when the person has no account yet. */
  firstName?: string | null | undefined;
  /** Used when the person has no account yet. */
  lastName?: string | null | undefined;
};

/** The kind of write an idempotency record holds. */
export type Operation =
  | 'capture'
  | 'hold'
  | 'revert'
  | 'transaction'
  | 'void';

export type PostingInput = {
  amount: string;
  asset: string;
  /** The credited account. */
  destination: string;
  /** The debited account. */
  source: string;
};

export type PutRuleInput = {
  /** The TigerBeetle account code, 1 to 65535. */
  code: number;
  history?: boolean | null | undefined;
  ledger: string;
  limit: BalanceLimit;
  /** *, an address such as world, or a prefix ending with a colon such as users:. */
  pattern: string;
};

export type ReplaceAccountMetadataInput = {
  address: string;
  ledger: string;
  metadata: Record<string, string>;
};

export type ReplaceLedgerMetadataInput = {
  ledger: string;
  metadata: Record<string, string>;
};

export type ReplaceTransactionMetadataInput = {
  id: string;
  ledger: string;
  metadata: Record<string, string>;
};

/** The state of an idempotency record. */
export type RequestStatus =
  | 'committed'
  | 'declined'
  | 'failed'
  | 'pending';

export type RevertTransactionInput = {
  id: string;
  idempotencyKey: string;
  ledger: string;
  metadata?: Record<string, string> | null | undefined;
  reference?: string | null | undefined;
};

export type RevokeCredentialRoleInput = {
  id: string;
  /** api-key or machine-client. */
  kind: string;
  ledger: string;
};

export type RevokeLedgerRoleInput = {
  ledger: string;
  subject: string;
};

export type SetCredentialRoleInput = {
  /** The key ID or the client ID. */
  id: string;
  /** api-key or machine-client. */
  kind: string;
  ledger: string;
  /** viewer, operator or admin. */
  role: string;
};

export type SetLedgerRoleInput = {
  ledger: string;
  /** viewer, operator or admin. */
  role: string;
  subject: string;
};

export type SetMemberRoleInput = {
  /** member, admin or owner. */
  role: string;
  subject: string;
};

export type UpdateOrganizationInput = {
  alias: string;
  name?: string | null | undefined;
  /** Why the organization is suspended, shown to its members. */
  reason?: string | null | undefined;
  /** active or suspended. */
  status?: string | null | undefined;
};

export type VoidHoldInput = {
  id: string;
  idempotencyKey: string;
  ledger: string;
};

export type AccountSummaryFieldsFragment = { address: string, createdAt: string, metadata: Record<string, string>, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> };

export type AccountFieldsFragment = { address: string, createdAt: string, metadata: Record<string, string>, openHolds: number, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> };

export type AccountsQueryVariables = Exact<{
  name: string;
  first?: number | null | undefined;
  after?: string | null | undefined;
  prefix?: string | null | undefined;
  search?: string | null | undefined;
  asset?: string | null | undefined;
}>;


export type AccountsQuery = { ledger: { accounts: { nodes: Array<{ address: string, createdAt: string, metadata: Record<string, string>, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> }>, pageInfo: { endCursor: string | null } } } | null };

export type AccountQueryVariables = Exact<{
  name: string;
  address: string;
}>;


export type AccountQuery = { ledger: { account: { address: string, createdAt: string, metadata: Record<string, string>, openHolds: number, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> } | null } | null };

export type AccountHistoryQueryVariables = Exact<{
  name: string;
  address: string;
  asset: string;
  limit?: number | null | undefined;
}>;


export type AccountHistoryQuery = { ledger: { accountHistory: { address: string, asset: string, data: Array<{ timestamp: string, at: string, balance: string, reserved: string, credits: string, debits: string }> } } | null };

export type SegmentsQueryVariables = Exact<{
  name: string;
  prefix?: string | null | undefined;
}>;


export type SegmentsQuery = { ledger: { segments: Array<{ segment: string, prefix: string, accounts: number, assets: Array<string>, isAccount: boolean, hasChildren: boolean }> } | null };

export type AggregateBalancesQueryVariables = Exact<{
  name: string;
  address: string;
}>;


export type AggregateBalancesQuery = { ledger: { aggregateBalances: { prefix: string, asOf: string, accounts: number, chunks: number, assets: Array<{ asset: string, accounts: number, balance: string, available: string, reserved: string, incoming: string }> } } | null };

export type WriteTargetQueryVariables = Exact<{
  name: string;
  address: string;
}>;


export type WriteTargetQuery = { ledger: { account: { address: string, createdAt: string, metadata: Record<string, string>, openHolds: number, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> } | null, ruleMatch: { rule: { pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null } } } | null };

export type ReplaceAccountMetadataMutationVariables = Exact<{
  input: ReplaceAccountMetadataInput;
}>;


export type ReplaceAccountMetadataMutation = { replaceAccountMetadata: { address: string, createdAt: string, metadata: Record<string, string>, openHolds: number, balances: Array<{ asset: string, accountId: string, tbLedger: number, balance: string, available: string, reserved: string, incoming: string, credits: string, debits: string, limit: BalanceLimit, history: boolean, code: number, rule: string, createdAt: string }> } };

export type ApiKeyFieldsFragment = { id: string, name: string, status: string, expiresAt: string | null, lastUsedAt: string | null, revokedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> };

export type MachineClientFieldsFragment = { clientId: string, name: string, subject: string, status: string, secretSetAt: string, lastTokenAt: string | null, deletedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> };

export type ApiKeysQueryVariables = Exact<{ [key: string]: never; }>;


export type ApiKeysQuery = { apiKeys: Array<{ id: string, name: string, status: string, expiresAt: string | null, lastUsedAt: string | null, revokedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> }> };

export type MachineClientsQueryVariables = Exact<{ [key: string]: never; }>;


export type MachineClientsQuery = { machineClients: Array<{ clientId: string, name: string, subject: string, status: string, secretSetAt: string, lastTokenAt: string | null, deletedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> }> };

export type CreateApiKeyMutationVariables = Exact<{
  input: CreateApiKeyInput;
}>;


export type CreateApiKeyMutation = { createApiKey: { secret: string, key: { id: string, name: string, status: string, expiresAt: string | null, lastUsedAt: string | null, revokedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> } } };

export type RevokeApiKeyMutationVariables = Exact<{
  input: ApiKeyInput;
}>;


export type RevokeApiKeyMutation = { revokeApiKey: boolean };

export type CreateMachineClientMutationVariables = Exact<{
  input: CreateMachineClientInput;
}>;


export type CreateMachineClientMutation = { createMachineClient: { secret: string, tokenUrl: string, client: { clientId: string, name: string, subject: string, status: string, secretSetAt: string, lastTokenAt: string | null, deletedAt: string | null, createdAt: string, createdBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }> } } };

export type RegenerateMachineClientSecretMutationVariables = Exact<{
  input: MachineClientInput;
}>;


export type RegenerateMachineClientSecretMutation = { regenerateMachineClientSecret: { secret: string } };

export type DeleteMachineClientMutationVariables = Exact<{
  input: MachineClientInput;
}>;


export type DeleteMachineClientMutation = { deleteMachineClient: boolean };

export type SetCredentialRoleMutationVariables = Exact<{
  input: SetCredentialRoleInput;
}>;


export type SetCredentialRoleMutation = { setCredentialRole: boolean };

export type RevokeCredentialRoleMutationVariables = Exact<{
  input: RevokeCredentialRoleInput;
}>;


export type RevokeCredentialRoleMutation = { revokeCredentialRole: boolean };

export type HoldFieldsFragment = { id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } };

export type HoldsQueryVariables = Exact<{
  name: string;
  first?: number | null | undefined;
  after?: string | null | undefined;
  status?: HoldStatus | null | undefined;
  address?: string | null | undefined;
}>;


export type HoldsQuery = { ledger: { holds: { nodes: Array<{ id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } }>, pageInfo: { endCursor: string | null } }, holdCounts: { open: number, captured: number, voided: number, expired: number, reserved: Array<{ asset: string, amount: string }> } } | null };

export type HoldQueryVariables = Exact<{
  name: string;
  id: string;
}>;


export type HoldQuery = { ledger: { hold: { id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } } | null } | null };

export type CreateHoldMutationVariables = Exact<{
  input: CreateHoldInput;
}>;


export type CreateHoldMutation = { createHold: { id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } } };

export type CaptureHoldMutationVariables = Exact<{
  input: CaptureHoldInput;
}>;


export type CaptureHoldMutation = { captureHold: { id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } } };

export type VoidHoldMutationVariables = Exact<{
  input: VoidHoldInput;
}>;


export type VoidHoldMutation = { voidHold: { id: string, ledger: string, source: string, destination: string, asset: string, amount: string, capturedAmount: string | null, status: HoldStatus, timeoutSeconds: number, createdAt: string, timestamp: string, expiresAt: string | null, resolvedAt: string | null, reference: string | null, metadata: Record<string, string>, idempotencyKey: string, pendingTransferId: string, resolutionTransferId: string | null, tbLedger: number, eventType: { name: string, code: number } } };

export type LedgerFieldsFragment = { name: string, metadata: Record<string, string>, createdAt: string, assetCodes: Array<string>, accountCount: number };

export type LedgersQueryVariables = Exact<{
  first?: number | null | undefined;
  after?: string | null | undefined;
}>;


export type LedgersQuery = { ledgers: { nodes: Array<{ name: string, metadata: Record<string, string>, createdAt: string, assetCodes: Array<string>, accountCount: number }>, pageInfo: { endCursor: string | null } } };

export type LedgerQueryVariables = Exact<{
  name: string;
}>;


export type LedgerQuery = { ledger: { name: string, metadata: Record<string, string>, createdAt: string, assetCodes: Array<string>, accountCount: number } | null };

export type LedgerStatsQueryVariables = Exact<{
  name: string;
}>;


export type LedgerStatsQuery = { ledger: { stats: { accounts: number, accountsToday: number, assets: Array<{ asset: string, tbLedger: number, createdAt: string, accounts: number, issued: string }>, holds: { open: number, captured: number, voided: number, expired: number, reserved: Array<{ asset: string, amount: string }> }, declined: { total: number, reasons: Array<{ reason: string, count: number }> } } } | null };

export type AssetSummaryFieldsFragment = { asset: string, tbLedger: number, createdAt: string, accounts: number, issued: string };

export type LedgerAssetsQueryVariables = Exact<{
  name: string;
}>;


export type LedgerAssetsQuery = { ledger: { assets: Array<{ asset: string, tbLedger: number, createdAt: string, accounts: number, issued: string }> } | null };

export type SearchQueryVariables = Exact<{
  name: string;
  q: string;
}>;


export type SearchQuery = { ledger: { search: { query: string, accounts: Array<{ address: string, assets: Array<string> }>, transactions: Array<{ id: string, reference: string | null, committedAt: string }>, holds: Array<{ id: string, status: HoldStatus }>, requests: Array<{ key: string, operation: Operation, status: RequestStatus }> } } | null };

export type CreateLedgerMutationVariables = Exact<{
  input: CreateLedgerInput;
}>;


export type CreateLedgerMutation = { createLedger: { name: string, metadata: Record<string, string>, createdAt: string, assetCodes: Array<string>, accountCount: number } };

export type ReplaceLedgerMetadataMutationVariables = Exact<{
  input: ReplaceLedgerMetadataInput;
}>;


export type ReplaceLedgerMetadataMutation = { replaceLedgerMetadata: { name: string, metadata: Record<string, string>, createdAt: string, assetCodes: Array<string>, accountCount: number } };

export type MemberFieldsFragment = { subject: string, email: string, emailVerified: boolean, name: string, status: string, role: string | null, addedAt: string | null, addedBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }>, invitation: { id: string, sentAt: string, expiresAt: string, expired: boolean } | null };

export type MembersQueryVariables = Exact<{ [key: string]: never; }>;


export type MembersQuery = { members: Array<{ subject: string, email: string, emailVerified: boolean, name: string, status: string, role: string | null, addedAt: string | null, addedBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }>, invitation: { id: string, sentAt: string, expiresAt: string, expired: boolean } | null }> };

export type InviteMemberMutationVariables = Exact<{
  input: InviteMemberInput;
}>;


export type InviteMemberMutation = { inviteMember: { subject: string, email: string, emailVerified: boolean, name: string, status: string, role: string | null, addedAt: string | null, addedBy: string | null, ledgers: Array<{ ledger: string, role: string, grantedAt: string, grantedBy: string | null }>, invitation: { id: string, sentAt: string, expiresAt: string, expired: boolean } | null } };

export type SetMemberRoleMutationVariables = Exact<{
  input: SetMemberRoleInput;
}>;


export type SetMemberRoleMutation = { setMemberRole: boolean };

export type RemoveMemberMutationVariables = Exact<{
  input: MemberInput;
}>;


export type RemoveMemberMutation = { removeMember: boolean };

export type SetLedgerRoleMutationVariables = Exact<{
  input: SetLedgerRoleInput;
}>;


export type SetLedgerRoleMutation = { setLedgerRole: boolean };

export type RevokeLedgerRoleMutationVariables = Exact<{
  input: RevokeLedgerRoleInput;
}>;


export type RevokeLedgerRoleMutation = { revokeLedgerRole: boolean };

export type ResendInvitationMutationVariables = Exact<{
  input: InvitationInput;
}>;


export type ResendInvitationMutation = { resendInvitation: boolean };

export type DeleteInvitationMutationVariables = Exact<{
  input: InvitationInput;
}>;


export type DeleteInvitationMutation = { deleteInvitation: boolean };

export type WriteRequestFieldsFragment = { key: string, operation: Operation, status: RequestStatus, createdAt: string, resolvedAt: string | null, targetId: string, problemType: string | null, responseStatus: number | null, requestId: string | null, bodyHash: string, attempts: number };

export type WriteRequestsQueryVariables = Exact<{
  name: string;
  first?: number | null | undefined;
  after?: string | null | undefined;
  status?: RequestStatus | null | undefined;
  operation?: Operation | null | undefined;
  search?: string | null | undefined;
}>;


export type WriteRequestsQuery = { ledger: { requests: { nodes: Array<{ key: string, operation: Operation, status: RequestStatus, createdAt: string, resolvedAt: string | null, targetId: string, problemType: string | null, responseStatus: number | null, requestId: string | null, bodyHash: string, attempts: number }>, pageInfo: { endCursor: string | null } } } | null };

export type WriteRequestQueryVariables = Exact<{
  name: string;
  key: string;
}>;


export type WriteRequestQuery = { ledger: { request: { key: string, operation: Operation, status: RequestStatus, createdAt: string, resolvedAt: string | null, targetId: string, problemType: string | null, responseStatus: number | null, requestId: string | null, bodyHash: string, attempts: number, request: Record<string, unknown>, response: Record<string, unknown> | null, transferIds: Array<string> } | null } | null };

export type RuleFieldsFragment = { pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null };

export type EventTypeFieldsFragment = { name: string, code: number, builtIn: boolean, createdAt: string | null, transactions: number };

export type RulesQueryVariables = Exact<{
  name: string;
}>;


export type RulesQuery = { ledger: { rules: Array<{ pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null }> } | null };

export type RuleMatchQueryVariables = Exact<{
  name: string;
  address: string;
}>;


export type RuleMatchQuery = { ledger: { ruleMatch: { address: string, existing: Array<string>, rule: { pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null }, candidates: Array<{ matches: boolean, rule: { pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null } }> } } | null };

export type EventTypesQueryVariables = Exact<{
  name: string;
}>;


export type EventTypesQuery = { ledger: { eventTypes: Array<{ name: string, code: number, builtIn: boolean, createdAt: string | null, transactions: number }> } | null };

export type PutRuleMutationVariables = Exact<{
  input: PutRuleInput;
}>;


export type PutRuleMutation = { putRule: { pattern: string, limit: BalanceLimit, history: boolean, code: number, builtIn: boolean, segments: number, createdAt: string | null, updatedAt: string | null } };

export type DeleteRuleMutationVariables = Exact<{
  input: DeleteRuleInput;
}>;


export type DeleteRuleMutation = { deleteRule: boolean };

export type CreateEventTypeMutationVariables = Exact<{
  input: CreateEventTypeInput;
}>;


export type CreateEventTypeMutation = { createEventType: { name: string, code: number, builtIn: boolean, createdAt: string | null, transactions: number } };

export type OrganizationFieldsFragment = { id: string, keycloakId: string, alias: string, name: string, status: string, suspensionReason: string | null, createdAt: string, createdBy: string | null, members: number, ledgers: number, owners: Array<{ subject: string, email: string, name: string, status: string, since: string }> };

export type OrganizationsQueryVariables = Exact<{
  first?: number | null | undefined;
  after?: string | null | undefined;
}>;


export type OrganizationsQuery = { organizations: { nodes: Array<{ id: string, keycloakId: string, alias: string, name: string, status: string, suspensionReason: string | null, createdAt: string, createdBy: string | null, members: number, ledgers: number, owners: Array<{ subject: string, email: string, name: string, status: string, since: string }> }>, pageInfo: { hasNextPage: boolean, endCursor: string | null } } };

export type OrganizationQueryVariables = Exact<{
  alias: string;
}>;


export type OrganizationQuery = { organization: { id: string, keycloakId: string, alias: string, name: string, status: string, suspensionReason: string | null, createdAt: string, createdBy: string | null, members: number, ledgers: number, owners: Array<{ subject: string, email: string, name: string, status: string, since: string }> } | null };

export type CreateOrganizationMutationVariables = Exact<{
  input: CreateOrganizationInput;
}>;


export type CreateOrganizationMutation = { createOrganization: { id: string, keycloakId: string, alias: string, name: string, status: string, suspensionReason: string | null, createdAt: string, createdBy: string | null, members: number, ledgers: number, owners: Array<{ subject: string, email: string, name: string, status: string, since: string }> } };

export type UpdateOrganizationMutationVariables = Exact<{
  input: UpdateOrganizationInput;
}>;


export type UpdateOrganizationMutation = { updateOrganization: { id: string, keycloakId: string, alias: string, name: string, status: string, suspensionReason: string | null, createdAt: string, createdBy: string | null, members: number, ledgers: number, owners: Array<{ subject: string, email: string, name: string, status: string, since: string }> } };

export type TransactionFieldsFragment = { id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } };

export type TransactionsQueryVariables = Exact<{
  name: string;
  first?: number | null | undefined;
  after?: string | null | undefined;
  address?: string | null | undefined;
  asset?: string | null | undefined;
  reference?: string | null | undefined;
  eventType?: string | null | undefined;
  committedAfter?: string | null | undefined;
  committedBefore?: string | null | undefined;
}>;


export type TransactionsQuery = { ledger: { transactions: { nodes: Array<{ id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } }>, pageInfo: { endCursor: string | null } } } | null };

export type TransactionQueryVariables = Exact<{
  name: string;
  id: string;
}>;


export type TransactionQuery = { ledger: { transaction: { id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } } | null } | null };

export type CreateTransactionMutationVariables = Exact<{
  input: CreateTransactionInput;
}>;


export type CreateTransactionMutation = { createTransaction: { id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } } };

export type RevertTransactionMutationVariables = Exact<{
  input: RevertTransactionInput;
}>;


export type RevertTransactionMutation = { revertTransaction: { id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } } };

export type ReplaceTransactionMetadataMutationVariables = Exact<{
  input: ReplaceTransactionMetadataInput;
}>;


export type ReplaceTransactionMetadataMutation = { replaceTransactionMetadata: { id: string, ledger: string, reference: string | null, businessTime: string | null, metadata: Record<string, string>, committedAt: string, timestamp: string, reverts: string | null, revertedBy: string | null, idempotencyKey: string, requestId: string | null, postings: Array<{ source: string, destination: string, asset: string, amount: string, transferId: string, tbLedger: number }>, eventType: { name: string, code: number } } };

export class TypedDocumentString<TResult, TVariables>
  extends String
  implements DocumentTypeDecoration<TResult, TVariables>
{
  __apiType?: NonNullable<DocumentTypeDecoration<TResult, TVariables>['__apiType']>;
  private value: string;
  public __meta__?: Record<string, any> | undefined;

  constructor(value: string, __meta__?: Record<string, any> | undefined) {
    super(value);
    this.value = value;
    this.__meta__ = __meta__;
  }

  override toString(): string & DocumentTypeDecoration<TResult, TVariables> {
    return this.value;
  }
}
export const AccountSummaryFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"AccountSummaryFields"}) as unknown as TypedDocumentString<AccountSummaryFieldsFragment, unknown>;
export const AccountFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"AccountFields"}) as unknown as TypedDocumentString<AccountFieldsFragment, unknown>;
export const ApiKeyFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"ApiKeyFields"}) as unknown as TypedDocumentString<ApiKeyFieldsFragment, unknown>;
export const MachineClientFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"MachineClientFields"}) as unknown as TypedDocumentString<MachineClientFieldsFragment, unknown>;
export const HoldFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"HoldFields"}) as unknown as TypedDocumentString<HoldFieldsFragment, unknown>;
export const LedgerFieldsFragmentDoc = new TypedDocumentString(`
    fragment LedgerFields on Ledger {
  name
  metadata
  createdAt
  assetCodes
  accountCount
}
    `, {"fragmentName":"LedgerFields"}) as unknown as TypedDocumentString<LedgerFieldsFragment, unknown>;
export const AssetSummaryFieldsFragmentDoc = new TypedDocumentString(`
    fragment AssetSummaryFields on AssetSummary {
  asset
  tbLedger
  createdAt
  accounts
  issued
}
    `, {"fragmentName":"AssetSummaryFields"}) as unknown as TypedDocumentString<AssetSummaryFieldsFragment, unknown>;
export const MemberFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"MemberFields"}) as unknown as TypedDocumentString<MemberFieldsFragment, unknown>;
export const WriteRequestFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"WriteRequestFields"}) as unknown as TypedDocumentString<WriteRequestFieldsFragment, unknown>;
export const RuleFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"RuleFields"}) as unknown as TypedDocumentString<RuleFieldsFragment, unknown>;
export const EventTypeFieldsFragmentDoc = new TypedDocumentString(`
    fragment EventTypeFields on EventType {
  name
  code
  builtIn
  createdAt
  transactions
}
    `, {"fragmentName":"EventTypeFields"}) as unknown as TypedDocumentString<EventTypeFieldsFragment, unknown>;
export const OrganizationFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"OrganizationFields"}) as unknown as TypedDocumentString<OrganizationFieldsFragment, unknown>;
export const TransactionFieldsFragmentDoc = new TypedDocumentString(`
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
    `, {"fragmentName":"TransactionFields"}) as unknown as TypedDocumentString<TransactionFieldsFragment, unknown>;
export const AccountsDocument = new TypedDocumentString(`
    query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {
  ledger(name: $name) {
    accounts(
      first: $first
      after: $after
      prefix: $prefix
      search: $search
      asset: $asset
    ) {
      nodes {
        ...AccountSummaryFields
      }
      pageInfo {
        endCursor
      }
    }
  }
}
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
}`) as unknown as TypedDocumentString<AccountsQuery, AccountsQueryVariables>;
export const AccountDocument = new TypedDocumentString(`
    query Account($name: String!, $address: String!) {
  ledger(name: $name) {
    account(address: $address) {
      ...AccountFields
    }
  }
}
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
}`) as unknown as TypedDocumentString<AccountQuery, AccountQueryVariables>;
export const AccountHistoryDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<AccountHistoryQuery, AccountHistoryQueryVariables>;
export const SegmentsDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<SegmentsQuery, SegmentsQueryVariables>;
export const AggregateBalancesDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<AggregateBalancesQuery, AggregateBalancesQueryVariables>;
export const WriteTargetDocument = new TypedDocumentString(`
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
fragment RuleFields on Rule {
  pattern
  limit
  history
  code
  builtIn
  segments
  createdAt
  updatedAt
}`) as unknown as TypedDocumentString<WriteTargetQuery, WriteTargetQueryVariables>;
export const ReplaceAccountMetadataDocument = new TypedDocumentString(`
    mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {
  replaceAccountMetadata(input: $input) {
    ...AccountFields
  }
}
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
}`) as unknown as TypedDocumentString<ReplaceAccountMetadataMutation, ReplaceAccountMetadataMutationVariables>;
export const ApiKeysDocument = new TypedDocumentString(`
    query ApiKeys {
  apiKeys {
    ...ApiKeyFields
  }
}
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
}`) as unknown as TypedDocumentString<ApiKeysQuery, ApiKeysQueryVariables>;
export const MachineClientsDocument = new TypedDocumentString(`
    query MachineClients {
  machineClients {
    ...MachineClientFields
  }
}
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
}`) as unknown as TypedDocumentString<MachineClientsQuery, MachineClientsQueryVariables>;
export const CreateApiKeyDocument = new TypedDocumentString(`
    mutation CreateApiKey($input: CreateAPIKeyInput!) {
  createApiKey(input: $input) {
    key {
      ...ApiKeyFields
    }
    secret
  }
}
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
}`) as unknown as TypedDocumentString<CreateApiKeyMutation, CreateApiKeyMutationVariables>;
export const RevokeApiKeyDocument = new TypedDocumentString(`
    mutation RevokeApiKey($input: APIKeyInput!) {
  revokeApiKey(input: $input)
}
    `) as unknown as TypedDocumentString<RevokeApiKeyMutation, RevokeApiKeyMutationVariables>;
export const CreateMachineClientDocument = new TypedDocumentString(`
    mutation CreateMachineClient($input: CreateMachineClientInput!) {
  createMachineClient(input: $input) {
    client {
      ...MachineClientFields
    }
    secret
    tokenUrl
  }
}
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
}`) as unknown as TypedDocumentString<CreateMachineClientMutation, CreateMachineClientMutationVariables>;
export const RegenerateMachineClientSecretDocument = new TypedDocumentString(`
    mutation RegenerateMachineClientSecret($input: MachineClientInput!) {
  regenerateMachineClientSecret(input: $input) {
    secret
  }
}
    `) as unknown as TypedDocumentString<RegenerateMachineClientSecretMutation, RegenerateMachineClientSecretMutationVariables>;
export const DeleteMachineClientDocument = new TypedDocumentString(`
    mutation DeleteMachineClient($input: MachineClientInput!) {
  deleteMachineClient(input: $input)
}
    `) as unknown as TypedDocumentString<DeleteMachineClientMutation, DeleteMachineClientMutationVariables>;
export const SetCredentialRoleDocument = new TypedDocumentString(`
    mutation SetCredentialRole($input: SetCredentialRoleInput!) {
  setCredentialRole(input: $input)
}
    `) as unknown as TypedDocumentString<SetCredentialRoleMutation, SetCredentialRoleMutationVariables>;
export const RevokeCredentialRoleDocument = new TypedDocumentString(`
    mutation RevokeCredentialRole($input: RevokeCredentialRoleInput!) {
  revokeCredentialRole(input: $input)
}
    `) as unknown as TypedDocumentString<RevokeCredentialRoleMutation, RevokeCredentialRoleMutationVariables>;
export const HoldsDocument = new TypedDocumentString(`
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
}`) as unknown as TypedDocumentString<HoldsQuery, HoldsQueryVariables>;
export const HoldDocument = new TypedDocumentString(`
    query Hold($name: String!, $id: U128!) {
  ledger(name: $name) {
    hold(id: $id) {
      ...HoldFields
    }
  }
}
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
}`) as unknown as TypedDocumentString<HoldQuery, HoldQueryVariables>;
export const CreateHoldDocument = new TypedDocumentString(`
    mutation CreateHold($input: CreateHoldInput!) {
  createHold(input: $input) {
    ...HoldFields
  }
}
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
}`) as unknown as TypedDocumentString<CreateHoldMutation, CreateHoldMutationVariables>;
export const CaptureHoldDocument = new TypedDocumentString(`
    mutation CaptureHold($input: CaptureHoldInput!) {
  captureHold(input: $input) {
    ...HoldFields
  }
}
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
}`) as unknown as TypedDocumentString<CaptureHoldMutation, CaptureHoldMutationVariables>;
export const VoidHoldDocument = new TypedDocumentString(`
    mutation VoidHold($input: VoidHoldInput!) {
  voidHold(input: $input) {
    ...HoldFields
  }
}
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
}`) as unknown as TypedDocumentString<VoidHoldMutation, VoidHoldMutationVariables>;
export const LedgersDocument = new TypedDocumentString(`
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
    fragment LedgerFields on Ledger {
  name
  metadata
  createdAt
  assetCodes
  accountCount
}`) as unknown as TypedDocumentString<LedgersQuery, LedgersQueryVariables>;
export const LedgerDocument = new TypedDocumentString(`
    query Ledger($name: String!) {
  ledger(name: $name) {
    ...LedgerFields
  }
}
    fragment LedgerFields on Ledger {
  name
  metadata
  createdAt
  assetCodes
  accountCount
}`) as unknown as TypedDocumentString<LedgerQuery, LedgerQueryVariables>;
export const LedgerStatsDocument = new TypedDocumentString(`
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
    fragment AssetSummaryFields on AssetSummary {
  asset
  tbLedger
  createdAt
  accounts
  issued
}`) as unknown as TypedDocumentString<LedgerStatsQuery, LedgerStatsQueryVariables>;
export const LedgerAssetsDocument = new TypedDocumentString(`
    query LedgerAssets($name: String!) {
  ledger(name: $name) {
    assets {
      ...AssetSummaryFields
    }
  }
}
    fragment AssetSummaryFields on AssetSummary {
  asset
  tbLedger
  createdAt
  accounts
  issued
}`) as unknown as TypedDocumentString<LedgerAssetsQuery, LedgerAssetsQueryVariables>;
export const SearchDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<SearchQuery, SearchQueryVariables>;
export const CreateLedgerDocument = new TypedDocumentString(`
    mutation CreateLedger($input: CreateLedgerInput!) {
  createLedger(input: $input) {
    ...LedgerFields
  }
}
    fragment LedgerFields on Ledger {
  name
  metadata
  createdAt
  assetCodes
  accountCount
}`) as unknown as TypedDocumentString<CreateLedgerMutation, CreateLedgerMutationVariables>;
export const ReplaceLedgerMetadataDocument = new TypedDocumentString(`
    mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {
  replaceLedgerMetadata(input: $input) {
    ...LedgerFields
  }
}
    fragment LedgerFields on Ledger {
  name
  metadata
  createdAt
  assetCodes
  accountCount
}`) as unknown as TypedDocumentString<ReplaceLedgerMetadataMutation, ReplaceLedgerMetadataMutationVariables>;
export const MembersDocument = new TypedDocumentString(`
    query Members {
  members {
    ...MemberFields
  }
}
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
}`) as unknown as TypedDocumentString<MembersQuery, MembersQueryVariables>;
export const InviteMemberDocument = new TypedDocumentString(`
    mutation InviteMember($input: InviteMemberInput!) {
  inviteMember(input: $input) {
    ...MemberFields
  }
}
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
}`) as unknown as TypedDocumentString<InviteMemberMutation, InviteMemberMutationVariables>;
export const SetMemberRoleDocument = new TypedDocumentString(`
    mutation SetMemberRole($input: SetMemberRoleInput!) {
  setMemberRole(input: $input)
}
    `) as unknown as TypedDocumentString<SetMemberRoleMutation, SetMemberRoleMutationVariables>;
export const RemoveMemberDocument = new TypedDocumentString(`
    mutation RemoveMember($input: MemberInput!) {
  removeMember(input: $input)
}
    `) as unknown as TypedDocumentString<RemoveMemberMutation, RemoveMemberMutationVariables>;
export const SetLedgerRoleDocument = new TypedDocumentString(`
    mutation SetLedgerRole($input: SetLedgerRoleInput!) {
  setLedgerRole(input: $input)
}
    `) as unknown as TypedDocumentString<SetLedgerRoleMutation, SetLedgerRoleMutationVariables>;
export const RevokeLedgerRoleDocument = new TypedDocumentString(`
    mutation RevokeLedgerRole($input: RevokeLedgerRoleInput!) {
  revokeLedgerRole(input: $input)
}
    `) as unknown as TypedDocumentString<RevokeLedgerRoleMutation, RevokeLedgerRoleMutationVariables>;
export const ResendInvitationDocument = new TypedDocumentString(`
    mutation ResendInvitation($input: InvitationInput!) {
  resendInvitation(input: $input)
}
    `) as unknown as TypedDocumentString<ResendInvitationMutation, ResendInvitationMutationVariables>;
export const DeleteInvitationDocument = new TypedDocumentString(`
    mutation DeleteInvitation($input: InvitationInput!) {
  deleteInvitation(input: $input)
}
    `) as unknown as TypedDocumentString<DeleteInvitationMutation, DeleteInvitationMutationVariables>;
export const WriteRequestsDocument = new TypedDocumentString(`
    query WriteRequests($name: String!, $first: Int, $after: String, $status: RequestStatus, $operation: Operation, $search: String) {
  ledger(name: $name) {
    requests(
      first: $first
      after: $after
      status: $status
      operation: $operation
      search: $search
    ) {
      nodes {
        ...WriteRequestFields
      }
      pageInfo {
        endCursor
      }
    }
  }
}
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
}`) as unknown as TypedDocumentString<WriteRequestsQuery, WriteRequestsQueryVariables>;
export const WriteRequestDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<WriteRequestQuery, WriteRequestQueryVariables>;
export const RulesDocument = new TypedDocumentString(`
    query Rules($name: String!) {
  ledger(name: $name) {
    rules {
      ...RuleFields
    }
  }
}
    fragment RuleFields on Rule {
  pattern
  limit
  history
  code
  builtIn
  segments
  createdAt
  updatedAt
}`) as unknown as TypedDocumentString<RulesQuery, RulesQueryVariables>;
export const RuleMatchDocument = new TypedDocumentString(`
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
    fragment RuleFields on Rule {
  pattern
  limit
  history
  code
  builtIn
  segments
  createdAt
  updatedAt
}`) as unknown as TypedDocumentString<RuleMatchQuery, RuleMatchQueryVariables>;
export const EventTypesDocument = new TypedDocumentString(`
    query EventTypes($name: String!) {
  ledger(name: $name) {
    eventTypes {
      ...EventTypeFields
    }
  }
}
    fragment EventTypeFields on EventType {
  name
  code
  builtIn
  createdAt
  transactions
}`) as unknown as TypedDocumentString<EventTypesQuery, EventTypesQueryVariables>;
export const PutRuleDocument = new TypedDocumentString(`
    mutation PutRule($input: PutRuleInput!) {
  putRule(input: $input) {
    ...RuleFields
  }
}
    fragment RuleFields on Rule {
  pattern
  limit
  history
  code
  builtIn
  segments
  createdAt
  updatedAt
}`) as unknown as TypedDocumentString<PutRuleMutation, PutRuleMutationVariables>;
export const DeleteRuleDocument = new TypedDocumentString(`
    mutation DeleteRule($input: DeleteRuleInput!) {
  deleteRule(input: $input)
}
    `) as unknown as TypedDocumentString<DeleteRuleMutation, DeleteRuleMutationVariables>;
export const CreateEventTypeDocument = new TypedDocumentString(`
    mutation CreateEventType($input: CreateEventTypeInput!) {
  createEventType(input: $input) {
    ...EventTypeFields
  }
}
    fragment EventTypeFields on EventType {
  name
  code
  builtIn
  createdAt
  transactions
}`) as unknown as TypedDocumentString<CreateEventTypeMutation, CreateEventTypeMutationVariables>;
export const OrganizationsDocument = new TypedDocumentString(`
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
}`) as unknown as TypedDocumentString<OrganizationsQuery, OrganizationsQueryVariables>;
export const OrganizationDocument = new TypedDocumentString(`
    query Organization($alias: String!) {
  organization(alias: $alias) {
    ...OrganizationFields
  }
}
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
}`) as unknown as TypedDocumentString<OrganizationQuery, OrganizationQueryVariables>;
export const CreateOrganizationDocument = new TypedDocumentString(`
    mutation CreateOrganization($input: CreateOrganizationInput!) {
  createOrganization(input: $input) {
    ...OrganizationFields
  }
}
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
}`) as unknown as TypedDocumentString<CreateOrganizationMutation, CreateOrganizationMutationVariables>;
export const UpdateOrganizationDocument = new TypedDocumentString(`
    mutation UpdateOrganization($input: UpdateOrganizationInput!) {
  updateOrganization(input: $input) {
    ...OrganizationFields
  }
}
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
}`) as unknown as TypedDocumentString<UpdateOrganizationMutation, UpdateOrganizationMutationVariables>;
export const TransactionsDocument = new TypedDocumentString(`
    query Transactions($name: String!, $first: Int, $after: String, $address: String, $asset: String, $reference: String, $eventType: String, $committedAfter: DateTime, $committedBefore: DateTime) {
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
}`) as unknown as TypedDocumentString<TransactionsQuery, TransactionsQueryVariables>;
export const TransactionDocument = new TypedDocumentString(`
    query Transaction($name: String!, $id: U128!) {
  ledger(name: $name) {
    transaction(id: $id) {
      ...TransactionFields
    }
  }
}
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
}`) as unknown as TypedDocumentString<TransactionQuery, TransactionQueryVariables>;
export const CreateTransactionDocument = new TypedDocumentString(`
    mutation CreateTransaction($input: CreateTransactionInput!) {
  createTransaction(input: $input) {
    ...TransactionFields
  }
}
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
}`) as unknown as TypedDocumentString<CreateTransactionMutation, CreateTransactionMutationVariables>;
export const RevertTransactionDocument = new TypedDocumentString(`
    mutation RevertTransaction($input: RevertTransactionInput!) {
  revertTransaction(input: $input) {
    ...TransactionFields
  }
}
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
}`) as unknown as TypedDocumentString<RevertTransactionMutation, RevertTransactionMutationVariables>;
export const ReplaceTransactionMetadataDocument = new TypedDocumentString(`
    mutation ReplaceTransactionMetadata($input: ReplaceTransactionMetadataInput!) {
  replaceTransactionMetadata(input: $input) {
    ...TransactionFields
  }
}
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
}`) as unknown as TypedDocumentString<ReplaceTransactionMetadataMutation, ReplaceTransactionMetadataMutationVariables>;