# Handoff

Updated on 2026-10-25 for the next coding agent. It records what exists, what was verified, what
was decided along the way, and what to do next. Read AGENTS.md first for the conventions; read
this file for the state of the work.

## Where things stand

A migration is under way, planned in `docs/plans/ddd-transports/plan.md` and approved by the
owner on 2026-10-25: bounded-context modules, GraphQL, gRPC, AMQP RPC, NATS RPC and Kafka, the
REST spec and GraphQL schema generated from Go, an orval REST client, the console on GraphQL and a
root justfile, in phases 0 to 7 with a review stop after each.

- Phase 0 (documentation) is done and was approved: ADR-0015 to ADR-0024, the amended status
  lines, and pointers in AGENTS.md and docs/architecture.md.
- Phase 1 (the justfile) is done and waits for the owner's review. The root `justfile` is the
  entry point for every task; the root `package.json` keeps only `prepare`; Turborepo runs the
  TypeScript packages only; the pre-commit hook and every CI job call just recipes; the console
  has a `typecheck` script; no caret range is left. It also restored the oapi-codegen tool's
  dependencies (fact 28) and formatted ten console components that `vp check` reported.
- Phase 2 (the REST spec generated from Go with Huma) comes next. The Go code and the layout are
  still those of the console work.

The API implements the ledger surface (ADR-0004 to ADR-0010, details in ADR-0013) and the console
covers every page of `docs/console.md` (ADR-0014). The work is on the `feat/console` branch, not
merged and not pushed, so CI has never run on GitHub. Everything below was verified on macOS 27
(arm64, OrbStack as the Docker engine).

Working and verified:

- After phase 1: `just lint`, `just typecheck`, `just test`, `just build`, `just vulncheck`,
  `just versions-check` and `just generate-check` (no diff) pass, and the pre-commit hook formats
  and lints a staged Go file and lints a staged spec through just.
- `TESTCONTAINERS_RYUK_DISABLED=true just test-integration` passes. The end-to-end test covers ledgers,
  rules, event types, multi-posting and multi-asset transactions, replays and key reuse,
  declines with per-posting results, reverts, holds (partial capture, void, expiry), aggregates,
  and both crash windows of ADR-0007 settled by the reconciler.
- The console was driven in a real browser against the local API and `scripts/seed-demo.sh`
  data: every page renders in light and dark; New transaction commits, declines (failing
  posting, ways forward, a new key on retry) and shows balances read back; holds capture in
  part and void; the Cmd+K search, the account tree and the request drawer work; the shell
  collapses to a drawer at phone width.
- `just compose-up` builds and starts the whole stack; `/readyz` answers 200 and the console in
  its container serves its pages and proxies `/api` to the API. The image smoke test was not
  rerun.

Not done, by design or by circumstance:

- No GitHub remote, so CI has not run. Push, open a PR, and watch `.github/workflows/ci.yml`.
- Renovate needs the Mend Renovate GitHub App on the repository.
- No authentication, release pipeline or Kubernetes: out of scope (AGENTS.md).
- Not built yet: the transaction list filters on metadata and on "reverted", an export, and the
  posting-rule DSL and live feed of `docs/architecture.md`.

## Pinned versions

| Area                           | Version                                                                                                                                                                                                                  |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Go                             | 1.27.3 (`go 1.27.0`, `toolchain go1.27.3` in go.mod; builder `golang:1.27.3-trixie`)                                                                                                                                     |
| TigerBeetle server             | `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`                                                                                                                                                                                 |
| TigerBeetle Go client          | `github.com/tigerbeetle/tigerbeetle-go` v0.17.10 (see exception below)                                                                                                                                                   |
| Runtime image                  | `gcr.io/distroless/base-debian13:nonroot`                                                                                                                                                                                |
| Postgres                       | `postgres:18.6-trixie`, pgx v5.11.0, sqlc v1.31.1, goose v3.29.0 (library only)                                                                                                                                          |
| oapi-codegen                   | v2.8.1 via the go.mod `tool` directive, runtime v1.8.0                                                                                                                                                                   |
| golangci-lint, govulncheck     | v2.15.0, v1.9.0 (installed by `scripts/install-tools.sh`)                                                                                                                                                                |
| just                           | 1.59.0 (installed by `scripts/install-tools.sh`, sha256 pinned per platform)                                                                                                                                             |
| env, go-cmp, testcontainers-go | v11.4.1, v0.7.0, v0.44.0                                                                                                                                                                                                 |
| Node, pnpm, Turborepo          | 24.22.1, 12.15.0, 2.11.10                                                                                                                                                                                                |
| Console                        | React 19.3.1, Vite+ 1.4.0 (Vite 8.3.5), TypeScript 7.0.2, ky 2.2.0, TanStack Router 1.170.46, Query 5.104.5, Form 1.33.5, Table 9.3.1, Pacer 0.24.1, Base UI 1.9.0, shadcn 4.22.1 (base-nova), Tailwind 4.3.3, zod 4.6.7 |
| Generators and lint            | openapi-typescript 7.13.0 with TypeScript 5.9.3 (packages/openapi only), Redocly CLI 2.71.0, Vite+ 1.4.0 (Oxfmt 0.74.0, Oxlint 1.89.0, git hooks), commitlint 21.2.4                                                     |
| GitHub Actions                 | checkout v7.0.1, setup-go v7.0.0, setup-node v7.1.0, pnpm/action-setup v6.1.0, golangci-lint-action v9.3.0, setup-buildx v4.4.1, build-push v7.4.0, all pinned by SHA, runners `ubuntu-24.04`                            |

