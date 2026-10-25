/* eslint-disable */
/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
import type { DocumentTypeDecoration } from '@graphql-typed-document-node/core';
/** The balance limit flag of the TigerBeetle accounts a rule creates. */
export type BalanceLimit =
  | 'credits_must_not_exceed_debits'
  | 'debits_must_not_exceed_credits'
  | 'none';

export type CreateEventTypeInput = {
  /** 2 to 65535; the next free code when absent. */
  code?: number | null | undefined;
  ledger: string;
  /** Lowercase, starting with a letter, such as card.authorized. */
  name: string;
};

export type CreateLedgerInput = {
  metadata?: Record<string, string> | null | undefined;
  /** Lowercase letters, digits, dash and underscore, 1 to 63 characters. */
  name: string;
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

/** The kind of write an idempotency record holds. */
export type Operation =
  | 'capture'
  | 'hold'
  | 'revert'
  | 'transaction'
  | 'void';

export type PutRuleInput = {
  /** The TigerBeetle account code, 1 to 65535. */
  code: number;
  history?: boolean | null | undefined;
  ledger: string;
  limit: BalanceLimit;
  /** *, an address such as world, or a prefix ending with a colon such as users:. */
  pattern: string;
};

export type ReplaceLedgerMetadataInput = {
  ledger: string;
  metadata: Record<string, string>;
};

/** The state of an idempotency record. */
export type RequestStatus =
  | 'committed'
  | 'declined'
  | 'failed'
  | 'pending';

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