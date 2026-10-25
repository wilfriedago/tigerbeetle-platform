# Handoff

Written on 2026-10-24 for the next coding agent. It records what exists, what was verified, what
was decided along the way, and what to do next. Read AGENTS.md first for the conventions; read
this file for the state of the work.

## Where things stand

The repository holds the project setup plus a thin vertical slice. Thirteen commits on `main`,
no remote yet, nothing pushed, so CI has never run on GitHub. Everything below was verified on
macOS 27 (arm64, OrbStack as the Docker engine) from a fresh clone of the repository.

Working and verified:

- `pnpm install && pnpm stack:up` then `curl localhost:8080/readyz` answers 200 with Postgres
  and TigerBeetle reported `up`. A second `pnpm stack:up` skips the TigerBeetle format step.
- The console at http://localhost:5173 renders the status of both dependencies from
  `GET /v1/_info` through the Vite proxy (`/api` is proxied to the API container).
- `pnpm lint`, `pnpm test`, `pnpm test:integration`, `pnpm build` and `pnpm typecheck` pass.
- `pnpm generate` produces no diff on a clean tree.
- `scripts/check-versions.sh` passes with the one documented exception (see below).
- `scripts/smoke-test.sh` passes against the image built from `apps/api/Dockerfile`: it runs as
  `nonroot` on `gcr.io/distroless/base-debian13`, prints its version, starts without its
  dependencies, answers `/healthz` with 200 and `/readyz` with 503.
- `govulncheck ./...` reports no vulnerabilities.
- Unknown routes and wrong methods answer `application/problem+json` with the request ID.

Not done, by design or by circumstance:

- No GitHub remote, so the acceptance criterion "CI is green on the first pull request" is
  unverified. Push, open a PR, and watch the seven jobs in `.github/workflows/ci.yml`.
- Renovate is configured (`renovate.json`) but the Mend Renovate GitHub App must be installed on
  the repository for it to run.
- No ledger business logic, no account or transaction endpoints, no authentication, no release
  pipeline, no Kubernetes. All out of scope for the setup task.

## Pinned versions

| Area                           | Version                                                                                                                                                                                       |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go                             | 1.27.3 (`go 1.27.0`, `toolchain go1.27.3` in go.mod; builder `golang:1.27.3-trixie`)                                                                                                          |
| TigerBeetle server             | `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`                                                                                                                                                      |
| TigerBeetle Go client          | `github.com/tigerbeetle/tigerbeetle-go` v0.17.10 (see exception below)                                                                                                                        |
| Runtime image                  | `gcr.io/distroless/base-debian13:nonroot`                                                                                                                                                     |
| Postgres                       | `postgres:18.6-trixie`, pgx v5.11.0, sqlc v1.31.1, goose v3.29.0 (library only)                                                                                                               |
| oapi-codegen                   | v2.8.1 via the go.mod `tool` directive, runtime v1.8.0                                                                                                                                        |
| golangci-lint, govulncheck     | v2.15.0, v1.9.0 (installed by `scripts/install-tools.sh`)                                                                                                                                     |
| env, go-cmp, testcontainers-go | v11.4.1, v0.7.0, v0.44.0                                                                                                                                                                      |
| Node, pnpm, Turborepo          | 24.22.1, 12.15.0, 2.11.10                                                                                                                                                                     |
| Console                        | React 19.3.1, Vite+ 1.4.0 (Vite 8.3.5), TypeScript 7.0.2, ky 2.2.0                                                                                                                            |
| Generators and lint            | openapi-typescript 7.13.0 with TypeScript 5.9.3 (packages/openapi only), Redocly CLI 2.71.0, Vite+ 1.4.0 (Oxfmt 0.74.0, Oxlint 1.89.0, git hooks), commitlint 21.2.4                          |
| GitHub Actions                 | checkout v7.0.1, setup-go v7.0.0, setup-node v7.1.0, pnpm/action-setup v6.1.0, golangci-lint-action v9.3.0, setup-buildx v4.4.1, build-push v7.4.0, all pinned by SHA, runners `ubuntu-24.04` |

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
14. **Go tools live in `$(go env GOPATH)/bin`.** `scripts/install-tools.sh` installs there;
    actions/setup-go puts it on PATH in CI; locally it must be on PATH. `.githooks/pre-commit`
    prefixes PATH itself.