## Facts learned the hard way

Each of these cost time. They are documented in code comments or ADRs, but here they are in one
place.

1. **TigerBeetle client 0.17.9 does not link on macOS 27.** The error is "64-bit mach-o not
   8-byte aligned" from the bundled static library. Tag v0.17.10 is a client-only rebuild with
   the fix (no server release, no image, empty changelog). It was verified to work against a
   0.17.9 server: lookup and create_accounts succeed. `scripts/check-versions.sh` carries this
   pair as an explicit exception. Remove the exception when a server release at or above
   0.17.10 ships, and move both together from then on.
2. **The Go client needs io_uring, not just the server.** Under Docker 25+ the default seccomp
   profile blocks it and the API dies at startup with "io_uring is not available". Both the
   `tigerbeetle` and the `api` services in `deploy/compose/compose.yaml` set
   `security_opt: seccomp=unconfined`; the smoke test passes the same flag. A custom seccomp
   profile that allows only `io_uring_setup`, `io_uring_enter` and `io_uring_register` is the
   production follow-up.
3. **The client accepts IP addresses only.** `tigerbeetle:3000` from compose is rejected with
   "invalid client cluster address". `apps/api/internal/adapters/tigerbeetle/addresses.go`
   resolves host names once at startup, preferring IPv4. The integration test does the same for
   the testcontainers host.
4. **The TigerBeetle image has no `/data` directory.** Compose creates it through the volume;
   the integration test runs `mkdir -p /data` before `format`. `format` exits 1 when the file
   exists, hence the `test -f || format` init service in compose.
5. **Postgres 18 images moved the volume** from `/var/lib/postgresql/data` to
   `/var/lib/postgresql`. Mounting the old path silently loses the cluster on recreate.
6. **Host ports 5432 and 3000 were taken on the owner's machine.** The stack publishes Postgres
   on 15432 and TigerBeetle on 13000 by default; `.env.example` documents the overrides.
7. **ky 2 reads the body before `beforeError` runs**, so decoding problem+json or the 503
   readiness body in a hook fails with "Body has already been read". The client uses
   `throwHttpErrors: false` and inspects the status itself. There is an empirical test for this
   pattern worth keeping in mind when extending the client.
8. **TypeScript 7.0.2 has no JS compiler API**, which openapi-typescript 7.13.0 needs. The
   generator runs in `packages/openapi` with TypeScript 5.9.3; everything else uses 7.0.2.
   Renovate is told to keep that package below 6.
9. **Corepack is not bundled with Node 25+** and was absent on the owner's machine. The
   `packageManager` field pins pnpm; pnpm switches itself. Do not reference `corepack` in docs.
10. **goose's CLI drags ClickHouse, MySQL, MSSQL and more into go.mod** through the `tool`
    directive. goose is a library only here (`internal/adapters/postgres/migrate.go`, with a
    Postgres session lock). Only oapi-codegen is a `go tool`.
11. **The distroless Debian 12 images were deprecated on 2026-09-01** and have glibc 2.36; a
    binary built on trixie (glibc 2.41) does not start on them. Builder and runtime are both
    Debian 13.
12. **`ubuntu-latest` is migrating to 26.04 during October 2026.** Workflows pin
    `ubuntu-24.04`.
13. **sqlc publishes no checksum file.** `scripts/install-tools.sh` pins the sha256 of each
    platform archive; update them with the version.
14. **Go tools live in `$(go env GOPATH)/bin`.** `scripts/install-tools.sh` installs there and,
    in GitHub Actions, adds the directory to `GITHUB_PATH`; locally it must be on PATH.
    `.githooks/pre-commit` and the justfile prefix PATH themselves.
15. **Turborepo strict env mode strips variables.** Since phase 1 Turborepo runs only the
    TypeScript packages, so the `dev` task declares only the console variables (`API_PROXY_TARGET`,
    `VITE_*`). The Go recipes run outside Turborepo and see the whole environment.
