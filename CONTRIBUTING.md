# Contributing

Thank you for helping. This page covers the toolchain, the commands, the commit convention
and the one workflow everybody uses: adding an endpoint.

## Prerequisites

| Tool                                                                                    | Version      | Notes                                                                                                                                                                                   |
| --------------------------------------------------------------------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Docker with Compose v2                                                                  | any recent   | Used by the local stack, the integration tests and the image smoke test. Docker 25+ needs the seccomp override already set in the compose file.                                         |
| Node.js                                                                                 | 24 (LTS)     | `engines` in package.json.                                                                                                                                                              |
| pnpm                                                                                    | 12           | `packageManager` in package.json pins the exact version; pnpm switches to it on its own. Install with `npm install --global pnpm@12` or your version manager. Corepack is not required. |
| Go                                                                                      | 1.27.3       | `go.mod` pins it; with `GOTOOLCHAIN=auto` (the default) Go downloads the right toolchain.                                                                                               |
| C compiler                                                                              | gcc or clang | The TigerBeetle Go client uses cgo. Xcode command line tools on macOS, `build-essential` on Debian.                                                                                     |
| just, golangci-lint, sqlc, govulncheck, oasdiff, buf, protoc-gen-go, protoc-gen-go-grpc | pinned       | Installed by `scripts/install-tools.sh` (later `just tools-install`) into `$(go env GOPATH)/bin`. Add that directory to your PATH.                                                      |

macOS note: the TigerBeetle Go client v0.17.10 is required to link on macOS 27; see ADR-0009.

## Setup

```sh
pnpm install              # workspace dependencies and the git hooks (vp config, ADR-0012)
scripts/install-tools.sh  # just, golangci-lint, sqlc, govulncheck, oasdiff, buf and the protoc plugins, pinned
just compose-up           # TigerBeetle, Postgres, API and console in Docker
```

## Commands

Every task is a `just` recipe at the root (ADR-0016); `just --list` prints them with a line each.
Go recipes call the Go toolchain directly. TypeScript recipes call Turborepo, which orders and
caches the TypeScript packages. Git hooks and CI run the same recipes.

| Command                                                            | What it does                                                                                                                                                                                                                                                                                                                  |
| ------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `just dev`                                                         | Dev servers side by side: `go run ./cmd/api` and the Vite console. Needs TigerBeetle and Postgres (the stack, or your own) and the variables from `.env.example`.                                                                                                                                                             |
| `just build`                                                       | `go build` for the API (`build-go`) and `vp build` for the console (`build-ts`).                                                                                                                                                                                                                                              |
| `just lint`                                                        | golangci-lint for Go (`lint-go`); `vp check` over the repository, each package's lint and Redocly for the spec (`lint-ts`). `just lint-spec` runs Redocly alone.                                                                                                                                                              |
| `just format`                                                      | golangci-lint formatters (gofumpt, goimports) and `vp check --fix`, writing changes.                                                                                                                                                                                                                                          |
| `just typecheck`                                                   | `tsc --noEmit` for the console and the client.                                                                                                                                                                                                                                                                                |
| `just test`                                                        | Go unit tests with `-race` (`test-go`) and the TypeScript unit tests (`test-ts`).                                                                                                                                                                                                                                             |
| `just test-integration`                                            | Go tests behind the `integration` build tag; starts real Postgres and TigerBeetle with testcontainers.                                                                                                                                                                                                                        |
| `just generate`                                                    | sqlc, then the spec and the GraphQL schema from the Go registrations (`cmd/openapi`, `cmd/graphqlschema`), then the gqlgen executor, then the spec lint, the TypeScript client (orval), the console's GraphQL types and the route tree. `just generate-check` also fails when the result differs from the commit, as CI does. |
| `just vulncheck`                                                   | govulncheck over the Go module.                                                                                                                                                                                                                                                                                               |
| `just compose-up`, `compose-down`, `compose-logs`, `compose-reset` | Local stack lifecycle. `compose-reset` deletes the volumes.                                                                                                                                                                                                                                                                   |
| `just tools-install`                                               | Installs the pinned tools again.                                                                                                                                                                                                                                                                                              |
| `just versions-check`                                              | Verifies the TigerBeetle client in go.mod matches the server image in the compose file.                                                                                                                                                                                                                                       |

