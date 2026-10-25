# Contributing

Thank you for helping. This page covers the toolchain, the root commands, the commit convention
and the one workflow everybody uses: adding an endpoint.

## Prerequisites

| Tool                             | Version      | Notes                                                                                                                                                                                   |
| -------------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Docker with Compose v2           | any recent   | Used by the local stack, the integration tests and the image smoke test. Docker 25+ needs the seccomp override already set in the compose file.                                         |
| Node.js                          | 24 (LTS)     | `engines` in package.json.                                                                                                                                                              |
| pnpm                             | 12           | `packageManager` in package.json pins the exact version; pnpm switches to it on its own. Install with `npm install --global pnpm@12` or your version manager. Corepack is not required. |
| Go                               | 1.27.3       | `go.mod` pins it; with `GOTOOLCHAIN=auto` (the default) Go downloads the right toolchain.                                                                                               |
| C compiler                       | gcc or clang | The TigerBeetle Go client uses cgo. Xcode command line tools on macOS, `build-essential` on Debian.                                                                                     |
| golangci-lint, sqlc, govulncheck | pinned       | Installed by `pnpm tools:install` into `$(go env GOPATH)/bin`. Add that directory to your PATH.                                                                                         |

macOS note: the TigerBeetle Go client v0.17.10 is required to link on macOS 27; see ADR-0009.

## Setup

```sh
pnpm install          # workspace dependencies and the git hooks (vp config, ADR-0012)
pnpm tools:install    # golangci-lint, sqlc, govulncheck at the pinned versions
pnpm stack:up         # TigerBeetle, Postgres, API and console in Docker
```

## Root commands

All of them run through Turborepo and apply to every package that defines the task.

| Command                                                                   | What it does                                                                                                                                          |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pnpm dev`                                                                | Dev servers: `go run ./cmd/api` and the Vite console. Needs the stack's TigerBeetle and Postgres (or your own) and the variables from `.env.example`. |
| `pnpm build`                                                              | `go build` for the API, `vp build` (Vite+) for the console.                                                                                           |
| `pnpm lint`                                                               | golangci-lint for Go, `vp check` (Oxfmt, Oxlint, type-aware checks) for TypeScript, Redocly for the spec.                                             |
| `pnpm format`                                                             | golangci-lint formatters (gofumpt, goimports) and `vp check --fix`, writing changes.                                                                  |
| `pnpm typecheck`                                                          | `tsc --noEmit` for the console and the client.                                                                                                        |
| `pnpm test`                                                               | Go unit tests with `-race`.                                                                                                                           |
| `pnpm test:integration`                                                   | Go tests behind the `integration` build tag; starts real Postgres and TigerBeetle with testcontainers.                                                |
| `pnpm generate`                                                           | Lints the spec, then regenerates the Go server (oapi-codegen), the SQL accessors (sqlc) and the TypeScript types (openapi-typescript).                |
| `pnpm stack:up`, `pnpm stack:down`, `pnpm stack:logs`, `pnpm stack:reset` | Local stack lifecycle. `stack:reset` deletes the volumes.                                                                                             |
| `pnpm versions:check`                                                     | Verifies the TigerBeetle client in go.mod matches the server image in the compose file.                                                               |

The TypeScript toolchain is Vite+ (ADR-0011), pinned in `pnpm-workspace.yaml`. Run its CLI through
pnpm (`pnpm exec vp check` at the root covers the whole repository, as CI does). `vp dev`,
`vp build` and `vp test` are the built-in tools; `vp run <task>` would run a package script, but
Turborepo stays the task runner, so prefer the `pnpm` commands above.

Go-only shortcuts live in `apps/api/package.json` (`pnpm --filter api vulncheck`, for example),
and plain `go test ./...` inside `apps/api` works as usual.

## Commits and pull requests

- Commits follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
  `feat(api): add ledger endpoints`, `fix(console): handle 503 from readiness`, `docs: ...`,
  `chore(deps): ...`. The `commit-msg` hook enforces it.
- The `pre-commit` hook runs `vp staged` (the `staged` block in `vite.config.ts`): `vp check --fix`
  on web files, `golangci-lint fmt` and lint on Go files, Redocly when the spec changed. Fix what it
  reports rather than skipping it. `VP_GIT_HOOKS=0` skips hooks for one command.
- Keep commits small; each one should leave `pnpm lint`, `pnpm test` and `pnpm build` green.
- Generated code is committed. CI fails when `pnpm generate` produces a diff.
- No emojis anywhere: code, comments, commit messages, documentation.
- SQL keywords in UPPERCASE, identifiers in lowercase.

## Adding an endpoint

1. Edit `packages/openapi/openapi.yaml`: path, operationId, request and response schemas, and
   the `default` problem+json response. Run `pnpm --filter @tigerbeetle-platform/openapi lint`.
2. Run `pnpm generate`. This regenerates `apps/api/internal/adapters/httpapi/gen` and
   `packages/api-client/src/schema.d.ts`; the Go build now fails until the new method of
   `gen.StrictServerInterface` exists.
3. Implement the use case in `apps/api/internal/app` against the ports in `ports.go`, adding to
   a port or adapter only when the use case needs something new. Keep domain rules in
   `internal/domain/ledger`.
4. Implement the handler in `apps/api/internal/adapters/httpapi/handlers.go`: translate the
   request object, call the use case, return a generated response object. Errors are returned as
   Go errors; `problem.go` maps them to problem+json.
5. Add tests: table-driven unit tests next to the code with in-memory fakes for the ports, an
   `httptest` test for the handler, and an integration test (build tag `integration`) when an
   adapter changed.
6. Wrap the client method in `packages/api-client/src/index.ts` if the console needs it.
7. Commit the spec, the generated files, the implementation and the tests together.

## Database changes

Add a goose migration in `apps/api/migrations` named `NNNNN_short_description.sql` with
`-- +goose Up` and `-- +goose Down` sections, and the matching queries in `apps/api/queries`.
Run `pnpm generate` to refresh the sqlc output. Migrations run at API startup when
`MIGRATE_ON_START=true` (the compose stack sets it).

## Design decisions

Anything that changes a decision recorded in `docs/adr` needs a new ADR that supersedes the old
one (see ADR-0001). Open an issue first for the larger ones.