16. **Inline `type` specifiers keep the import at runtime.** With `verbatimModuleSyntax`,
    `import { type A } from './schema'` leaves `import './schema'` in the bundle, and
    `schema.d.ts` has no runtime module, so the build fails. The lint autofix
    (`import/consistent-type-specifier-style: prefer-inline`) produces that form, so
    `packages/api-client/src/index.ts` keeps one top-level `import type` with a disable comment.
17. **The lint autofix can delete type assertions that matter.** `[] as MetadataRow[]` in a
    default value became `[]`, typed `never[]`. Type default values with a named constant
    (`const defaultValues: TransactionDraft = ...`) instead of `as`.
18. **Base UI buttons check what they render.** A `Button` rendered as a router `Link` needs
    `nativeButton={false}`, and one rendered as a Base UI trigger needs `nativeButton`; the
    console's `Button` sets the first by default and accepts the second explicitly.
19. **Stored write answers are domain problems, not HTTP documents.** A declined record keeps
    `reason`, `detail`, `posting` and `results`; `features/requests/stored-problem.ts` rebuilds
    the problem type and title the way the HTTP adapter does.
20. **The Pencil app's integrated browser hung on screenshots** of the dev server. Browser checks
    used `npx agent-browser@0.39.2` with its own Chrome (`agent-browser install`) instead.
21. **cmdk keeps its highlight when items change.** With server-side search the palette controls
    the selected value and resets it to the first match when an answer arrives.
22. **The console container needs `--ignore-scripts` and `--host`.** The root `prepare` script
    installs the Vite+ git hooks and fails without git, and `vp dev` listens on the container's
    localhost unless told otherwise. `deploy/compose/console.Dockerfile` handles both.
23. **A partly staged file breaks the pre-commit hook.** When a file is staged and also has
    unstaged changes, and `vp check --fix` reformats the staged version, lint-staged cannot put the
    unstaged changes back: it prints "Failed to restore unstaged changes", reverts and aborts the
    commit. Stage whole files.
24. **Staged Go files failed the hook between 67af5db and phase 1 commit 4.** The `staged` block
    still called the deleted shim (`pnpm --filter api ...`); it now calls `just fmt-go-files` and
    `just lint-go`.
25. **TigerBeetle CDC as seen from the 0.17.9 image** (spike S3, ADR-0022). u128 and u64 values
    are JSON numbers up to 2^53 - 1 and JSON strings from 2^53 on; nanosecond timestamps are
    always strings, so decode both forms without a float. `--host` takes an IP address only. The
    job keeps its progress as headers on `tigerbeetle.internal.progress.<cluster>` and holds the
    exclusive queue `tigerbeetle.internal.locker.<cluster>`, so a second job fails with
    `RESOURCE_LOCKED`. Expirations arrive as `two_phase_expired` with no client involved.
26. **RabbitMQ's `guest` user only connects from localhost.** A container needs a user created
    with `RABBITMQ_DEFAULT_USER` and `RABBITMQ_DEFAULT_PASS`.
27. **`tigerbeetle repl` wants a terminal**, and piped input prints "ANSI escape sequences not
    supported". `repl --command="create_accounts ...;"`, absent from `--help`, runs one statement
    and prints JSON, which suits scripts.
28. **Upgrading every Go dependency broke the oapi-codegen tool.** `go get -u` in b89e9f7 moved
    `speakeasy-api/openapi`, `speakeasy-api/jsonpath` and `dprotaso/go-yit` past the versions
    oapi-codegen v2.8.1 compiles with (they switched YAML libraries), and nothing noticed because
    only `go tool oapi-codegen` uses them. 87eba56 sets them back to what the tool requires. Leave
    them there until phase 2 removes oapi-codegen; `just generate` now runs the tool, so a
    regression fails the drift check.
29. **Only the comment line right above a recipe shows in `just --list`.** Recipes with a longer
    comment use `[doc('...')]` for the listed text.
30. **`vp check` at the root checks formatting; the console's `lint` task does not.** Ten console
    components with stray blank lines passed `pnpm lint` but failed `vp check`, which CI's web job
    runs. `just lint` runs both.

## Decisions amended along the way

On 2026-10-25 the owner approved the migration plan and answered its questions (plan section 23).
ADR-0015 adds the status `Accepted, amended by ADR-NNNN`; ADR-0001, 0002, 0003, 0004, 0007, 0009,
0012, 0013 and 0014 carry it, and ADR-0008 is superseded by ADR-0018 and ADR-0019. Read the
amending ADRs before acting on an amended one.

Earlier, during the console work, the owner approved these on 2026-10-24; the ADRs reflect them.

