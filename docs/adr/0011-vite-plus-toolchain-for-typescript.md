# 11. Vite+ as the TypeScript toolchain, replacing Biome

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

ADR-0003 chose Biome 2.6.2 as the single formatter and linter for TypeScript, and ADR-0009 lists
it among the root tooling and runs it from the lefthook `pre-commit` hook. Vite itself was a
plain dependency of `apps/console`, invoked as `vite`, `vite build` and `vite preview`.

Vite+ (the `vite-plus` package, CLI `vp`) bundles Vite, Rolldown, Vitest, tsdown, Oxlint and
Oxfmt behind one pinned version. Its `vp check` command formats, lints and type-checks in one
pass, with type-aware lint rules powered by tsgolint on the TypeScript 7 toolchain that the
console already uses. Adopting it for the console build while keeping Biome would leave two
formatters and two linters with overlapping rules. This ADR decides which one owns TypeScript
formatting and linting. It replaces only the Biome parts of ADR-0003 and ADR-0009; everything
else in those records stands.

## Decision Drivers

- One tool and one config for TypeScript formatting and linting, as ADR-0003 already required.
- Type errors and lint findings reported by the same command, locally, in the hook and in CI.
- Exact version pins (ADR-0009): one version number moves Vite, Vitest, Oxlint and Oxfmt.
- No change to the Go side or to Turborepo as the task runner. The hook runner is a separate
  decision, recorded in ADR-0012.
- Minimal churn in existing source files.

## Considered Options

1. Replace Biome with Vite+ (`vp check`: Oxfmt, Oxlint, type-aware checks) (chosen).
2. Keep Biome for formatting and linting, use Vite+ only for dev, build and an extra lint pass.
3. Use Vite+ only as the Vite replacement for dev and build, without its check command.

Option 2 runs two linters with different rule sets and two formatters that can disagree, which
is what ADR-0003 set out to avoid. Option 3 keeps one tool per job but pays for Vite+ without
using its main benefit.

## Decision Outcome

Chosen option: 1, Vite+ 1.4.0 replaces Biome.

- `vp migrate` (1.4.0, `--no-interactive`) produced the dependency layout: `vite-plus` 1.4.0 and
  `vite` aliased to `@voidzero-dev/vite-plus-core` 1.4.0 (Vite 8.3.5) in the default catalog of
  `pnpm-workspace.yaml`, a `vite@*` override and `peerDependencyRules` so that every `vite` peer
  resolves to the core. Manifests reference them with `catalog:`; the catalog holds exact
  versions, so ADR-0009's pinning rule holds.
- Format and lint settings live only in the root `vite.config.ts`; package configs cannot
  override them. The style differs from the former Biome one: single quotes, no semicolons, no
  trailing commas, width 150, sorted imports in named groups (React, TanStack, external,
  internal, relative) and sorted `package.json` keys. The generated
  `packages/api-client/src/schema.d.ts` is ignored, as it was in `biome.json`.
- Oxlint runs the `correctness` category as errors and `suspicious`, `perf` and `style` as
  warnings, with the `eslint`, `typescript`, `unicorn`, `oxc`, `react`, `react-perf`,
  `jsx-a11y`, `import`, `promise` and `vitest` plugins, plus `typeAware`, `typeCheck` and
  `reportUnusedDisableDirectives`. Warnings do not fail `vp check`; only errors do.
- The console scripts are `vp dev --host`, `vp build` and `vp preview`. The `lint` and `format`
  scripts of `apps/console` and `packages/api-client` are `vp check` and `vp check --fix`, so
  `pnpm lint` and `pnpm format` keep their meaning through Turborepo. The Turborepo `lint` task
  lists `$TURBO_ROOT$/vite.config.ts` as an input so a config change invalidates the cache.
- The pre-commit hook runs `vp check --fix` on staged files of the types Oxfmt formats (ADR-0012
  covers how hooks are installed and run). The CI `web` job runs `pnpm exec vp check` at the root, which also covers Markdown and
  YAML outside the packages.
- `tsc --noEmit` stays as the `typecheck` task; `vp check` type-checks too, but the separate task
  keeps `pnpm typecheck` independent of lint configuration.
- Turborepo stays the task runner (ADR-0009); Vite Task (`vp run`) is not adopted.

### Consequences

- Good: one command checks formatting, lint rules and types for TypeScript, and the hook, CI
  and `pnpm lint` all call it.
- Good: Vite, Vitest, Oxlint and Oxfmt move together with one catalog entry, which Renovate
  bumps as a single dependency.
- Good: the first `vp test` run needs no extra install once TypeScript tests exist.
- Bad: the warning categories report many findings on the current code (for example
  `react-in-jsx-scope`, which does not apply to the automatic JSX runtime); they need triage
  before any of them can become errors.
- Bad: Oxfmt formats Markdown, which Biome did not; the first run added blank lines before lists
  in the docs. Prose wrapping is preserved.
- Bad: `lazyPlugins()` in `apps/console/vite.config.ts` is typed as possibly undefined, which
  `exactOptionalPropertyTypes` rejects; the config adds `?? []`.
- Bad: `vp test` exits 1 until the first `*.test.ts` file exists, so it is not wired into
  `pnpm test` yet.

## More Information

- [ADR-0003](0003-go-api-typescript-react-console.md): the console stack, whose Biome choice
  this record replaces.
- [ADR-0009](0009-pnpm-turborepo-monorepo-with-go-module.md): the monorepo, Turborepo and the
  pinning policy this record keeps.
- [ADR-0012](0012-vite-plus-git-hooks.md): the git hooks that run `vp check` before a commit.
- Repo paths: `vite.config.ts`, `apps/console/vite.config.ts`, `pnpm-workspace.yaml`,
  `turbo.json`, `.github/workflows/ci.yml`.
- Vite+ migration guide and rules: https://viteplus.dev/guide/migrate,
  https://viteplus.dev/guide/migrate-rules
- `vp check`, lint and format: https://viteplus.dev/guide/check,
  https://viteplus.dev/guide/lint, https://viteplus.dev/guide/fmt
