# 8. OpenAPI-first with generated server and client

Status: Superseded by [ADR-0018](0018-rest-spec-generated-from-go-with-huma.md)
Date: 2026-10-24

## Context and Problem Statement

The platform exposes a ledger API over TigerBeetle (ADR-0004) consumed by a web console we write
ourselves and, eventually, by third-party integrators. Two codebases in two languages must agree
on every path, status code, field name and error shape: the Go service in `apps/api` and the
TypeScript client in `packages/api-client` that the console in `apps/console` uses. Hand-writing
both sides invites drift: a renamed field in a Go struct silently breaks the console, and the
documentation we would publish becomes a third copy that nobody keeps current.

The question is which artifact is the contract, and how the other artifacts are kept in step with
it so that drift is caught by a machine rather than by a user of the console.

## Decision Drivers

- One source of truth for the API, readable by people and by tooling.
- Go server and TypeScript client must be derived from it, not maintained by hand.
- Drift between the spec and the generated code must fail CI, not surprise a reviewer.
- Error responses must have one shape across every endpoint (RFC 9457 problem details), so the
  client can handle them generically.
- Stay on the Go standard library where possible (`net/http` routing, no framework).
- Keep the generated code out of linting and formatting so tool upgrades do not create noise.
- Engineers new to the codebase should be able to add an endpoint by following a short, fixed
  sequence of steps.

## Considered Options

1. Spec-first: author `openapi.yaml`, generate the Go server interface and the TypeScript types
   from it (chosen).
2. Code-first: write Go handlers, derive the spec from comments or struct tags (swag-style
   annotations or a framework such as huma), then generate the TypeScript client from that.
3. Hand-written handlers and client, with the spec kept as documentation only.
4. A different IDL such as Protobuf with gRPC or Connect, with JSON transcoding for the browser.

Code-first ties the contract to Go idioms and makes the spec a build output that reviewers rarely
read. Hand-written everything is the drift scenario described above. gRPC gives strong typing but
is a poor fit for a REST-shaped ledger API that integrators expect to call with curl.

## Decision Outcome

Option 1. `packages/openapi/openapi.yaml` (OpenAPI 3.1.0) is the contract. Everything that depends
on the API shape is generated from it and committed.

Spec and lint:

- The spec is linted by Redocly CLI 2.71.0 with the `recommended-strict` ruleset, configured in
  `packages/openapi/redocly.yaml`. `operation-4xx-response` is switched off because every
  operation documents errors through one shared `default` response instead of listing each 4xx;
  `no-unused-components`, `operation-operationId-unique` and `tag-description` are errors.
- `operationId` names (`getLiveness`, `getReadiness`, `getInfo`) become the Go method names and
  the TypeScript operation keys, so they are chosen with care and never renamed casually.

Go server:

- oapi-codegen v2.8.1 is declared with the `tool` directive in `apps/api/go.mod`, so `go tool
  oapi-codegen` runs the pinned version without a separate install. v2.8.0 added OpenAPI 3.1
  support, which is why the floor is there.
- `apps/api/oapi-codegen.yaml` enables `std-http-server`, `strict-server` and `models`, writing
  `apps/api/internal/adapters/httpapi/gen/api.gen.go`. `std-http-server` registers routes on
  `net/http.ServeMux` with Go 1.22+ method patterns such as `GET /v1/_info`, so no router
  dependency is needed. `strict-server` means a handler receives a typed request object and
  returns a typed response object (for example `gen.GetReadiness503JSONResponse`); handler code
  never touches `http.ResponseWriter`, which keeps status codes and bodies within what the spec
  allows.
- `apps/api/internal/adapters/httpapi/handlers.go` implements `gen.StrictServerInterface` by
  calling `app` use cases; `router.go` wires the generated handler onto a `ServeMux` and wraps it
  with request ID, access log, panic recovery and problem+json conversion of unknown routes.

TypeScript client:

- openapi-typescript 7.13.0 reads the same `redocly.yaml` (the `x-openapi-ts` block) and writes
  `packages/api-client/src/schema.d.ts`. `packages/openapi` pins TypeScript 5.9.3 because
  openapi-typescript 7 needs the TypeScript 5 compiler API; the rest of the workspace uses 7.0.2.
- `packages/api-client/src/index.ts` is hand-written: a thin ky 2.2.0 wrapper that sets
  `X-Request-Id`, retries only idempotent methods on 502, 503 and 504, and turns
  `application/problem+json` answers into `ApiError`. It imports the generated types and exposes
  one method per operation. The console calls only this client.

Generated code is committed and excluded from lint:

- Committing the output means `go build` and `tsc` work from a fresh clone without running
  generators, and reviewers see the effect of a spec change in the same diff.
- golangci-lint uses `exclusions.generated: strict` in `apps/api/.golangci.yml`, which skips
  files carrying the `Code generated ... DO NOT EDIT` header. The root
  `vite.config.ts` excludes `packages/api-client/src/schema.d.ts` from formatting and lint.
- The `codegen-drift` job in `.github/workflows/ci.yml` runs `pnpm generate` and fails on any
  resulting diff (`git add -A && git diff --cached --exit-code`). The `openapi` job runs the
  Redocly lint on its own so a spec error is reported by name.
- Turborepo orders the work: in `turbo.json`, `api#generate` depends on
  `@tigerbeetle-platform/openapi#generate`, and build, lint, typecheck and test depend on
  `generate`, so `pnpm generate` at the root regenerates Go, TypeScript and sqlc output in order.

Error responses are part of the contract:

- `components/responses/Problem` and `components/schemas/Problem` in the spec declare RFC 9457
  problem details with the `application/problem+json` media type; every operation references it as
  its `default` response.
- `apps/api/internal/adapters/httpapi/problem.go` maps the typed domain errors
  (`ledger.ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid`, `ErrUnavailable`) to status, title and a
  stable `type` URI under `/problems/`. Binding errors from the generated layer go through
  `RequestErrorHandlerFunc` and `ErrorHandlerFunc` in `router.go`, and the mux's text 404 and 405
  are rewritten, so no client ever sees a non-problem error body or raw driver text.

Adding an endpoint follows a fixed sequence:

1. Edit `packages/openapi/openapi.yaml`: path, `operationId`, request and response schemas,
   `default: $ref: "#/components/responses/Problem"`.
2. Run `pnpm generate` at the root. `gen.StrictServerInterface` gains a method and the build
   fails until it is implemented; `schema.d.ts` gains the new types.
3. Add or extend the use case in `apps/api/internal/app` and, if needed, a port in `ports.go`
   and its adapter implementation.
4. Implement the method in `apps/api/internal/adapters/httpapi/handlers.go`, mapping domain
   values to generated models.
5. Add a method to `packages/api-client/src/index.ts` and use it from the console.
6. Add tests: a handler test in `httpapi_test.go` with fakes for the ports, use case tests in
   `app`, and an integration test behind the `integration` build tag when an adapter changed.
7. Commit the spec, the generated files and the implementation together.

### Consequences

- Good: one file defines the API; Go and TypeScript cannot disagree with it without CI failing.
- Good: strict handlers make an undocumented status code a compile error rather than a runtime
  surprise.
- Good: no HTTP framework; routing is `net/http` and the only runtime dependency the generated
  code adds is the tiny oapi-codegen runtime.
- Good: the console and third-party clients share the same typed error handling.
- Bad: every API change touches at least three places (spec, Go, client) and regenerated files
  can make diffs large; reviewers should read the spec hunk first.
- Bad: two codegen toolchains (Go tool directive, pnpm package) must stay at matching, pinned
  versions; Renovate and `scripts/check-versions.sh` carry that load.
- Bad: `packages/openapi` carries its own TypeScript 5 pin until openapi-typescript supports the
  TypeScript 7 compiler API.
- Bad: oapi-codegen's OpenAPI 3.1 support is recent; some keywords (for example `const`) may be
  generated less precisely than their 3.0 equivalents, which is checked by reading the output.

## More Information

- [ADR-0003](0003-go-api-typescript-react-console.md): the standard-library routing and the ky
  client that the generated code plugs into.
- [ADR-0004](0004-ledger-api-surface.md): the API surface that this spec describes,
  including string amounts and hex-encoded 128-bit IDs.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md): the `Idempotency-Key` header every write
  operation declares.
- Spec and lint: `packages/openapi/openapi.yaml`, `packages/openapi/redocly.yaml`,
  `packages/openapi/README.md`.
- Go generation: `apps/api/oapi-codegen.yaml`, `apps/api/go.mod` (tool directive),
  `apps/api/internal/adapters/httpapi/{gen,handlers.go,router.go,problem.go}`.
- TypeScript generation and client: `packages/api-client/src/{schema.d.ts,index.ts}`,
  `apps/console/src/api.ts`.
- Enforcement: `.github/workflows/ci.yml` (jobs `codegen-drift`, `openapi`), `turbo.json`,
  `apps/api/.golangci.yml`, `vite.config.ts`.
- RFC 9457, Problem Details for HTTP APIs: https://www.rfc-editor.org/rfc/rfc9457
