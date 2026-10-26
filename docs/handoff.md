# Handoff

Updated on 2026-10-26 for the next coding agent. It records what exists, what was verified, what
was decided along the way, and what to do next. Read AGENTS.md first for the conventions; read
this file for the state of the work.

## Where things stand

### Authentication, authorization and multi-tenancy (in progress)

Planned in `docs/plans/auth/plan.md`, approved by the owner on 2026-10-26, on the branch
`feat/auth` (worktree `../tigerbeetle-platform-worktrees/auth`, so the owner's uncommitted edits
in the main checkout stay untouched). ADR-0026 to ADR-0032 record the decisions.

- Phase 0 (documentation) is done: the plan, ADR-0026 to ADR-0032, the amended status lines, and
  pointers in AGENTS.md and docs/architecture.md.
- Phase 1 (tenancy in the data model) is done, from dda1c65:
  - Migration 00009: `organizations`, and `organization_id` leading the keys of `ledgers` and the
    eight tables that name a ledger. Pre-alpha data is in the organization `default`
    (`ledger.DefaultOrganizationID`, `00000000-0000-0000-0000-000000000001`).
  - `ledger.Tenant` travels in the context; repositories read it with `platformpg.TenantOf` and
    fail closed without one. Until phase 2, the HTTP middleware, the gRPC interceptors and the
    RPC dispatcher put `ledger.DefaultTenant` in every context.
  - Write records carry their organization; the reconciler settles each record inside it.
  - `apps/api/migrations/tenancy_test.go` fails when a query touches a tenant table without
    `organization_id`; `-- tenant-free:` comments mark the four queries that may cross.
  - Deviation from the plan, recorded in ADR-0027: composite keys and the tenant from the
    context, not a platform-wide ledger ID (that would have changed about 350 signatures).
  - Verified: `just lint`, Go unit and integration tests (with an isolation test per store),
    `just generate-check`. Not run: the e2e suite against the compose stack.
  - Known gap for phase 3: the reconciler status (`getReconcilerStatus`) still counts pending
    writes across every organization; it must be filtered by tenant once tenants are real.
