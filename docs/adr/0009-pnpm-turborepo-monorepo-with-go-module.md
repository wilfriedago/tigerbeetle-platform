# 9. pnpm and Turborepo monorepo with a Go module inside

Status: Accepted, amended by [ADR-0011](0011-vite-plus-toolchain-for-typescript.md), [ADR-0012](0012-vite-plus-git-hooks.md)
Date: 2026-10-24

## Context and Problem Statement

The platform is one product built from two languages: a Go API service (`apps/api`), a React
console (`apps/console`) and the TypeScript packages that sit between them (`packages/openapi`,
`packages/api-client`). The OpenAPI contract (ADR-0008) is consumed by both sides: oapi-codegen
produces the Go server interfaces, openapi-typescript produces the TypeScript types. A change to
the spec must regenerate both outputs, in the right order, before anything is built or tested.

We need one checkout, one install command and one task runner that understands this dependency
graph, without forcing the Go code to be built by Node tooling or the Node code to be built by Go
tooling. We also need every tool and dependency pinned so that a fresh clone, a CI runner and a
Renovate PR all run the same versions.

## Decision Drivers

- One command per verb at the root (`pnpm generate`, `pnpm build`, `pnpm test`), with
  cross-language ordering: Go and TypeScript generation both wait for the spec package.
- Go must keep resolving its own dependencies with `go.mod` and `go.sum`. pnpm must never become
  a dependency manager for Go.
- Reproducibility: exact versions everywhere, no floating ranges, no "latest", no tool that
  silently picks a version from the developer's machine.
- The TigerBeetle client and server must move together (a client newer than the cluster is
  rejected at connection time), so their bumps must be coupled, not merely coincident.
- Low ceremony for a contributor new to Go: install Node and Go, run two scripts, start working.

## Considered Options

1. pnpm workspace plus Turborepo, with `apps/api/package.json` as a thin shim whose scripts call
   the Go toolchain (chosen).
2. Two independent repositories (Go API, web), with the OpenAPI spec published as a package.
3. One repository, no orchestrator: a Makefile at the root calling `go` and `pnpm` by hand.
4. Nx or Bazel as the orchestrator.

Option 2 makes every spec change a two-repository, two-PR affair and loses the "generated code is
committed and current" check across languages. Option 3 works but recomputes everything on every
run and encodes the dependency graph in shell. Option 4 brings far more machinery than three
packages and one Go module justify.

## Decision Outcome

Chosen option: pnpm workspace plus Turborepo.

- `pnpm-workspace.yaml` lists `apps/*` and `packages/*`. Root `package.json` holds only the
  tooling used across the workspace (Turborepo 2.11.10, Biome 2.6.2, lefthook 2.2.2, commitlint
  21.2.4) and the `stack:*` scripts that wrap `deploy/compose/compose.yaml`.
- `apps/api/package.json` is a shim: no runtime dependencies, only scripts that call `go`,
  `go tool oapi-codegen`, `sqlc`, `golangci-lint` and `govulncheck`. Its single devDependency is
  `@tigerbeetle-platform/openapi` with `workspace:*`, which is how Turborepo learns that
  `api#generate` depends on `@tigerbeetle-platform/openapi#generate`.
- `turbo.json` declares explicit `inputs` and `outputs` for the `api#*` tasks (`**/*.go`,
  `go.mod`, `go.sum`, `migrations/**`, `queries/**`, `sqlc.yaml`, `oapi-codegen.yaml`). Without
  this, Turborepo would hash the whole package directory and cache hits would be rare; with it,
  the Go build and lint are cached like any other task. `test:integration` and `dev` are never
  cached.
- Go resolves its own dependencies. There is no `go.work` file: a workspace file is only useful
  once a second Go module exists, and adding it early would change how `go` commands resolve
  modules for no benefit. Revisit when a second module appears.
- Tool pinning, one mechanism per kind of tool:
  - oapi-codegen v2.8.1 is pinned with the `tool` directive in `apps/api/go.mod` and invoked as
    `go tool oapi-codegen`. It is a Go program whose version should track the module, and the
    `tool` directive (Go 1.24 and later) makes `go mod tidy` manage it. It is the only tool
    installed this way.
  - goose v3.29.0 is a library dependency, used from
    `apps/api/internal/adapters/postgres/migrate.go` to apply the embedded migrations at
    startup. Its CLI is not installed because it would drag ClickHouse, MySQL, MSSQL and other
    drivers into `go.mod`.
  - sqlc v1.31.1, golangci-lint v2.15.0 and govulncheck v1.9.0 are pinned in
    `scripts/install-tools.sh` (`pnpm tools:install`). sqlc and golangci-lint are release
    binaries rather than `go install` targets: both are large and built by their maintainers
    with a specific toolchain, and sqlc publishes no checksum file, so the script carries the
    sha256 of each archive. The pins carry `# renovate:` comments so Renovate bumps them.
