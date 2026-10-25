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
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": typeof types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": typeof types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": typeof types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": typeof types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": typeof types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": typeof types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": typeof types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.ReplaceLedgerMetadataDocument,
};
const documents: Documents = {
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.ReplaceLedgerMetadataDocument,
};

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


export function graphql(source: string) {
  return (documents as any)[source] ?? {};
}
