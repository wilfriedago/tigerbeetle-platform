# 16. Justfile as the single entry point; Turborepo for TypeScript only

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

ADR-0009 made pnpm and Turborepo the task runner for the whole repository. The root
`package.json` scripts were the verbs (`pnpm generate`, `pnpm build`, `pnpm test`), and a shim,
`apps/api/package.json`, let Turborepo run and cache the Go tasks through hand-written `inputs`
lists in `turbo.json`. ADR-0009 listed the shim and those lists as costs.

The shim is gone (commit 67af5db), but `turbo.json` still declares `api#*` tasks, the `staged`
block of `vite.config.ts` still calls `pnpm --filter api`, `deploy/compose/console.Dockerfile`
still copies the file and `CONTRIBUTING.md` still describes it. Go tasks currently have no root
command at all.

The migration in `docs/plans/ddd-transports/plan.md` adds generators on both sides of the
language line, and their order matters: sqlc, then the Go commands that write the REST spec and
the GraphQL schema, then gqlgen and buf, which read those files, then the TypeScript generators
(the REST client, the console's GraphQL documents, the route tree). With Turborepo alone, the Go
steps would need the shim back and a longer hand-kept `inputs` list.

ADR-0009 rejected "a Makefile at the root calling `go` and `pnpm` by hand" because it
"recomputes everything on every run and encodes the dependency graph in shell". This ADR replaces
only the task-runner part of ADR-0009 (the root scripts, the shim, the Go tasks in `turbo.json`)
and the commands in ADR-0012's `staged` block. The pnpm workspace, Turborepo for the TypeScript
packages, the pinning policy and the coupled TigerBeetle versions stand.

## Decision Drivers

- One entry point for every task in the repository, with a listing that says what each does.
- The cross-language generate order written down where it runs, readable top to bottom.
- Turborepo keeps caching the TypeScript packages (ADR-0011).
- Go keeps its own caches: `go build` and `go test` cache by content, golangci-lint caches its
  analysis; no hand-kept list of Go inputs.
- Developers, git hooks and CI run the same commands.
- The runner is one pinned binary with a checksum, like the other tools in
  `scripts/install-tools.sh`.

## Considered Options

1. just at the root: Go recipes call the Go toolchain directly, TypeScript recipes call pnpm and
   Turborepo (chosen).
2. Restore the shim and keep Turborepo as the only runner, as ADR-0009 wrote it.
3. A root Makefile.
4. Task (`Taskfile.yml`) or mise tasks.

Option 2 brings back a package that Go developers do not otherwise need, and every new Go
generator adds inputs that someone must remember to list; a forgotten one gives a stale cache
hit. Option 3 is what ADR-0009 rejected: make thinks in file targets, so every task needs
`.PHONY`, arguments are awkward, and macOS ships an old GNU make 3.81. Option 4 adds its own task
caching (Task) or its own tool installer (mise), which overlap with Turborepo and
`scripts/install-tools.sh`.

ADR-0009's objection does not apply to option 1. just has no cache of its own and does not need
one: it calls `go`, which caches Go work exactly, and `pnpm turbo run`, which caches TypeScript
work. Nothing is recomputed that was not recomputed before. The dependency graph that remains in
the justfile is the four-step generate order, which no single tool owns.

## Decision Outcome

Chosen option: 1, just 1.59.0.

- `scripts/install-tools.sh` pins just 1.59.0 with the sha256 of each release archive for the
  four platforms it supports, and a `# renovate:` comment, as for sqlc. The first install on a
  machine runs the script directly; afterwards `just tools-install` does.
- The root `justfile` holds every verb, grouped with `[group(...)]` attributes so `just --list`
  prints them with their comments: `generate` (and one recipe per step), `generate-check`,
  `lint` (`lint-go`, `lint-ts`, `lint-spec`), `format`, `typecheck`, `test` (`test-go`,
  `test-ts`), `test-integration`, `build`, `dev`, `vulncheck`, `compose-up`, `compose-down`,
  `compose-logs`, `compose-reset`, `tools-install` and `versions-check`. Later phases add
  `lint-proto`, `lint-depguard-probe`, `spec-diff`, `test-e2e` and `compose-up-brokers`.
- Go recipes run in `apps/api` and call `go`, `go tool`, `sqlc`, `golangci-lint` and
  `govulncheck` directly. The justfile puts `$(go env GOPATH)/bin` first on `PATH`, the
  convention the shim had (ADR-0009).
- TypeScript recipes call `pnpm turbo run <task>` or `pnpm exec vp check`. Turborepo still orders
  and caches the TypeScript packages; `turbo.json` loses its `api#*` tasks, and the API
  variables leave the `dev` task's `env` list.
- `just generate` runs, in order: 1. `sqlc generate`; 2. the Go commands that write
  `packages/openapi/openapi.yaml` and `packages/graphql/schema.graphql`; 3. gqlgen and
  `buf generate`; 4. `pnpm turbo run generate`. Steps 2 and 3 appear with the phases that add
  those commands. `just generate-check` runs it and fails on any change to tracked files; CI's
  drift job uses it.
- The root `package.json` keeps one script, `prepare`, which installs the git hooks (ADR-0012).
  Every other script goes.
- Git hooks (ADR-0012): the mechanism is unchanged; only the commands in the `staged` block
  change. Staged Go files run `just fmt-go-files <files>` then `just lint-go`; the spec runs
  `just lint-spec`.
- CI: every job installs the tools with `scripts/install-tools.sh`, which installs just, then
  calls recipes. The console unit tests run in CI through `just test-ts`, which they did not
  before.
- `.editorconfig` gets a `[justfile]` section with a four-space indent, the indent
  `just --fmt` writes.

### Consequences

- Good: `just --list` is the one list of verbs for Go, TypeScript, the compose stack and the
  tools; contributors, hooks and CI share it.
- Good: the generate order is four lines in one file. A new generator takes one line in the
  right step.
- Good: no shim and no hand-kept Go `inputs`; Go's own caches decide what to rebuild.
- Good: the Go recipes need only Go and just; pnpm and Node are needed for the TypeScript
  recipes and the git hooks.
- Bad: one more tool to install, and the first install must call the script directly.
- Bad: two runners to understand, just on top and Turborepo under the TypeScript recipes.
- Bad: recipes are shell. Logic beyond a few lines belongs in `scripts/`, not in the justfile.
- Bad: the just checksums are updated by hand with each bump, as for sqlc; Renovate cannot do it.

## More Information

- [ADR-0009](0009-pnpm-turborepo-monorepo-with-go-module.md): the monorepo; this record replaces
  its task-runner part.
- [ADR-0011](0011-vite-plus-toolchain-for-typescript.md): Vite+ and Turborepo for TypeScript.
- [ADR-0012](0012-vite-plus-git-hooks.md): the git hooks; this record changes the commands in
  its `staged` block.
- [ADR-0015](0015-amended-status-for-accepted-adrs.md): the amended status set on ADR-0009 and
  ADR-0012.
- `docs/plans/ddd-transports/plan.md`, section 16: recipe list, sketch and generate order.
- Repo paths: `justfile`, `package.json`, `turbo.json`, `vite.config.ts`,
  `scripts/install-tools.sh`, `.github/workflows/ci.yml`, `.editorconfig`.
- just manual: https://just.systems/man/en/
- just 1.59.0 release and checksums: https://github.com/casey/just/releases/tag/1.59.0