The TypeScript toolchain is Vite+ (ADR-0011), pinned in `pnpm-workspace.yaml`. Run its CLI through
pnpm (`pnpm exec vp check` at the root covers the whole repository; `just lint-ts` runs it).
`vp dev`, `vp build` and `vp test` are the built-in tools; `vp run <task>` would run a package
script, but prefer the just recipes above, which go through Turborepo.

Go-only recipes end in `-go` (`just lint-go`, `just test-go`), and plain `go test ./...` inside
`apps/api` works as usual.

## Commits and pull requests

- Commits follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
  `feat(api): add ledger endpoints`, `fix(console): handle 503 from readiness`, `docs: ...`,
  `chore(deps): ...`. The `commit-msg` hook enforces it.
- The `pre-commit` hook runs `vp staged` (the `staged` block in `vite.config.ts`): `vp check --fix`
  on web files, `just fmt-go-files` and `just lint-go` on Go files, `just lint-spec` when the spec
  changed. Fix what it
  reports rather than skipping it. `VP_GIT_HOOKS=0` skips hooks for one command.
- Keep commits small; each one should leave `just lint`, `just test` and `just build` green.
- Generated code is committed. CI fails when `just generate` produces a diff (`just generate-check`).
- No emojis anywhere: code, comments, commit messages, documentation.
- SQL keywords in UPPERCASE, identifiers in lowercase.

## Adding an endpoint

The Go code is the source of both contracts: the REST spec and its TypeScript client (ADR-0018),
and the GraphQL schema the console uses (ADR-0020).

1. Find the module that owns the data (`ledgers` or `bookkeeping` in `apps/api/internal/modules`).
   Implement the use case in its `application` package, on a command or query type, and export
   its method on the interface in `usecases.go`. Use the module's domain ports, adding to a port
   or its infrastructure only when the use case needs something new; ask another module through
   an interface in `application/ports.go` that `internal/bootstrap` wires, never by importing it.
   Keep rules, validation included, in the module's `domain` or in `internal/kernel/ledger` when
   two modules share them.
2. Register the operation in the module's `interfaces/rest/v1`, in the file of its tag:
   `restapi.Register` with the operationId, method, path, tag, summary, description and success
   response, and a handler from a typed input struct (path, query and header fields, `Body`) to
   an output struct; shared answer types go in `interfaces/dto`. Struct tags document constraints
   for the spec; Huma does not enforce them, the domain does. Return errors as Go errors;
   `internal/platform/problem` maps them to problem+json.
3. Add the operationId to the `registered` table in `internal/transport/rest/register_test.go`
   and raise the operation count in `cmd/openapi/main_test.go`.
4. When the console needs it, register the GraphQL side in the module's `interfaces/graphql`:
   `graphqlschema.Query` or `Mutation` for a root field, `Field` for a field on `Ledger`, with a
   resolver method that calls the same use case. Arguments and results are plain Go structs
   (the `dto` types); a mutation takes one `input` struct, and a money write has
   `IdempotencyKey` in it. The generator reads descriptions from `doc` tags and nullability from
   pointers.
5. Run `just generate`: `cmd/openapi` rewrites `packages/openapi/openapi.yaml`, orval
   regenerates `packages/api-client/src/gen`, `cmd/graphqlschema` rewrites
   `packages/graphql/schema.graphql`, gqlgen regenerates `internal/transport/graphql/exec`, and
   graphql-codegen the console's `src/graphql/gen`. A gqlgen error naming a missing resolver
   means the module's resolver type lacks the method. `just lint-spec` lints the spec and
   `just spec-diff` lists what changed against `main`; CI fails on a breaking change.
6. Add tests: table-driven unit tests of the use case with hand-written fakes of the ports, an
   `httptest` test of the handler with fakes of the use cases, and an integration test (build tag
   `integration`) when infrastructure changed.
7. In the console, write the document in the feature's `api.ts` with `graphql()`, call it with
   `request()` and return the REST type through `present()`.
8. Commit the Go change, the regenerated spec, schema and clients, and the tests together.

## Database changes

Add a goose migration in `apps/api/migrations` named `NNNNN_short_description.sql` with
`-- +goose Up` and `-- +goose Down` sections and a header comment naming the module that owns
the table, and the matching queries in that module's `infrastructure/postgres/queries`.
Run `just generate` to refresh the sqlc output. Migrations run at API startup when
`MIGRATE_ON_START=true` (the compose stack sets it).

## Design decisions

Anything that changes a decision recorded in `docs/adr` needs a new ADR that supersedes the old
one (see ADR-0001). Open an issue first for the larger ones.