- **Transfer IDs are minted, not hashed** (ADR-0007). The pending idempotency record stores the
  transaction ID and one `ID()` per leg; retries and the reconciler reuse them. Hashing the
  Idempotency-Key was dropped because random IDs hurt TigerBeetle throughput and a burned ID
  (`id_already_failed` after a transient failure) could never be recovered.
- **Reconciler semantics** (ADR-0007): look up stored IDs first; resubmit only when none exist;
  the pattern `[exists, linked_event_failed, ...]` means already committed.
- **Client and server versions may differ by the documented exception only** (ADR-0009).
- **Decision 11 (string amounts, uuid columns) is folded into ADR-0002 and ADR-0004**. ADR-0011
  is unrelated: it replaced Biome with Vite+ (`vp check`) for TypeScript.
- **ADR-0007 caps postings per transaction at 64.** The number was chosen by the ADR author, not
  the owner; confirm it before implementing transactions.

- **Idempotency-Key only on money writes** (ADR-0013): transactions, reverts, holds, captures and
  voids. Ledger creation, metadata, rules and event types are idempotent by their nature.
- **Event types are per ledger** (ADR-0013); codes start at 2, `default` is code 1.
- **The console is a TanStack Router single-page app, not TanStack Start** (ADR-0014): it has no
  data of its own and must stay deployable as static files behind any proxy.

## Code map

API (`apps/api`):

- Domain `internal/domain/ledger`: amounts (u128), addresses, assets, rules and their matching,
  postings, metadata, event types, write operations and statuses, problems with stable reasons.
- Use cases `internal/app`: `writes.go` is the one write protocol of ADR-0007 for transactions,
  reverts, holds, captures and voids; `reconciler.go`; `accounts.go`, `transactions.go`,
  `holds.go`, `ledgers.go`, `settings.go`, `overview.go` (stats, requests, search).
- Adapters: `tigerbeetle/ledger.go` (bounded calls, result names), `postgres` (sqlc queries in
  `apps/api/queries`, migrations 00001 to 00008), `httpapi` (strict handlers, problem mapping).
- Wiring `cmd/api/wire.go`; the end-to-end test is `cmd/api/api_integration_test.go`.

Console (`apps/console/src`):

- `routes/`: one file per URL under `_console/`; `__root.tsx` provides Query and tooltips.
- `features/`: `ledgers`, `overview`, `transactions` (with `new-transaction/`), `accounts`,
  `balances`, `assets`, `holds`, `requests`, `settings`, `system`, `preferences`, `search`.
- `components/ui` (shadcn, restyled), `components/ledger` (domain display), `components/form`
  (TanStack Form hook), `components/data-table`, `components/layout` (shell, page, breadcrumbs).
- `lib/`: `money.ts`, `preflight.ts`, `net-effect.ts`, `validation.ts`, `idempotency.ts`,
  `time.ts`, `preferences.ts`, `curl.ts`, `api.ts`, `query-client.ts`.
- `styles/globals.css`: the Ledger Instrument tokens and their mapping to shadcn variables.

## Suggested next steps, in order

1. The owner reviews phase 1: `just --list`, the hook, the CI workflow (plan section 19).
2. Phase 2, the REST spec generated from Go with Huma, starting with spikes S1 (Huma) and S2
   (orval); then phases 3 to 7, each ending with a review stop, an update of this file and a
   summary of versions and deviations.
3. Push to GitHub, open the pull request for `feat/console`, make CI green, install Renovate.
4. Review the console against `.design/home.pen` with the owner; the screens were followed
   closely but not pixel by pixel (the account tree and the pending result view simplify the
   design's activity log and cancel button, which the API does not support).
5. Transaction list filters on metadata and on "reverted" need API support first.
6. Operational follow-ups: a custom seccomp profile instead of `unconfined`, image digests pinned
   by Renovate, a multi-arch image and a static console image in a release pipeline.
7. The posting-rule DSL and the console's live feed, future work in `docs/architecture.md`; the
   Kafka relay of phase 7 is the first CDC consumer.

## Loose ends to be aware of

- `.design/` holds the Pencil files of the design system and the screens. Open them with Pencil
  (pen.dev); they are encrypted, so do not read them as text.
- `tigerbeetle` and `tigerbeetle.zip` at the repository root are a local TigerBeetle binary and
  its archive, untracked. Do not commit them.
- The readiness latency is rounded to whole milliseconds and shows 0 ms locally, so the System
  page's latency chart is flat on a laptop.
- The hook output (lint-staged, Redocly) prints emojis and symbols in the terminal. The rule
  "no emojis" applies to repository content, which is checked with
  `grep -rnP '[^\x00-\x7F]'` over tracked files.
