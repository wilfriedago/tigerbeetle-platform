# 19. REST client generated with orval

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

`packages/api-client` today is openapi-typescript types (`src/schema.d.ts`) under a hand-written
ky 2.2.0 wrapper (`src/index.ts`) with one method per operation, as ADR-0003 and ADR-0008 chose.
ADR-0003 already expected the hand-written methods to grow tedious. With the spec now generated
from Go (ADR-0018), the client should be generated from the spec as well, organised by resource
(one module per OpenAPI tag), with a thin hand-written layer for what the spec cannot express.

The owner asked for @hey-api/openapi-ts unless an ADR argues for orval or for openapi-typescript
with openapi-fetch. This record makes that argument. It replaces the client half of ADR-0008 and
the ky choice of ADR-0003.

## Decision Drivers

- Generated from `packages/openapi/openapi.yaml`, committed under `src/gen`, never edited.
- One module per OpenAPI tag: operations, ledgers, accounts, transactions, holds, settings.
- Runs with the workspace TypeScript, 7.0.2, which ships no compiler API; no second TypeScript
  kept for one tool, the situation openapi-typescript's 5.9.3 pin created.
- Keeps every behaviour of today's client: a request ID on every call, the Idempotency-Key on
  money writes, retries only for idempotent requests, `Retry-After` on 409 and 503, and the abort
  signal TanStack Query passes.
- A stable release, pinned, that pnpm's 24-hour minimum release age accepts.

## Considered Options

1. orval 8.44.0, `tags-split` mode, `fetch` client, a custom mutator (chosen).
2. @hey-api/openapi-ts 0.99.0.
3. openapi-typescript with openapi-fetch.

@hey-api/openapi-ts 0.99.0, the latest stable release, imports the TypeScript compiler API at run
time (`ts.factory`, `createPrinter`). TypeScript 7.0.2 does not provide it, and the package's peer
range accepts 7.0.2, so pnpm would not warn and the generator would fail when run. The workaround,
aliasing `typescript` to `@typescript/typescript6` inside the package, keeps a second TypeScript
for one tool. It also splits by tag only into classes or namespaces inside one `sdk.gen.ts`, not
one module per tag, and its stable line is four months old while a rewrite ships as `next`
prereleases. openapi-typescript needs the compiler API too, and openapi-fetch adds a typed `fetch`
but no per-tag modules.

orval's core, fetch and query packages import no `typescript`, run on Node 24, write one folder
per tag with `mode: 'tags-split'`, and send every request through one hand-written function
(`override.mutator`), which is exactly the thin layer wanted. orval 8.45.0 was published on
2026-10-25 and is blocked by pnpm's minimum release age for a day, hence 8.44.0.

## Decision Outcome

Chosen option: 1, orval 8.44.0, a devDependency of `packages/api-client`.

```
packages/api-client/
  orval.config.ts        input ../openapi/openapi.yaml; mode tags-split; client fetch;
                         override.mutator ./src/client/fetcher.ts
  src/index.ts           exports only: the tag modules, the model types, the client layer
  src/gen/               orval output, committed, ignored by Oxfmt and Oxlint
    operations/ ledgers/ accounts/ transactions/ holds/ settings/ model/
  src/client/
    config.ts            base URL, fetch, timeout (15 s), retries (2); configureClient()
    errors.ts            ApiError from RFC 9457 problems: status, type, requestId, retryAfter
    headers.ts           X-Request-Id and Idempotency-Key, randomRequestId()
    fetcher.ts           the mutator: URL, headers, timeout and abort, retry policy, problems
```

Behaviour kept from today's client:

- Every call sends `X-Request-Id` (32 hexadecimal characters from `crypto.getRandomValues`).
  The five money writes take `Idempotency-Key` as the typed header parameter orval generates from
  the spec, so a call without it does not compile.
- Only GET, HEAD and OPTIONS retry, on network errors and 502, 503 and 504, at most `retries`
  times; a 503 with `Retry-After` waits that long, capped at 10 s. Writes never retry. A 409 is
  never retried; its `Retry-After` is surfaced on the error.
- `ApiError.retryAfter` parses both forms of `Retry-After`, seconds and HTTP date. The spec
  declares the header (ADR-0018), so the generated types know it too.
- The signal TanStack Query passes is combined with the timeout through `AbortSignal.any`.
- Each body is read once and inspected by status, the lesson of the ky body-read pitfall in
  `docs/handoff.md`.

Unit tests cover the hand-written layer with a fake `fetch`: header assembly, retry and no-retry
cases, `Retry-After` parsing, problem decoding, abort and timeout.

openapi-typescript, the TypeScript 5.9.3 pin in `packages/openapi` and its Renovate rule, and ky
are removed. Spike S2 of the plan runs orval 8.44.0 against the first generated spec before the
switch; if it fails (OpenAPI 3.1 handling, non-null arrays, the named scalars,
`exactOptionalPropertyTypes`), the fallback is hey-api with the `@typescript/typescript6` alias
confined to `packages/api-client`, recorded as an amendment to this ADR.

### Consequences

- Good: adding an endpoint needs no client code; the spec regeneration brings the function and
  its types.
- Good: no generator in the repository needs the TypeScript compiler API, so TypeScript 7.0.2 is
  the only TypeScript.
- Good: request policy (IDs, retries, timeouts, problems) lives in four small tested files.
- Bad: orval releases often; bumps change generated code that someone must review.
- Bad: the console's imports change from one client object to per-tag functions, a one-time
  migration.
- Bad: the mutator is called by generated code, so its signature follows orval's conventions,
  not ours.

## More Information

- [ADR-0008](0008-openapi-first-generated-server-and-client.md): the hand-written client this
  record replaces, with ADR-0018.
- [ADR-0003](0003-go-api-typescript-react-console.md): the ky choice this record replaces.
- [ADR-0018](0018-rest-spec-generated-from-go-with-huma.md): the generated spec the client reads.
- `docs/plans/ddd-transports/plan.md`, section 15; `docs/plans/ddd-transports/research.md`,
  section (c), the compiler API scan of each generator.
- orval output modes: https://orval.dev/docs/reference/configuration/output
- orval 8.44.0: https://www.npmjs.com/package/orval/v/8.44.0
- TypeScript 7.0 and its missing API: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
