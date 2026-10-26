# 18. REST spec generated from Go with Huma v2

Status: Accepted, amended by [ADR-0025](0025-problem-details-per-rfc-9457.md), [ADR-0031](0031-authenticated-transports-and-strict-request-bodies.md)
Date: 2026-10-25

## Context and Problem Statement

ADR-0008 made `packages/openapi/openapi.yaml` the hand-written contract, with oapi-codegen
generating a strict Go server and openapi-typescript the client types. It considered code-first
and rejected it: code-first "ties the contract to Go idioms and makes the spec a build output that
reviewers rarely read".

The API is about to serve the same use cases over five transports (ADR-0017), and the GraphQL
schema will be generated from Go registrations (ADR-0020). Keeping REST hand-written would leave
two contracts with two sources of truth: YAML for REST, Go for everything else. The owner wants
the Go code to be the source of the REST contract too, the way springdoc derives it from Spring
controllers, with no comment annotations that can drift from the handlers.

This record supersedes ADR-0008. The client half of ADR-0008 is replaced by ADR-0019.

## Decision Drivers

- REST behaviour does not change for third parties: same paths, parameters, schemas, status
  codes, problem types, operation IDs and tags. A tool proves it, not a reviewer.
- The spec stays a committed, reviewed, linted file; it is never edited by hand.
- Typed input and output structs per operation, so the spec cannot disagree with the handlers.
- `net/http.ServeMux` keeps routing; the standard-library middleware stays.
- Validation behaves the same on every transport.
- Writing the spec needs no server, no TigerBeetle and no Postgres.

## Considered Options

1. Huma v2 on its `humago` adapter, with a command that writes the spec (chosen).
2. swag-style comment annotations over plain handlers.
3. A hand-written spec builder in the repository.
4. Keep the spec hand-written and oapi-codegen (ADR-0008).

Option 2 is what drifts: comments are not checked against the code. Option 3 rebuilds what Huma
already does (schemas from types, parameters from tags) and must be maintained. Option 4 keeps
two sources of truth once GraphQL comes from Go.

## Decision Outcome

Chosen option: 1, Huma v2.39.1 (MIT, no cgo). It is the one accepted exception to "no web
framework" in `AGENTS.md` and to the "standard library only for HTTP" choice of ADR-0003, and it
stays narrow:

- `humago.New` registers `METHOD path` patterns on the existing `ServeMux`; Huma does no routing
  of its own. Request ID, access log, panic recovery and the 404 and 405 problems stay the
  standard-library middleware they are today.
- depguard allows Huma only in `internal/platform/restapi`, `internal/platform/scalar`, module
  `interfaces/rest` folders, `internal/transport/rest` and `cmd/openapi`.

How it is used:

- `restapi.NewAPI(mux, version)` builds the API every caller shares; `restapi.Register` adds one
  operation and sends every handler error through the transport-neutral problem mapping, so no
  TigerBeetle or Postgres text reaches a client.
- Each operation has typed input and output structs. Struct tags carry the descriptions
  (`doc`), examples, formats and constraints that become the spec.
- Huma reads three settings from package-level variables: `huma.NewError`,
  `huma.NewErrorWithContext` (the error model) and `huma.DefaultArrayNullable` (true by default,
  which would make every slice nullable in the spec). `AGENTS.md` forbids globals and `init()`.
  They are assigned in one function guarded by a `sync.Once`, called by `restapi.NewAPI`, and the
  values never change after the first call. That `sync.Once` is the only package-level variable
  of the REST code; `init()` is still not used.
- The error model renders RFC 9457 problems with the members of today's `Problem` schema and no
  `errors` array.
- Validation stays in the domain, for good. Every operation is registered with
  `SkipValidateParams` and `SkipValidateBody`: five transports share the domain's validation, and
  Huma's would apply to REST only and answer differently (422 where REST answers 400). The tags
  still document constraints (`maximum: 100` on the page limit, `maxItems: 64` on postings) as
  the committed spec does today; the domain keeps clamping where it clamps. Rejecting instead of
  clamping would be a domain change, made for every transport at once.
