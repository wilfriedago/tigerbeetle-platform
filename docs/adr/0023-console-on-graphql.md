# 23. Console on GraphQL

Status: Accepted, amended by [ADR-0030](0030-console-sessions-through-a-backend-for-frontend.md)
Date: 2026-10-25

## Context and Problem Statement

ADR-0003 made the console "a thin client of the OpenAPI contract", and ADR-0014 has it talk to
the API only through `packages/api-client`, with each feature's `api.ts` holding REST query
options and mutations. Several pages need several REST calls: the overview reads the ledger, its
stats, assets, transactions and holds separately, and New transaction calls `getAccount` and
`matchRule` once per posting. The API now serves GraphQL with a schema generated from Go
(ADR-0020). REST stays the public API for third parties; the console should stop using it.

This record replaces only the data-access parts of ADR-0014 (the console reaches the API through
`packages/api-client`; the Idempotency-Key travels as a header) and the "thin client of the
OpenAPI contract" driver of ADR-0003. Routing, TanStack Query as the only cache, forms, tables,
the design system, the slices and the write rules of ADR-0007 stand.

## Decision Drivers

- TanStack Query stays the only cache (ADR-0014): no normalised GraphQL cache next to it.
- No hand-written types: documents are typed from the generated schema.
- Money stays bigint and integer strings, never a JavaScript number.
- Errors behave exactly as today in the UI: problem panel, declines with their posting, key
  renewal, `Retry-After`.
- The health indicator works even when GraphQL is disabled or broken.
- Integrators keep seeing REST, the API they use.

## Considered Options

1. graphql-codegen's client preset with string documents, a small fetch client, TanStack Query
   (chosen).
2. Apollo Client or urql with their normalised caches.
3. Keep the console on REST.

Option 2 puts a second cache beside TanStack Query, with its own invalidation rules. Option 3
keeps the extra round trips and leaves GraphQL without its main client.

## Decision Outcome

Chosen option: 1.

- Generation: `@graphql-codegen/cli` 7.4.6 with `@graphql-codegen/client-preset` 6.2.3 and
  `graphql` 17.0.2, devDependencies of `apps/console`; none needs the TypeScript compiler API.
  `apps/console/codegen.ts` reads `../../packages/graphql/schema.graphql` (the workspace package
  `@tigerbeetle-platform/graphql`, so Turborepo hashes the schema) and the documents each slice
  declares in its `api.ts` with the generated `graphql()` function. `documentMode: 'string'`
  emits typed strings, so the bundle carries no GraphQL runtime. Scalars map to `string`
  (`Amount`, `SignedAmount`, `U128`, `U64`, `DateTime`), `number` (`Int64`, counts only),
  `Record<string, string>` (`Metadata`) and `unknown` (`JSON`). Output goes to
  `src/graphql/gen`, committed and ignored by Oxfmt and Oxlint. The console's `generate` script
  runs graphql-codegen, then the route generator.
- Client: `src/lib/graphql/client.ts`, about 80 lines. It POSTs `{query, variables}` to
  `${VITE_API_URL ?? '/api'}/graphql` with an `X-Request-Id` from `packages/api-client`, the
  signal TanStack Query passes and a 15 s timeout. Any entry in `errors` rejects the promise; a
  page never shows partial data next to an error.
- Errors: `src/lib/graphql/errors.ts` builds a `GraphQLProblemError` from the first error's
  `extensions` (`code`, `reason`, `type`, `title`, `status`, `detail`, `requestId`,
  `retryAfter`, `posting`, `results`, ADR-0021), with a separate branch for gqlgen's parse and
  validation errors and for network failures. `lib/problem.ts`, `lib/idempotency.ts`,
  `lib/query-client.ts` and `features/requests/stored-problem.ts` accept either error through
  one small interface (`status`, `type`, `problem`, `requestId`, `retryAfter`). The rules do not
  change: a key is spent by any answer below 500 except `request-in-flight` and `write-pending`,
  and client errors are not retried.
- Idempotency-Key: an `idempotencyKey` field of the mutation input (ADR-0021), because one
  GraphQL request may hold several mutations. `useIdempotencyKey` is unchanged: the form makes
  the key when it opens, sends it again on a retry and renews it only after a final answer.
- Queries: keys stay under `['ledger', name]`, so one invalidation still refreshes a ledger.
  The overview becomes one query (the ledger with stats, assets, transactions and holds), and
  New transaction reads its accounts and rule matches in one nested query.
- REST that stays: the system slice (`getInfo`, polled by the shell's health indicator, and
  `getReconcilerStatus`) keeps calling REST through the generated `packages/api-client`
  (ADR-0019), so the indicator still works without GraphQL. No hand-written `fetch` anywhere.
- Copy as curl keeps showing the equivalent REST request: integrators build on REST, its spec
  and its problems. A mutation's input is the REST body plus `ledger` and `idempotencyKey`, so
  the page builds the REST body by dropping those two fields. There is no "Copy as GraphQL".
- Migration happens slice by slice (ledgers and search, settings, accounts and balances,
  transactions and New transaction, holds, requests, overview), one commit each, with the
  slice's tests kept, then a browser check of every page.

### Consequences

- Good: fewer requests per page, and types that follow the schema the API generates.
- Good: one error interface for REST and GraphQL, so the UI code does not care which answered.
- Good: no GraphQL runtime or second cache in the bundle.
- Bad: the console now depends on two contracts, GraphQL for pages and REST for the system
  slice and curl.
- Bad: GraphQL reports resolver errors in the `errors` member of a 200 answer, so error handling
  reads the body, not the status; the shared interface hides it, but tests cover both paths.
- Bad: the curl a user copies is not the request the console sent; its comment says so.

## More Information

- [ADR-0014](0014-console-architecture.md): the console architecture this record amends.
- [ADR-0003](0003-go-api-typescript-react-console.md): the "thin client" driver it replaces.
- [ADR-0019](0019-rest-client-generated-with-orval.md), [ADR-0020](0020-graphql-schema-generated-from-go-registrations.md),
  [ADR-0021](0021-transports-over-one-application-layer.md).
- `docs/plans/ddd-transports/plan.md`, section 17.
- graphql-codegen client preset: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client