- Version policy: everything is pinned to an exact version. `rangeStrategy: pin` in
  `renovate.json`, exact versions in every `package.json`, exact image tags in compose and
  Dockerfiles (Debian 13 "trixie" variants only), the Go version via `go 1.27.0` plus
  `toolchain go1.27.3` in `go.mod` with `GOTOOLCHAIN=local` in CI so the runner never downloads
  a different toolchain.
- Renovate groups `github.com/tigerbeetle/tigerbeetle-go` and `ghcr.io/tigerbeetle/tigerbeetle`
  into one PR that is never auto-merged. `scripts/check-versions.sh` (CI job `versions`) fails
  when the client version in `go.mod` differs from the server tag in `compose.yaml`. It carries
  one documented exception, `0.17.9=0.17.10`: v0.17.10 is a client-only rebuild that fixes the
  static libraries on macOS 27, speaks the 0.17.9 protocol and was verified against a 0.17.9
  replica. The exception is removed once a 0.17.10 or later server exists.
- pnpm 12.15.0 is declared in the `packageManager` field with its integrity hash. Corepack is
  not required (it is absent from Node 25 and later); `pnpm/action-setup` reads the field in CI
  and developers install pnpm however they like, with `engines` rejecting the wrong major.
- lefthook installs the git hooks from its postinstall script, which `pnpm-workspace.yaml`
  explicitly allows under `allowBuilds` (pnpm 11 and later refuse unapproved install scripts).
  `lefthook.yml` runs Biome on staged web files, `golangci-lint fmt` and `golangci-lint run` on
  staged Go files, Redocly lint on spec changes, and commitlint on the message.
- CI (`.github/workflows/ci.yml`) runs on `ubuntu-24.04`, pinned rather than `ubuntu-latest`
  because the latter is migrating to 26.04 during October 2026. Every action is pinned by commit
  SHA with the tag in a trailing comment; Renovate's `helpers:pinGitHubActionDigests` keeps the
  SHAs current. Jobs: `go`, `go-integration`, `codegen-drift`, `versions`, `openapi`, `web`,
  `docker`. `codegen-drift` runs `pnpm generate` and fails on any uncommitted diff, which is the
  cross-language guarantee that motivated the monorepo.

### Consequences

- Good: one `pnpm generate` regenerates the Go server and the TypeScript types in dependency
  order, and CI proves the committed output is current.
- Good: a contributor runs `pnpm install` and `pnpm tools:install` and has every tool at the
  pinned version, with hooks installed as a side effect.
- Good: Turborepo caches Go builds, lints and unit tests when their declared inputs are
  unchanged, which makes root-level `pnpm build` cheap for web-only changes.
- Good: Renovate owns every bump, including shell-pinned tools and action SHAs, and cannot split
  the TigerBeetle client from the server.
- Bad: `apps/api/package.json` is a file Go developers would not otherwise have, and the Turborepo
  `inputs` lists must be maintained by hand; a forgotten input yields a stale cache hit, so new
  input kinds (for example a new config file read at build time) must be added to `turbo.json`.
- Bad: the `tool` directive pulls oapi-codegen's dependency tree into `go.mod` as indirect
  requirements, which lengthens the file and widens what govulncheck scans.
- Bad: the shim scripts prepend `$(go env GOPATH)/bin` to `PATH`, so a tool installed elsewhere is
  ignored; developers must use `scripts/install-tools.sh` rather than their own installs.
- Bad: sqlc archive checksums are pinned by hand in the install script and must be updated with
  each sqlc bump, which Renovate cannot do on its own.

## More Information

- [ADR-0003](0003-go-api-typescript-react-console.md): the two languages and toolchains this
  layout has to serve.
- [ADR-0008](0008-openapi-first-generated-server-and-client.md): the OpenAPI spec is the
  contract that both generators read.
- Repo paths: `package.json`, `pnpm-workspace.yaml`, `turbo.json`, `apps/api/package.json`,
  `apps/api/go.mod`, `apps/api/oapi-codegen.yaml`, `apps/api/sqlc.yaml`,
  `scripts/install-tools.sh`, `scripts/check-versions.sh`, `renovate.json`, `lefthook.yml`,
  `.github/workflows/ci.yml`.
- TigerBeetle client and server version rule (client must not be newer than the replicas,
  upgrade replicas first): https://docs.tigerbeetle.com/operating/upgrading/
