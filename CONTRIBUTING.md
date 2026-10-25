# Contributing

Thank you for helping. This page covers the toolchain, the commands, the commit convention
and the one workflow everybody uses: adding an endpoint.

## Prerequisites

| Tool                                            | Version      | Notes                                                                                                                                                                                   |
| ----------------------------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Docker with Compose v2                          | any recent   | Used by the local stack, the integration tests and the image smoke test. Docker 25+ needs the seccomp override already set in the compose file.                                         |
| Node.js                                         | 24 (LTS)     | `engines` in package.json.                                                                                                                                                              |
| pnpm                                            | 12           | `packageManager` in package.json pins the exact version; pnpm switches to it on its own. Install with `npm install --global pnpm@12` or your version manager. Corepack is not required. |
| Go                                              | 1.27.3       | `go.mod` pins it; with `GOTOOLCHAIN=auto` (the default) Go downloads the right toolchain.                                                                                               |
| C compiler                                      | gcc or clang | The TigerBeetle Go client uses cgo. Xcode command line tools on macOS, `build-essential` on Debian.                                                                                     |
| just, golangci-lint, sqlc, govulncheck, oasdiff | pinned       | Installed by `scripts/install-tools.sh` (later `just tools-install`) into `$(go env GOPATH)/bin`. Add that directory to your PATH.                                                      |

macOS note: the TigerBeetle Go client v0.17.10 is required to link on macOS 27; see ADR-0009.

## Setup

```sh
pnpm install              # workspace dependencies and the git hooks (vp config, ADR-0012)
scripts/install-tools.sh  # just, golangci-lint, sqlc, govulncheck and oasdiff at the pinned versions
just compose-up           # TigerBeetle, Postgres, API and console in Docker
```

## Commands

Every task is a `just` recipe at the root (ADR-0016); `just --list` prints them with a line each.
Go recipes call the Go toolchain directly. TypeScript recipes call Turborepo, which orders and
caches the TypeScript packages. Git hooks and CI run the same recipes.

| Command                                                            | What it does                                                                                                                                                                                                               |
| ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `just dev`                                                         | Dev servers side by side: `go run ./cmd/api` and the Vite console. Needs TigerBeetle and Postgres (the stack, or your own) and the variables from `.env.example`.                                                          |
| `just build`                                                       | `go build` for the API (`build-go`) and `vp build` for the console (`build-ts`).                                                                                                                                           |
| `just lint`                                                        | golangci-lint for Go (`lint-go`); `vp check` over the repository, each package's lint and Redocly for the spec (`lint-ts`). `just lint-spec` runs Redocly alone.                                                           |
| `just format`                                                      | golangci-lint formatters (gofumpt, goimports) and `vp check --fix`, writing changes.                                                                                                                                       |
| `just typecheck`                                                   | `tsc --noEmit` for the console and the client.                                                                                                                                                                             |
| `just test`                                                        | Go unit tests with `-race` (`test-go`) and the TypeScript unit tests (`test-ts`).                                                                                                                                          |
| `just test-integration`                                            | Go tests behind the `integration` build tag; starts real Postgres and TigerBeetle with testcontainers.                                                                                                                     |
| `just generate`                                                    | sqlc, then the spec from the Go registrations (`cmd/openapi`), then the spec lint, the TypeScript client (orval) and the route tree. `just generate-check` also fails when the result differs from the commit, as CI does. |
| `just vulncheck`                                                   | govulncheck over the Go module.                                                                                                                                                                                            |
| `just compose-up`, `compose-down`, `compose-logs`, `compose-reset` | Local stack lifecycle. `compose-reset` deletes the volumes.                                                                                                                                                                |
| `just tools-install`                                               | Installs the pinned tools again.                                                                                                                                                                                           |
| `just versions-check`                                              | Verifies the TigerBeetle client in go.mod matches the server image in the compose file.                                                                                                                                    |

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

The Go code is the source of the REST contract (ADR-0018): the spec and the TypeScript client are
generated from it.

1. Implement the use case in `apps/api/internal/app` against the ports in `ports.go`, adding to
   a port or adapter only when the use case needs something new. Keep domain rules, validation
   included, in `internal/kernel/ledger`.
2. Register the operation in `apps/api/internal/adapters/httpapi/v1`, in the file of its tag:
   `restapi.Register` with the operationId, method, path, tag, summary, description and success
   response, and a handler from a typed input struct (path, query and header fields, `Body`) to
   an output struct. Struct tags document constraints for the spec; Huma does not enforce them,
   the domain does. Return errors as Go errors; `internal/platform/problem` maps them to
   problem+json.
3. Add the operationId to the `registered` table in `v1_test.go` and raise the operation count
   in `cmd/openapi/main_test.go`.
4. Run `just generate`: `cmd/openapi` rewrites `packages/openapi/openapi.yaml` and orval
   regenerates `packages/api-client/src/gen`. `just lint-spec` lints the spec and
   `just spec-diff` lists what changed against `main`; CI fails on a breaking change.
5. Add tests: table-driven unit tests next to the code with in-memory fakes for the ports, an
   `httptest` test for the handler, and an integration test (build tag `integration`) when an
   adapter changed.
6. In the console, call the generated function from the feature's `api.ts`.
7. Commit the Go change, the regenerated spec and client, and the tests together.

## Database changes

Add a goose migration in `apps/api/migrations` named `NNNNN_short_description.sql` with
`-- +goose Up` and `-- +goose Down` sections, and the matching queries in `apps/api/queries`.
Run `just generate` to refresh the sqlc output. Migrations run at API startup when
`MIGRATE_ON_START=true` (the compose stack sets it).

## Design decisions

Anything that changes a decision recorded in `docs/adr` needs a new ADR that supersedes the old
one (see ADR-0001). Open an issue first for the larger ones.
