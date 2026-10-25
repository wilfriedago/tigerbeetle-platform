# 12. Vite+ git hooks, replacing lefthook

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

ADR-0009 chose lefthook 2.2.2 to run the git hooks. Its `postinstall` script installed the hooks
during `pnpm install`, which required an `allowBuilds` entry in `pnpm-workspace.yaml`, and
`lefthook.yml` described four `pre-commit` jobs (format and lint for web files, `golangci-lint
fmt` and `golangci-lint run` for Go, Redocly for the spec) and one `commit-msg` job
(commitlint).

ADR-0011 made Vite+ the TypeScript toolchain. Vite+ also ships a hook dispatcher (`vp hooks`,
`vp config`) and a staged-file runner (`vp staged`, which bundles lint-staged 17) configured
from the same root `vite.config.ts`. Keeping lefthook means two tools that both know how to
install hooks and select staged files. This ADR decides which one runs the hooks. It replaces
only the lefthook part of ADR-0009; Turborepo, pnpm and the pinning policy stand.

## Decision Drivers

- One place for workspace tooling configuration, as ADR-0011 started with `vite.config.ts`.
- Every check lefthook ran must keep running on the same files, including the Go jobs, so no
  coverage is lost for Go contributors.
- Hooks install themselves on `pnpm install`, and an install without a `.git` directory (the
  console Docker image) must not fail.
- No dependency install scripts that need approval in `allowBuilds`.
- Fewer pinned packages: the hook runner should move with the Vite+ version.

## Considered Options

1. Vite+ hooks: `.githooks/` scripts, the `staged` block and `vp config` in `prepare`
   (chosen).
2. Keep lefthook, with its first job calling `vp check --fix` (the state after ADR-0011).
3. Husky plus lint-staged as separate dependencies.

Option 2 works but keeps a second hook tool and its own YAML next to `vite.config.ts`. Option 3
is what Vite+ already bundles, with two more dependencies to pin.

## Decision Outcome

Chosen option: 1, Vite+ 1.4.0 runs the git hooks.

- The root `package.json` has `"prepare": "vp config --hooks-dir .githooks"`. `vp config` installs the
  dispatcher in `.githooks/_` (ignored by its own `.gitignore`) and sets `core.hooksPath`.
  Without a `.git` directory it prints a
  notice and exits 0. `lefthook` and its `allowBuilds` entry are removed.
- `.githooks/pre-commit` prepends `$(go env GOPATH)/bin` to `PATH` when Go is installed, as
  the lefthook Go jobs did, then runs `vp staged`. `.githooks/commit-msg` runs
  `pnpm exec commitlint --edit "$1"`. Both are committed; the dispatcher adds
  `node_modules/.bin` to `PATH` and passes the git arguments through.
- The `staged` block in the root `vite.config.ts` maps each former lefthook job:
  - web files (`js`, `ts`, `tsx`, `json`, `css`, `html`, `md`, `yaml` and friends):
    `vp check --fix`, the same types CI checks;
  - `apps/api/**/*.go`: `golangci-lint fmt` on the staged files, then `pnpm --filter api lint`
    once for the module, in that order, both from `apps/api` so `.golangci.yml` applies;
  - `packages/openapi/**/*.yaml`: `pnpm --filter @tigerbeetle-platform/openapi lint`.
- Contributors can skip hooks for one command with `VP_GIT_HOOKS=0 git commit ...`, or turn
  them off in a clone with `pnpm exec vp hooks disable`.

### Consequences

- Good: hook policy, staged-file rules, format and lint settings live in one file, and the
  hook runner moves with the `vite-plus` catalog entry.
- Good: lint-staged hides unstaged changes while the tasks run and restores them afterwards,
  so a partially staged file is checked as it will be committed, and it restores the original
  files when a task fails.
- Good: no install script needs approval in `pnpm-workspace.yaml`.
- Bad: `vp config` and `vp hooks enable` never overwrite an existing `core.hooksPath`, even one
  Vite+ set itself. After changing the hooks directory, an existing clone must run
  `pnpm exec vp hooks disable` and then `pnpm exec vp hooks enable --hooks-dir .githooks`;
  `vp hooks status` shows which directory is active.
- Bad: globs run in parallel. A spec change runs `vp check --fix` and Redocly at the same time
  on `openapi.yaml`; harmless today because Redocly only reads.
- Bad: when a fix leaves nothing to commit (for example `golangci-lint fmt` reverting a
  whitespace-only change), lint-staged stops with "prevented an empty git commit" instead of
  committing nothing.
- Bad: the `staged` block uses functions for the Go and spec tasks, so it is TypeScript rather
  than declarative YAML; the comments in `vite.config.ts` explain each entry.

## More Information

- [ADR-0009](0009-pnpm-turborepo-monorepo-with-go-module.md): the monorepo and the lefthook
  choice this record replaces.
- [ADR-0011](0011-vite-plus-toolchain-for-typescript.md): Vite+ as the TypeScript toolchain.
- Repo paths: `vite.config.ts` (`staged` block), `.githooks/pre-commit`,
  `.githooks/commit-msg`, `package.json` (`prepare`), `pnpm-workspace.yaml`.
- Vite+ commit hooks and staged config: https://viteplus.dev/guide/commit-hooks,
  https://viteplus.dev/config/staged
- lint-staged 17 release notes: https://github.com/lint-staged/lint-staged/releases/tag/v17.0.0