15. **Turborepo strict env mode strips variables.** The `dev` task declares the API and console
    variables, and `test:integration` passes `DOCKER_HOST` and `TESTCONTAINERS_*` through.

## Decisions amended during setup

The owner approved these on 2026-10-24; the ADRs reflect them.

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

## Code map for the next features

- Ports: `apps/api/internal/app/ports.go`. `LedgerStore` (TigerBeetle) only has `Ready` today;
  `MetadataStore` has `Ready`, `CreateLedger`, `GetLedger`; `Clock` and `IDGenerator` exist and
  `IDGenerator` is implemented by the TigerBeetle adapter (`NewID`).
- Use cases: `internal/app/status.go` (readiness and info) and `internal/app/ledgers.go`
  (create and get, no endpoint yet, tested with fakes).
- Domain: `internal/domain/ledger` has `Ledger`, `Asset` (`XOF/0`), `Address`
  (`users:1234:wallet`), `ID` (big-endian 128-bit) and the four sentinel errors.
- HTTP: `internal/adapters/httpapi/handlers.go` implements `gen.StrictServerInterface`;
  `problem.go` maps errors to RFC 9457; `router.go` wires middleware and converts the mux's
  404 and 405 to problems; `middleware.go` has request IDs, access log and panic recovery.
- TigerBeetle: `internal/adapters/tigerbeetle/client.go` (shared readiness probe, `Nop` round
  trip), `id.go` (byte-order conversion), `addresses.go`. Result-code mapping for accounts and
  transfers is still to be written; `mapClientError` covers client-level errors only.
- Postgres: `internal/adapters/postgres/store.go` (pool, ledgers, `mapError` with SQLSTATE
  classes), `migrate.go`, `gen/` from sqlc. Migrations in `apps/api/migrations`, queries in
  `apps/api/queries`.
- Test infrastructure: `internal/devstack` reads the image tags from the compose file so the
  integration tests and the stack never drift.

## Suggested next steps, in order

1. Push to GitHub, open the first pull request, make CI green, install the Renovate app.
2. Ledgers endpoints (`POST /v1/ledgers`, `GET /v1/ledgers/{ledger}`, list with cursor
   pagination) on top of the existing `app.Ledgers` use case. Follow the add-an-endpoint
   workflow in CONTRIBUTING.md.
3. Ledger-number allocation per (ledger, asset) in Postgres (ADR-0005) and the accounts table
   with resolved flags and code (ADR-0006), then `GET /v1/ledgers/{ledger}/accounts/{address}`
   backed by `lookup_accounts`.
4. Transactions: idempotency records with minted IDs, lazy account creation, linked
   `create_transfers`, problem types for `exceeds_credits` and friends, the reconciler
   (ADR-0007). Add the 422 problem type for Idempotency-Key reuse.
5. Holds (ADR-0010), revert, aggregated balances and volumes (ADR-0004).
6. Operational follow-ups: a custom seccomp profile instead of `unconfined`, Docker image
   digests pinned by Renovate, multi-arch image build in a release pipeline.
7. The posting-rule DSL and the CDC feed (TigerBeetle AMQP) for the console, both future work in
   docs/architecture.md.

## Loose ends to be aware of

- `.design/` at the repository root holds local design references for the future console.
- The console is a single status page on purpose. Its styling is plain CSS; no component library
  has been chosen.
- The access log does not record 1xx statuses; the readiness latency is rounded to whole
  milliseconds and shows 0 ms locally.
- The hook output (lint-staged, Redocly) prints emojis and symbols in the terminal. The rule "no emojis" applies to repository
  content, which is checked with `grep -rnP '[^\x00-\x7F]'` over tracked files.
