# AGENTS.md

Conventions for coding agents working in this repository. Humans follow the same rules; this file
is the condensed version. The reasoning behind each rule lives in docs/adr. The current state of
the work, the pitfalls already hit and the suggested next steps are in docs/handoff.md.

## Project

- tigerbeetle-platform: a Go API in front of TigerBeetle with Postgres for metadata, a React
  console that only talks to the API, and later a posting-rule DSL. Pre-alpha.
- TigerBeetle is the only source of truth for money. Postgres never stores a balance.
- Describe the API as "Formance-style", never "Formance-compatible".

## Layout

- `apps/api`: Go module `github.com/wilfriedago/tigerbeetle-platform/apps/api`.
  - `cmd/api/main.go`: entrypoint, all wiring by hand. No DI framework, no globals, no `init()`.
  - `internal/domain/ledger`: pure types and rules, no I/O imports.
  - `internal/app`: use cases; `ports.go` declares the interfaces they need.
  - `internal/adapters/tigerbeetle`, `postgres`, `httpapi`: implement the ports. Generated code
    is in `gen` subpackages, committed, excluded from lint, never edited by hand.
  - `internal/platform`: config, logging, HTTP server lifecycle.
  - `migrations/` (goose, embedded), `queries/` (sqlc), `sqlc.yaml`, `oapi-codegen.yaml`.
- `packages/openapi/openapi.yaml` is the contract. `packages/api-client` is generated types plus a
  ky client. `apps/console` is the React console. `deploy/compose` is the local stack.

## Dependency rules (enforced by golangci-lint depguard)

- `domain` imports nothing from `internal`.
- `app` imports only `domain`.
- Adapters import `app` and `domain`, never each other.
- No packages named `utils`, `common` or `helpers`. Name packages by what they provide.

## Go rules

- Standard library first: `net/http` with Go 1.22+ patterns, `log/slog` JSON, `testing` with
  `go-cmp`. No web framework, no assertion library.
- `context.Context` is the first parameter of anything doing I/O.
- Errors: typed domain errors (`ledger.ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid`,
  `ErrUnavailable`), wrapped with `%w`, checked with `errors.Is` and `errors.As`. The HTTP adapter
  maps them to RFC 9457 `application/problem+json`. Never leak TigerBeetle or Postgres error text
  to clients.
- `GET /healthz` is liveness only. `GET /readyz` probes Postgres and TigerBeetle with a short
  timeout and reports each one.
- Tests: table-driven unit tests next to the code; integration tests behind the `integration`
  build tag with testcontainers; everything runs with `-race`.
- Format with gofumpt and goimports (`pnpm format`). Lint with `pnpm lint` before committing.
- Explain non-obvious choices in a short comment or an ADR, not in chat.

## TigerBeetle rules

- IDs come from the client's `ID()` helper (time-ordered), never from hashing. Convert with
  `adapters/tigerbeetle/id.go` when storing in a uuid column (big-endian).
- Source is the debit account, destination the credit account, balance is credits minus debits.
- One API transaction is one linked chain; `user_data_128` carries the transaction ID; `code`
  carries the business event type.
- The Go client version must not exceed the server image version. Bump both together;
  `scripts/check-versions.sh` enforces it.

## Workflow

- Adding an endpoint: edit the spec, run `pnpm generate`, implement the handler and use case,
  add tests. Commit all of it together.
- Conventional Commits, small commits, each leaving `pnpm lint`, `pnpm test`, `pnpm build` green.
- `pnpm generate` must produce no diff on a clean tree (CI checks it).
- Pin everything: Go modules, npm packages, Docker tags, GitHub Actions by SHA, tool binaries in
  `scripts/install-tools.sh`.
- No emojis anywhere. SQL keywords uppercase, identifiers lowercase. `.editorconfig` applies.
- Out of scope until decided otherwise: authentication, release pipelines, Kubernetes.
- When instructions conflict or a tool behaves unexpectedly, stop and ask rather than guess.