- Errors Huma still raises itself (an unparsable body, a missing required body) map to
  `400 /problems/invalid-request`, or to `missing-idempotency-key` when that header is missing.
- A small middleware keeps today's behaviour of decoding a body as JSON whatever its
  `Content-Type`; a stricter answer (415) waits for a new API minor version.

The spec as an output:

- `cmd/openapi` registers every operation with nil use cases on a fresh mux and writes
  `packages/openapi/openapi.yaml`. It opens no socket. depguard keeps drivers and the cgo
  TigerBeetle client out of it, and `just generate` builds it with `CGO_ENABLED=0`, which fails
  if a cgo import slips in. A test runs it twice and compares the bytes.
- The file starts with a "generated, do not edit" comment. `just generate-check` fails CI when
  the committed file differs from what the code writes. Redocly keeps linting it with
  `recommended-strict`.
- Spec features without a direct Huma equivalent are kept by hand-written schemas: the 14 named
  scalar components (types implementing `huma.SchemaProvider`), `allOf` on `Account`,
  `PostingRecord` and `WriteRequestDetail`, `propertyNames` on `Metadata`, the shared `Problem`
  response as every operation's `default`, `security: []`, and the 503 body of `/readyz`. If
  Huma adds `additionalProperties: false`, `cmd/openapi` removes it: with validation off it would
  document a rejection that never happens.
- The running API serves neither the spec nor a docs page. The committed file is the contract,
  and Huma's docs page loads JavaScript from a CDN.

Proof that REST did not change: at the switch, oasdiff v1.34.1 (pinned in
`scripts/install-tools.sh`) compares the hand-written spec with the generated one (`breaking`,
`changelog` and `diff`), and every remaining difference is explained in
`docs/plans/ddd-transports/oasdiff-phase2.md`. An end-to-end test and a replay test of fixed
requests compare statuses, headers and problem bodies. From then on CI runs
`oasdiff breaking --fail-on ERR` against the base branch on every pull request. Declaring the
existing `X-Request-Id` and `Retry-After` headers is a separate additive commit after the parity
commit.

The answer to ADR-0008's objection: the spec is still committed, so every API change shows a
YAML hunk in the same pull request, and CI names breaking changes instead of hoping a reviewer
spots them. The contract is not tied to Go idioms: wire names come from explicit `json` tags,
components keep their names through the scalar types, and oasdiff fails the build when Go
changes the contract. Adding an endpoint now starts from its input and output structs; reviewers
read the spec hunk first, as ADR-0008 already asked.

### Consequences

- Good: one source of truth for REST, and later GraphQL, in Go; handlers and spec cannot drift.
- Good: oapi-codegen, its runtime, `httpapi/gen` (5,620 generated lines) and the TypeScript 5.9
  pin of `packages/openapi` go away.
- Good: breaking changes are named by CI on every pull request.
- Bad: oapi-codegen's strict server made an undocumented status code a compile error. Huma
  checks success statuses through the output structs, but error statuses come from the problem
  mapping and are documented once, through the shared `default` response.
- Bad: a framework dependency with package-level settings; a Huma upgrade can change the
  generated spec, which `generate-check` and oasdiff catch but someone must read.
- Bad: the first generated file reorders the YAML, one large mechanical diff.
- Bad: several schemas are hand-written `SchemaProvider` code that must match the domain types.

## More Information

- [ADR-0008](0008-openapi-first-generated-server-and-client.md): spec-first, superseded.
- [ADR-0003](0003-go-api-typescript-react-console.md): the standard-library rule this record
  makes one exception to.
- [ADR-0017](0017-bounded-context-modules-and-dependency-rules.md): modules register their
  operations from their `interfaces/rest` folders.
- [ADR-0019](0019-rest-client-generated-with-orval.md): the client generated from this spec.
- `docs/plans/ddd-transports/plan.md`, section 7.
- Huma: https://huma.rocks/ and https://github.com/danielgtaylor/huma/releases/tag/v2.39.1
- oasdiff: https://github.com/oasdiff/oasdiff/releases/tag/v1.34.1
- RFC 9457, Problem Details for HTTP APIs: https://www.rfc-editor.org/rfc/rfc9457