- Phase 2 (authentication) is done, from 1bb4d50:
  - Every call except liveness, readiness, gRPC health and reflection carries a bearer token,
    verified by `internal/platform/oidc` (go-oidc v3.21.0) and turned into a principal and a
    tenant by `internal/platform/authn`, on REST, GraphQL, gRPC, AMQP RPC and NATS RPC alike.
    The tenant comes from `X-Organization` (gRPC `x-organization`, broker `organization`),
    checked against the token's `organization` claim; anything else is 404
    organization-not-found, a suspended organization 403, a ledger call without one 400.
  - The access module resolves aliases to the platform's `organizations` rows.
  - The local stack runs Keycloak 26.8.1 (port 18080, admin / admin, realm `tbp` from
    `deploy/compose/keycloak/realm.json`, every user's password `password`) and Mailpit (18025).
    Keycloak names `http://localhost:18080` in tokens; the API reads discovery at
    `http://keycloak:8080` (`AUTH_DISCOVERY_URL`). Requesting the scope `organization:*` gives
    the list of a person's organization aliases, verified against Keycloak on 2026-10-26.
  - The one-shot `organizations-seed` service inserts the platform rows of acme and globex until
    staff create organizations (phase 5).
  - The e2e suite and `scripts/seed-demo.sh` sign in as ada through the development-only client
    `tbp-cli` (password grant). Verified: `just lint`, Go unit and integration tests (with
    `TestAuthenticationEndToEnd`), and `just test-e2e` against a second compose stack.
  - Running a second stack next to the main one needs another subnet: the compose network pins
    `172.30.231.0/24` for the CDC job. An override file with `172.30.232.0/24` for the network,
    TigerBeetle and RabbitMQ, plus other host ports and `COMPOSE_PROJECT_NAME`, works.
  - The console cannot sign in until phase 4: its calls now answer 401.
  - Deferred to phase 7: `/readyz` reporting Keycloak (it changes the readiness payload of
    REST, GraphQL and gRPC).
- Phase 3 (authorization) is done, from 001cd1e:
  - Roles and permissions in `kernel/ledger/permission.go`; `grants` (migration 00010) in the
    access module, whose authorizer reads the caller's grants on every call. Organization roles
    member, admin, owner; ledger roles viewer, operator, admin; staff hold only
    `platform.organizations`.
  - `application/authorize.go` in ledgers and bookkeeping wraps every use case;
    `TestEveryUseCaseIsChecked` calls each method behind a refusing authorizer and fails when one
    reaches the real use case. The ledger list is filtered in SQL. The reconciler status shows
    only the organization's own backlog.
  - Idempotency keys replay only for their actor (migration 00011, ADR-0031).
  - The spec (SpecVersion 0.2.0) declares `bearerAuth` and `X-Organization`; 33 accepted
    breaking lines are in `packages/openapi/oasdiff-ignore.txt`. `jsonBodies` is gone: a non-JSON
    body answers 415 (`unsupported-media-type`) from the `jsonOnly` middleware, because Huma maps
    an unknown content type to 400 when body validation is skipped. The REST client takes
    `accessToken` and `organization` options; orval strips `X-Organization` like `X-Request-Id`.
  - Dev users keep fixed Keycloak IDs; `deploy/compose/dev-seed.sql` grants their organization
    roles (ada owns default and acme, carol is acme admin, bob owns globex).
  - Deviation: the plan's "creator becomes the ledger's admin" is not built. Only organization
    admins and owners hold `org.ledgers`, and they already hold every ledger permission.
  - Verified: `just lint`, unit, integration (`TestAuthorizationEndToEnd` covers REST, GraphQL and
    gRPC, two organizations with the same ledger name, viewer and operator, the key rule),
    `just test-e2e` and `scripts/seed-demo.sh` against Keycloak in the second compose stack.
- Phase 4 (console sign-in) is done, from 367a382:
  - The API's backend-for-frontend: `/auth/login`, `/auth/callback`, `/auth/logout`,
    `/auth/session` in `access/interfaces/web`; sessions (migration 00012) keep only the hash of
    the cookie's ID and the tokens sealed by `platform/sealed`. The middleware turns the cookie
    into a bearer token, refreshes it before it expires (one refresh per session), and refuses a
    cookie-authenticated write whose `Sec-Fetch-Site`/`Origin` is not the console's.
  - Deviation: the person, their organizations and roles come from `GET /auth/session`, not from
    a GraphQL `viewer` query. It needs no schema change and is the console's own route.
  - The console: `_console` and `/` call `requireSession`; pages of an organization moved under
    `src/routes/_console/$org/`; every link passes `org` (81 links in 26 files); switching
    organizations drops the cached answers of the previous one; the organization switcher, the
    account menu with sign-out, `useCan` and `canCreateLedgers` hide what a role cannot do; a 401
    anywhere sends the browser to sign in. "Copy as curl" adds `Authorization: Bearer $TOKEN` and
    `X-Organization`.
  - Verified: Go unit and integration tests (`TestConsoleSignInEndToEnd` runs the code flow, the
    cookie, the CSRF check, a refresh and sign-out against the test issuer), the console's tests,
    and a real browser against Keycloak in the second stack: sign-in as carol, the Acme and Globex
    ledger lists, every ledger page, a GraphQL write with the cookie, sign-out. The second stack's
    console port needed its redirect URI added to the realm's client at run time.
  - Note: agent-browser's clicks did not open the base-ui menus in that session (dispatched DOM
    pointer events did); the main stack's menus opened with the same tool.
- Rebased onto `feat/rfc9457-problems` on 2026-10-26, which landed first and took ADR-0025
  (problem details). The auth ADRs moved to 0026-0032 (bdaedb9) and the auth reasons are entries
  in `internal/platform/problem/catalog.go`, so their types are absolute URIs into
  `docs/problems.md`. The hashes above are after the rebase. Some commits in the middle of the
  rebased range do not build on their own (they call `writeProblem`, which the problems branch
  renamed to `restapi.WriteProblem`); the tip builds and passes `just lint`, `just test`,
  `just test-integration` and `just generate-check`. Bisecting through phases 1 to 4 needs
  `git bisect skip` on those commits.
