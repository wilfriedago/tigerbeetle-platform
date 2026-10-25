/* eslint-disable */
import * as types from './graphql';



/**
 * Map of all GraphQL operations in the project.
 *
 * This map has several performance disadvantages:
 * 1. It is not tree-shakeable, so it will include all operations in the project.
 * 2. It is not minifiable, so the string of a GraphQL query will be multiple times inside the bundle.
 * 3. It does not support dead code elimination, so it will add unused operations.
 *
 * Therefore it is highly recommended to use the babel or swc plugin for production.
 * Learn more about it here: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client#reducing-bundle-size
 */
type Documents = {
    "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n": typeof types.AccountSummaryFieldsFragmentDoc,
    "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n": typeof types.AccountFieldsFragmentDoc,
    "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": typeof types.AccountsDocument,
    "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n": typeof types.AccountDocument,
    "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n": typeof types.AccountHistoryDocument,
    "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n": typeof types.SegmentsDocument,
    "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n": typeof types.AggregateBalancesDocument,
    "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n": typeof types.WriteTargetDocument,
    "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n": typeof types.ReplaceAccountMetadataDocument,
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": typeof types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": typeof types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": typeof types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": typeof types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": typeof types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": typeof types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": typeof types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.ReplaceLedgerMetadataDocument,
    "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n": typeof types.RuleFieldsFragmentDoc,
    "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n": typeof types.EventTypeFieldsFragmentDoc,
    "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n": typeof types.RulesDocument,
    "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n": typeof types.RuleMatchDocument,
    "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n": typeof types.EventTypesDocument,
    "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n": typeof types.PutRuleDocument,
    "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n": typeof types.DeleteRuleDocument,
    "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n": typeof types.CreateEventTypeDocument,
};
const documents: Documents = {
    "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n": types.AccountSummaryFieldsFragmentDoc,
    "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n": types.AccountFieldsFragmentDoc,
    "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": types.AccountsDocument,
    "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n": types.AccountDocument,
    "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n": types.AccountHistoryDocument,
    "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n": types.SegmentsDocument,
    "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n": types.AggregateBalancesDocument,
    "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n": types.WriteTargetDocument,
    "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n": types.ReplaceAccountMetadataDocument,
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.ReplaceLedgerMetadataDocument,
    "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n": types.RuleFieldsFragmentDoc,
    "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n": types.EventTypeFieldsFragmentDoc,
    "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n": types.RulesDocument,
    "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n": types.RuleMatchDocument,
    "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n": types.EventTypesDocument,
    "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n": types.PutRuleDocument,
    "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n": types.DeleteRuleDocument,
    "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n": types.CreateEventTypeDocument,
};

/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n"): typeof import('./graphql').AccountSummaryFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n"): typeof import('./graphql').AccountFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AccountsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n"): typeof import('./graphql').AccountDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AccountHistoryDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n"): typeof import('./graphql').SegmentsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AggregateBalancesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').WriteTargetDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n"): typeof import('./graphql').ReplaceAccountMetadataDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n"): typeof import('./graphql').LedgerFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n"): typeof import('./graphql').LedgersDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').LedgerDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').LedgerStatsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n"): typeof import('./graphql').AssetSummaryFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n"): typeof import('./graphql').LedgerAssetsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').SearchDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').CreateLedgerDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').ReplaceLedgerMetadataDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n"): typeof import('./graphql').RuleFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n"): typeof import('./graphql').EventTypeFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n"): typeof import('./graphql').RulesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n"): typeof import('./graphql').RuleMatchDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n"): typeof import('./graphql').EventTypesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n"): typeof import('./graphql').PutRuleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n"): typeof import('./graphql').DeleteRuleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n"): typeof import('./graphql').CreateEventTypeDocument;


export function graphql(source: string) {
  return (documents as any)[source] ?? {};
}