- Phase 5 (organizations and members) is done, from fc204eb:
  - The O6 spike settled how a person without an account is invited (ADR-0027): the API creates
    the account and Keycloak emails the required actions; a person with an account gets
    Keycloak's invitation. Keycloak's own registration link was rejected: it makes a managed
    member. Roles go to the person's subject at once; there is no pending-grant table (deviation
    from plan 5.3, recorded there).
  - `internal/platform/keycloak` is the admin API client, used by the service account `tbp-admin`
    (realm file, `AUTH_ADMIN_CLIENT_ID`/`AUTH_ADMIN_CLIENT_SECRET`; without them the calls answer
    503). `access/infrastructure/keycloak` adapts it to the `Directory` port.
  - Staff: list, create (retry-safe by alias), rename, suspend with a reason, reactivate.
    Members: list (every member may read it), invite with an organization role and ledger roles,
    set roles, remove or leave, resend and withdraw invitations, ledger roles by ledger admins.
    REST (`organizations`, `members` tags) and GraphQL; no broker or gRPC methods (ADR-0027).
  - The console follows `.design/auth.pen`: Members with an Invitations tab and the invite
    dialog, Member, a ledger's Access, Roles and permissions, Organization settings, Profile,
    the staff's Organizations, and the suspended and welcome pages. `docs/console.md` lists them.
  - Migration 00013 was extended after its first commit (creator name, suspension reason, granter
    name). A database that ran the first version needs it re-run: drop `created_by_issuer` and
    `created_by_subject`, delete version 13 from `goose_db_version`, restart the API.
  - Verified: Go unit and integration tests (`TestOrganizationsAndMembersEndToEnd` with the
    in-memory directory, store tests for the new queries), console tests, every flow against
    Keycloak 26.8.1 in the second stack (staff create, invite with and without an account, the
    setup email to a working sign-in, suspension and the 403 with its reason), and every new page
    in a browser as ada (owner), staff and ivy (owner of a suspended organization).
  - Not done from the design: the identity provider and email domain panels, passkeys and active
    sessions (Keycloak's account console has them; the profile links to it), API key and machine
    client counts (phase 6), "last opened" in the organization chooser. The pages Keycloak itself
    renders (sign-in, invitation, setup, emails) are designed in `.design/auth.pen` and are the
    next step, as a Keycloakify theme (the owner's suggestion).

A migration is under way, planned in `docs/plans/ddd-transports/plan.md` and approved by the
owner on 2026-10-25: bounded-context modules, GraphQL, gRPC, AMQP RPC, NATS RPC and Kafka, the
REST spec and GraphQL schema generated from Go, an orval REST client, the console on GraphQL and a
root justfile, in phases 0 to 7 with a review stop after each.

- Phase 0 (documentation) is done and was approved: ADR-0015 to ADR-0024, the amended status
  lines, and pointers in AGENTS.md and docs/architecture.md.
- Phase 1 (the justfile) is done and was approved. The root `justfile` is the entry point for
  every task; the root `package.json` keeps only `prepare`; Turborepo runs the TypeScript
  packages only; the pre-commit hook and every CI job call just recipes.
- Phase 2 (the REST spec generated from Go, ADR-0018 and ADR-0019) is done: REST on Huma,
  the spec from `cmd/openapi`, the oasdiff gate, the orval client and the console on it. On
  2026-10-25 the owner asked for phases 3 to 7 in one go, without review stops in between.
- Phase 3 (bounded-context modules, ADR-0017) is done, from 6d626c6:
  - `internal/kernel/ledger` is the shared kernel; `internal/platform/{postgres,tigerbeetle,
    readiness}` own the process-wide resources; `internal/bootstrap` builds and runs everything
    (Runners under an errgroup; the reconciler behind `RECONCILER_ENABLED`, the module REST
    operations behind `REST_ENABLED`).
  - `internal/modules/ledgers`, `bookkeeping` and `health`, each in domain, application,
    infrastructure and interfaces layers, with their own sqlc blocks. Modules talk through the
    ports of plan section 4.5, wired in bootstrap.
  - `internal/transport/rest` lists every REST operation; the spec stayed byte-identical.
  - depguard enforces the layering, and `just lint-depguard-probe` (in CI) proves every rule
    fires on its fixture in `apps/api/testdata/depguard`.
- Phase 4 (GraphQL, ADR-0020 and ADR-0023) is done, from e359d28:
  - `cmd/graphqlschema` writes `packages/graphql/schema.graphql` from the modules'
    `interfaces/graphql` registrations with the in-repo generator `internal/platform/graphqlschema`;
    gqlgen generates the executor in `internal/transport/graphql/exec` from it.
  - `POST /graphql` behind `GRAPHQL_ENABLED`: complexity 1000, depth 12, 1 MiB, introspection on,
    errors with the problem members in `extensions`. A write made over GraphQL replays over REST
    under the same key.
  - The console's ledger slices (ledgers, accounts and balances, transactions, holds, requests,
    settings, overview) run on GraphQL; the System page stays on the REST client. The write
    forms read an address's account and rule in one request.
- Phase 5 (gRPC, ADR-0021) is done, from 2cbe027:
  - `packages/proto`: one package, `tigerbeetle.platform.v1`, a service per REST tag, 33
    methods (the 35 REST operations less liveness and readiness, which `grpc.health.v1` covers).
    `just lint-proto` runs `buf lint`; `just proto-breaking` and the CI `proto` job run
    `buf breaking`. buf writes `internal/platform/grpcapi/platformv1`.
  - Module services in `interfaces/grpc/v1` over the same use cases and `dto` types;
    `internal/transport/grpc` registers them and joins LedgerService and OperationsService,
    which span modules.
  - `internal/platform/grpcserver`: request ID, access log, error and recovery interceptors,
    reflection, a drain bounded by `SHUTDOWN_TIMEOUT`, grpc-go's logger routed to slog. Errors
    carry `ErrorInfo` (AIP-193 reason from `problem.ErrorReason`), `RetryInfo` and
    `BadRequest`.
  - `GRPC_ENABLED` (off) and `GRPC_ADDR` (`:9090`); compose passes the switch and publishes
    `GRPC_PORT`.
- Phase 6 (AMQP and NATS RPC, ADR-0021) is done:
  - `internal/platform/rpc`: the envelope, the router of `v1.<operationId>` methods, the worker
    pool and the deadline rules; `amqp` (connection supervisor), `amqprpc` (durable queue
    `tbp.rpc.v1`, direct reply-to) and `natsrpc` (micro service, queue group `tbp-api`), each
    with a test client.
  - Every module registers one method per REST operation in `interfaces/rpc/v1`;
    `internal/transport/rpc` builds the router and a test keeps it equal to the REST operations
    (35).
  - Switches `AMQP_RPC_ENABLED`, `NATS_RPC_ENABLED` and their settings; readiness reports the
    enabled brokers and answers 200 `degraded` when one is down.
  - Compose profile `brokers` (`just compose-up-brokers`) and `just test-e2e`.
- Phase 7 (Kafka, ADR-0022) is done:
  - The `brokers` profile adds `apache/kafka:4.3.2` and the TigerBeetle CDC job (the server
    image's `amqp` subcommand), with a fixed subnet and addresses for `tigerbeetle` and
    `rabbitmq`, and a RabbitMQ definitions file for the exchange `tbp.cdc` and the queue
    `tbp.ledger-events.relay`.
  - `internal/platform/kafka` (franz-go, idempotent, acks=all); bookkeeping's
    `interfaces/cdc` (consumer and exact decoder), `application/relay_ledger_event.go` and
    `infrastructure/kafka` (publisher); `KAFKA_EVENTS_ENABLED` and its settings; readiness
    reports `kafka`.
  - `docs/ledger-events.md` is the record format.
- All seven phases are done; the owner reviews phases 2 to 7 together.

Where the commits are: phases 0 to 4 and the first three commits of phase 5 (up to 725f75b)
are on `feat/console` in the main checkout. While phase 5 ran, the main checkout gained
uncommitted edits that were not part of this work (`.vscode/settings.json`, a reformatted
`bookkeeping/interfaces/rest/v1/accounts.go` that fails gofumpt, and two other Go files). The
pre-commit hook lints the whole module, so every later commit was made in a separate worktree,
`../tigerbeetle-platform-worktrees/transports`, on the branch `feat/console-transports`, which
starts at the tip of `feat/console`. Fast-forward `feat/console` to it once those edits are
committed or set aside.

The API implements the ledger surface (ADR-0004 to ADR-0010, details in ADR-0013) and the console
covers every page of `docs/console.md` (ADR-0014). The work is on the `feat/console` branch, not
merged and not pushed, so CI has never run on GitHub. Everything below was verified on macOS 27
(arm64, OrbStack as the Docker engine).

Working and verified:

- After phase 7: `cmd/api/kafka_integration_test.go` runs TigerBeetle, Postgres, RabbitMQ, the
  CDC job and Kafka in containers. One transaction gives one record per leg with string amounts
  and the ledger, asset and addresses; a hold that times out gives a `two_phase_expired` record;
  stopping the relay and the CDC job in the middle of 30 writes and starting them again leaves no
  gap (no duplicate either in that run, which proves nothing). On the compose stack the relay
  drained the 22 events the CDC job had queued before it existed, and `/readyz` reported kafka.
- After phase 6: the platform integration tests (spike S5) show each request running once across
  two replicas on both brokers, concurrency capped at `RPC_CONCURRENCY`, deadlines kept, a
  dropped AMQP connection redialled, and a NATS timeout told apart from no responders.
  `cmd/api/rpc_integration_test.go` runs two replicas through bootstrap: one write replays over
  AMQP, NATS and REST and moves the balance once, and 40 writes across both brokers each apply
  once. `just test-e2e` passed against `just compose-up-brokers` on every transport, and
  stopping NATS turned `/readyz` into 200 `degraded` until it came back.
- After phase 5: `cmd/api/grpc_integration_test.go` passes against real Postgres and
  TigerBeetle. A gRPC write replays over REST and gRPC under its key, a reused key is
  `IDEMPOTENCY_KEY_REUSED`, a decline carries `ErrorInfo` with its posting and results, and a
  lost write answers `WRITE_PENDING` then `REQUEST_IN_FLIGHT`, both with `RetryInfo`, until the
  reconciler settles it. The compose stack with `GRPC_ENABLED=true` answered `buf curl` through
  reflection (health, create ledger and transaction, a decline with `ErrorInfo`, and a REST
  replay of a gRPC key), and stopped cleanly with exit code 0 while 40 gRPC writes were in
  flight.
- After phase 4: `TESTCONTAINERS_RYUK_DISABLED=true just test-integration` passes, including
  `cmd/api/graphql_integration_test.go` (a ledger, a transaction replayed over REST under the
  same key, a reused key, nested reads with null for a missing hold and ledger, and a decline with
  its code, reason and posting in the extensions). The console was driven in
  a browser against the compose stack and seed data: every ledger page renders from GraphQL, the
  request drawer shows a stored decline, and a hold voided from the console commits and updates
  the counts.
- After phase 3: `just lint`, `just lint-depguard-probe`, `just typecheck`, `just test`,
  `just build` and `just generate` (no diff) pass, and the pre-commit hook formats and lints a staged Go file and lints a staged
  spec through just.
- After the switch to the generated client, the console was driven in a real browser against
  the API and `scripts/seed-demo.sh` data: every page loads, a hold voided from the console is
  stored under the console's key, and a taken ledger name shows the 409 problem with its request
  ID.
- `TESTCONTAINERS_RYUK_DISABLED=true just test-integration` passes against the Huma server,
  including the replay test (`cmd/api/replay_integration_test.go`). The end-to-end test covers
  ledgers, rules, event types, multi-posting and multi-asset transactions, replays and key reuse,
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

| Area                           | Version                                                                                                                                                                                                        |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go                             | 1.27.3 (`go 1.27.0`, `toolchain go1.27.3` in go.mod; builder `golang:1.27.3-trixie`)                                                                                                                           |
| TigerBeetle server             | `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`                                                                                                                                                                       |
| TigerBeetle Go client          | `github.com/tigerbeetle/tigerbeetle-go` v0.17.10 (see exception below)                                                                                                                                         |
| Runtime image                  | `gcr.io/distroless/base-debian13:nonroot`                                                                                                                                                                      |
| Postgres                       | `postgres:18.6-trixie`, pgx v5.11.0, sqlc v1.31.1, goose v3.29.0 (library only)                                                                                                                                |
| REST                           | Huma v2.39.1 (`humago` on the standard ServeMux); oasdiff 1.34.1 (installed by `scripts/install-tools.sh`, sha256 pinned)                                                                                      |
| Kafka                          | franz-go v1.22.2; `apache/kafka:4.3.2` pinned by digest; the CDC job runs from `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`, the server's own image                                                                |
| Broker RPC                     | amqp091-go v1.16.0, nats.go v1.55.0, testcontainers modules rabbitmq and nats v0.44.0; `rabbitmq:4.4.0-management` and `nats:2.15.1-alpine`, both pinned by digest in the `brokers` profile                    |
| gRPC                           | grpc v1.84.0, protobuf v1.36.12; buf v1.75.0, protoc-gen-go v1.36.12, protoc-gen-go-grpc v1.6.2 (installed by `scripts/install-tools.sh`, sha256 pinned)                                                       |
| GraphQL                        | gqlgen v0.17.98 (a `go tool`), gqlparser v2.5.64; console: graphql-codegen 7.4.6 with client-preset 6.2.3, graphql 17.0.2, @graphql-typed-document-node/core 3.2.0                                             |
| golangci-lint, govulncheck     | v2.15.0, v1.9.0 (installed by `scripts/install-tools.sh`)                                                                                                                                                      |
| just                           | 1.59.0 (installed by `scripts/install-tools.sh`, sha256 pinned per platform)                                                                                                                                   |
| env, go-cmp, testcontainers-go | v11.4.1, v0.7.0, v0.44.0                                                                                                                                                                                       |
| Node, pnpm, Turborepo          | 24.22.1, 12.15.0, 2.11.10                                                                                                                                                                                      |
| Console                        | React 19.3.1, Vite+ 1.4.0 (Vite 8.3.5), TypeScript 7.0.2, TanStack Router 1.170.46, Query 5.104.5, Form 1.33.5, Table 9.3.1, Pacer 0.24.1, Base UI 1.9.0, shadcn 4.22.1 (base-nova), Tailwind 4.3.3, zod 4.6.7 |
| Generators and lint            | orval 8.44.0 (packages/api-client), Redocly CLI 2.71.0, Vite+ 1.4.0 (Oxfmt 0.74.0, Oxlint 1.89.0, git hooks), commitlint 21.2.4                                                                                |
| GitHub Actions                 | checkout v7.0.1, setup-go v7.0.0, setup-node v7.1.0, pnpm/action-setup v6.1.0, golangci-lint-action v9.3.0, setup-buildx v4.4.1, build-push v7.4.0, all pinned by SHA, runners `ubuntu-24.04`                  |

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
7. **ky 2 read the body before `beforeError` ran**, so decoding problem+json or the 503
   readiness body in a hook failed with "Body has already been read". ky is gone since phase 2;
   the fetch layer in `packages/api-client/src/client/fetcher.ts` keeps the lesson and reads
   every body once, then inspects the status.
8. **TypeScript 7.0.2 has no JS compiler API.** openapi-typescript needed it and ran on
   TypeScript 5.9.3 until phase 2 removed it; orval does not use it. TypeScript 5.9.3 is still in
   the lockfile as an auto-installed peer of the root's ESLint plugins, because
   `@typescript-eslint` accepts only `<6.1.0`; nothing compiles with it.
9. **Corepack is not bundled with Node 25+** and was absent on the owner's machine. The
   `packageManager` field pins pnpm; pnpm switches itself. Do not reference `corepack` in docs.
10. **goose's CLI drags ClickHouse, MySQL, MSSQL and more into go.mod** through the `tool`
    directive. goose is a library only here (`internal/adapters/postgres/migrate.go`, with a
    Postgres session lock). No `go tool` is left since phase 2 removed oapi-codegen.
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
    `import { type A } from './x'` leaves `import './x'` in the bundle, which fails when `./x` is
    a `.d.ts` with no runtime module. The lint autofix
    (`import/consistent-type-specifier-style: prefer-inline`) produces that form. The orval
    model files are `.ts` modules, so the client no longer needs the workaround.
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
    its YAML dependencies past the versions it compiled with, and nothing noticed because only
    `go tool oapi-codegen` used them. Phase 2 removed the tool and `go mod tidy` dropped them; a
    `go tool` dependency is only exercised when the tool runs, so keep generators in `just
    generate`.
29. **Only the comment line right above a recipe shows in `just --list`.** Recipes with a longer
    comment use `[doc('...')]` for the listed text.
30. **`vp check` at the root checks formatting; the console's `lint` task does not.** Ten console
    components with stray blank lines passed `pnpm lint` but failed `vp check`, which CI's web job
    runs. `just lint` runs both.

31. **Huma skips embedded unexported structs when it collects parameters.** The shared parameter
    structs of `httpapi/v1` (`LedgerPath`, `Page`, `ResourcePath`, `WriteKey` and others) are
    exported for that reason. A parameter declared in `op.Parameters` wins over the struct tag,
    which is how descriptions with backticks, impossible in a Go tag, reach the spec.
32. **Huma's defaults change the spec.** It marks structs `additionalProperties: false`, query
    parameters `explode: false`, and Go `int` as `format: int64`; `restapi.Spec` and a type
    alias undo each one, because oasdiff calls an added format on a request integer breaking.
    Its package globals (`NewError`, `DefaultArrayNullable`) are set once behind a `sync.Once`.
33. **oasdiff needs `--flatten-params` here.** The hand-written spec declared path parameters per
    path, Huma declares them per operation; without the flag `oasdiff breaking` reports 42
    `new-request-path-parameter` errors for parameters both sides have. `--exclude-elements`
    exists for `oasdiff diff` only.
34. **orval's esbuild has an install script pnpm refuses to run.** pnpm 12 fails the install
    with `ERR_PNPM_IGNORED_BUILDS`; `pnpm-workspace.yaml` sets `allowBuilds: esbuild: false`,
    since esbuild finds its binary through its platform package without the script.
35. **orval turns every header parameter into an argument.** `headers: true` makes the
    Idempotency-Key of the money writes a required typed argument, but would also give every
    read an X-Request-Id argument. `orval.config.ts` removes X-Request-Id from the generator
    input; the fetch layer sets it on every call.
36. **Generated query types reject `undefined` under `exactOptionalPropertyTypes`.** The
    console's filter objects hold `undefined` for unset filters, so list calls go through
    `queryParams()`, which drops undefined, null and empty members as the ky client did.
37. **A fetch timeout is a `DOMException` named `TimeoutError`**, not an `Error` subclass as ky's
    was, and jsdom's `DOMException` is not an `instanceof Error`. `toProblem` checks for it by
    type.
38. **`Date.parse('-1')` is a valid date in V8** (year -1). `parseRetryAfter` only parses an
    HTTP date that starts with a day name, and reads the asctime form, which has no zone, as GMT.
39. **commitlint reads a body line that starts with `word:` as a footer** and warns
    "footer must have leading blank line". Reword such lines.

40. **A comma inside a YAML flow mapping splits the value.** `{pkg: x, desc: a, b}` in
    `.golangci.yml` gives `desc: a` and a stray key `b`, and depguard prints half the message.
    Quote any `desc` with a comma.
41. **depguard globs match the probe fixtures.** The fixtures under `apps/api/testdata/depguard`
    mirror the real paths, so `**/internal/kernel/**` matches them too; `./...` skips
    `testdata`, and golangci-lint lints such a package only when it is named explicitly, which is
    what the probe does. A fixture under `cmd/` must not be `package main` without a `main`.
42. **commitlint caps the subject line at 100 characters**, and the commit-msg hook rejects a
    longer one after the pre-commit hook already ran.
43. **`just fmt-go-files` takes paths relative to `apps/api`**, because it changes into that
    directory; the pre-commit hook passes absolute paths.
44. **A ledger's assets and address count are a second query now.** The ledgers module reads its
    rows, then asks bookkeeping for the footprints of the whole page in one query
    (`LedgerFootprints`, one row per requested name), as plan section 4.5 planned.

45. **commitlint rejects a sentence-case subject**, so a subject that starts with "GraphQL" fails;
    write "serve a GraphQL schema" or "graphql end-to-end".
46. **Oxfmt formats `.graphql` files.** `vp check --fix` reformats the generated schema and the
    generator's golden files, and `just generate-check` then fails. The root `vite.config.ts`
    ignores `packages/graphql/schema.graphql`, `apps/console/src/graphql/gen/**` and the
    generator's testdata.
47. **gqlgen crashes (nil Tuple) when it autobinds `platform/graphqlschema`**, whose generic
    functions are named `Query` and `Mutation`. `gqlgen.yml` binds `PageInfo` explicitly instead,
    and the modules name their resolver types `Queries` and `Mutations`.
48. **Two embedded types with the same name are ambiguous in Go.** The root resolvers embed the
    `Queries` of three modules through type aliases (`healthQueries`, `ledgersQueries`), which
    give each embedded field its own name.
49. **graphql-codegen's client preset imports `@graphql-typed-document-node/core` once a document
    exists**, so the console lists it as a devDependency. Fragments are found by name across
    files: a document can spread a fragment declared in another slice without importing it.
50. **`present()` strips nulls deeply**, which would change stored JSON. The request detail keeps
    the stored request and response bodies as they are and presents only the record's members.

51. **commitlint reads any body line that starts with `word:` as a footer**, not only the
    first one (fact 39): "tag: ledgers, ..." and "first: request ID" wrapped onto the start of a
    line both warned. Reword the line or move the colon.
52. **`buf breaking` needs the protos on the base.** `just proto-breaking` exits 0 with a note
    when the base revision has no `packages/proto`, the case of the first pull request. buf's
    git input accepts a remote-tracking ref (`origin/main`) and works from a git worktree.
53. **A gRPC service implemented by two modules needs the stubs without
    `mustEmbedUnimplemented...`.** `buf.gen.yaml` sets `require_unimplemented_servers=false`,
    so `internal/transport/grpc` joins two module types into one service by embedding, and a
    method neither has is a compile error instead of an Unimplemented answer.
54. **grpc-go's health server is static.** The health module overrides `Check` for the whole
    server (empty service name) to probe on demand, as `/readyz` does, and refreshes the
    embedded server's status so `Watch` sees it.

55. **`grpclog.SetLoggerV2` races with gRPC in use.** Setting it from `grpcserver.New` failed
    `go test -race` now and then, because parallel tests already ran gRPC. `cmd/api` calls
    `grpcserver.RouteLogs` once at start, before anything touches gRPC.
56. **oasdiff 1.34.1 rates a new response enum value as an error**, not the warning the plan
    expected. The `degraded` readiness status is accepted on purpose (ADR-0021), so
    `packages/openapi/oasdiff-ignore.txt` lists exactly its two lines and `just spec-diff` passes
    the file as `--err-ignore`. Remove the lines once `main` has the change.
57. **nats.go refuses `FlushWithContext` without a deadline**, so a readiness probe called with a
    plain context failed every time. `natsrpc.Server.Ready` sets one when the caller did not.
58. **The testcontainers nats module starts the server with `-DV -js`.** devstack overrides the
    command with `-m 8222`, as compose runs it: no JetStream, no debug logs.
59. **A compose `depends_on` on a profile service needs `required: false`**, or the default stack
    refuses to start without the brokers.
60. **RabbitMQ direct reply-to needs the client to consume `amq.rabbitmq.reply-to` with automatic
    acknowledgement, on the channel it publishes the request from.** The server replies to the
    `reply_to` name through the default exchange.

61. **testcontainers v0.44 refuses `host:container` in `ExposedPorts`.** A fixed host port goes
    through `HostConfigModifier` and `PortBindings` (`devstack.StartKafka`).
62. **Kafka hands clients the address it advertises**, so a random mapped port does not work:
    devstack picks a free host port first and advertises `127.0.0.1:<port>`.
63. **RabbitMQ's imported definitions replace its default user.** The compose definitions file
    holds `guest` with the hash `rabbitmqctl hash_password guest` printed; the official image
    still lets `guest` connect from other containers.
64. **The CDC job needs its exchange before its first publish.** Compose declares it in the
    definitions file; the relay also declares the exchange, queue and binding when it connects,
    so tests start the relay before the job.
65. **A stopped `bootstrap.App` has closed its Postgres pool and TigerBeetle client.** A test that
    stops one relay process and starts another writes through a third process, as two replicas
    would.
66. **franz-go's sub-packages need their own go.sum lines**: `go get
    github.com/twmb/franz-go/pkg/kgo@v1.22.2` added kmsg and lz4.

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

On 2026-10-26 the owner chose, for RFC 9457 (ADR-0025), absolute problem types that point at the
generated catalogue on GitHub (`docs/problems.md#<reason>`, the same in every deployment), a
`not-ready` problem for the 503 of `/readyz`, and an `errors` member for the input errors Huma
raises. ADR-0013, 0018 and 0021 carry the amended status.

## Code map

API (`apps/api`):

- Kernel `internal/kernel/ledger`: amounts (u128), addresses, assets, rules and their matching,
  metadata, event types, the six error sentinels and `Problem`, the clock, page cursors.
- Modules `internal/modules/<module>/{domain,application,infrastructure,interfaces}`:
  - `ledgers`: ledgers, account rules, event types; `application/directory.go` and
    `policy_queries.go` are what bookkeeping asks of it.
  - `bookkeeping`: `application/writeflow.go` is the one write protocol of ADR-0007 for
    transactions, reverts, holds, captures and voids; `reconciler.go`; the account, transaction,
    hold, request, overview and footprint queries. `infrastructure/tigerbeetle` holds the bounded
    calls and `id.go`.
  - `health`: liveness, readiness and service information, an interfaces layer only.
  - Each has `interfaces/dto` (wire types, request bodies and list answers included), `rest/v1`,
    `graphql`, `grpc/v1` and `rpc/v1`.
- Transports `internal/transport/rest` (the operation list), `internal/transport/graphql`
  (handler, resolver root, schema registry, generated `exec`), `internal/transport/grpc` (the
  service registry) and `internal/transport/rpc` (the broker router).
- `internal/bootstrap`: resources, module construction (`modules.go`), servers and runners.
- `internal/platform`: config, logging, httpserver, postgres, tigerbeetle, readiness, restapi,
  graphqlschema, grpcserver, grpcapi (generated `platformv1` and conversions), rpc, amqp,
  amqprpc, natsrpc, problem, scalar, requestid.
- `apps/api/e2e`: the `e2e`-tagged suite against the compose stack.
- End-to-end tests in `cmd/api/*_integration_test.go`.

Console (`apps/console/src`):

- `routes/`: one file per URL under `_console/`; `__root.tsx` provides Query and tooltips.
- `features/`: `ledgers`, `overview`, `transactions` (with `new-transaction/`), `accounts`,
  `balances`, `assets`, `holds`, `requests`, `settings`, `system`, `preferences`, `search`. Each
  `api.ts` holds its GraphQL documents and query options; `system` uses the REST client.
- `graphql/gen`: the typed documents graphql-codegen writes.
- `components/ui` (shadcn, restyled), `components/ledger` (domain display), `components/form`
  (TanStack Form hook), `components/data-table`, `components/layout` (shell, page, breadcrumbs).
- `lib/`: `graphql/` (client, errors, `present()`), `money.ts`, `preflight.ts`, `net-effect.ts`,
  `validation.ts`, `idempotency.ts`, `time.ts`, `preferences.ts`, `curl.ts`, `api.ts`,
  `query-client.ts`.
- `styles/globals.css`: the Ledger Instrument tokens and their mapping to shadcn variables.

## Suggested next steps, in order

1. The owner reviews phases 2 to 7 together; the combined summary of changes, versions and
   deviations went to the owner with the last commit.
2. Fast-forward `feat/console` to `feat/console-transports` (see "Where the commits are"), after
   resolving the uncommitted edits in the main checkout; `bookkeeping/interfaces/rest/v1/accounts.go`
   changed on both sides. Remove the worktree with `git worktree remove`.
3. Push to GitHub, open the pull request for `feat/console`, make CI green, install Renovate.
4. Review the console against `.design/home.pen` with the owner; the screens were followed
   closely but not pixel by pixel (the account tree and the pending result view simplify the
   design's activity log and cancel button, which the API does not support).
5. Transaction list filters on metadata and on "reverted" need API support first.
6. Operational follow-ups: a custom seccomp profile instead of `unconfined`, image digests pinned
   by Renovate, a multi-arch image and a static console image in a release pipeline.
7. The posting-rule DSL and the console's live feed, future work in `docs/architecture.md`; the
   Kafka relay of phase 7 is the first CDC consumer.
8. Once `main` holds the `degraded` readiness status, remove its two lines from
   `packages/openapi/oasdiff-ignore.txt`.

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
