# Plan: bounded-context modules, multiple transports, specs from code, justfile

Date: 2026-10-25. Written at 2cfa3a3; revised the same day with the owner's answers to the open
questions (section 23) against the baseline a82f2c2 on `feat/console` (section 19, phase 0).
Status: approved by the owner on 2026-10-25, with the answers of section 23. The ADRs of section
18 are written first (phase 0).

Inputs, copied byte for byte next to this file: `research.md` (references, versions, TigerBeetle
CDC) and `audit.md` (repository inventory and conflicts). Every load-bearing fact taken from them
was re-checked against the code; where this plan disagrees with them it says so.

How to read it. Sections 1 to 4 say what changes and why. Sections 5 to 17 are the design, one
concern per section. Sections 18 and 19 are the delivery: ADRs, then seven phases of small
commits. Sections 20 to 23 list every conflict, the lead's review points, the owner decisions
carried over from the audit, and the owner's answers to the open questions with the few questions
those answers raised. Short "Go note" paragraphs explain choices that are not obvious to someone
new to Go.

Words used throughout:

- module: one bounded context under `apps/api/internal/modules/<name>`, with the layers `domain`,
  `application`, `infrastructure`, `interfaces`.
- kernel: the small set of domain types shared by every module (IDs, amounts, addresses, errors).
- platform: shared infrastructure with no business rule (config, logging, servers, the Postgres
  pool, the TigerBeetle client, the REST and GraphQL plumbing).
- transport: one way of calling the use cases: REST, GraphQL, gRPC, AMQP RPC, NATS RPC; Kafka is
  an outbound event stream (section 12).

## 1. Context

### 1.1 What exists

- The API (`apps/api`) serves 35 REST operations (audit 3.1) through an oapi-codegen strict server
  (`internal/adapters/httpapi/gen`, 5,620 generated lines) on `net/http.ServeMux`. The layout is
  ports and adapters: one flat domain package `internal/domain/ledger`, one use-case package
  `internal/app` with eight use cases and `ports.go`, three adapters (`httpapi`, `postgres`,
  `tigerbeetle`) and `internal/platform` (config, logging, HTTP server). `cmd/api/wire.go` wires
  everything by hand.
- The spec `packages/openapi/openapi.yaml` (OpenAPI 3.1.0) is hand-written and is the contract
  (ADR-0008). openapi-typescript writes `packages/api-client/src/schema.d.ts`; a hand-written ky
  client wraps it. The console calls 33 of the 35 operations (audit 5.1).
- pnpm workspace and Turborepo run every task. The Turborepo shim `apps/api/package.json` was
  deleted (67af5db) but is still referenced in four places (audit 1.2).
- No justfile, no GraphQL, no gRPC, no broker, no outbox (ADR-0002 promised one; none was built).

### 1.2 What the owner asked for (summary of the binding requirements A to L)

- Bounded-context modules with four layers, commands and queries as separate use cases, a health
  module with only an interfaces layer, bootstrap kept apart from use cases (go-boilerplate).
- One controller folder per transport, versioned by folder; REST, GraphQL, gRPC, AMQP RPC, NATS RPC
  and Kafka; every transport calls the same use cases; one place starts and stops every server
  (evrone/go-clean-template).
- Project rules win over both references: `net/http`, `log/slog`, `testing` and go-cmp, sqlc and
  pgx, goose, hand-written fakes; Huma v2 on `humago` is the single accepted framework exception.
- The REST spec and the GraphQL schema are generated from Go code and committed; REST behaviour
  does not change for third parties; the console moves to GraphQL.
- A justfile becomes the single entry point; Turborepo keeps caching TypeScript.

### 1.3 Invariants that hold in every phase

- TigerBeetle is the only source of truth for money. Postgres never stores a balance.
- One API transaction is one linked chain; `user_data_128` carries the transaction ID; `code`
  carries the event type; IDs come from the client's `ID()` helper.
- Idempotency (ADR-0007) behaves the same on every transport, so it lives in the application layer
  (it already does: `internal/app/writes.go:117-223`).
- Amounts and u128 IDs are strings on every wire. No transport shows TigerBeetle or Postgres error
  text.
- `/healthz` is liveness only; `/readyz` probes every dependency the process uses.

### 1.4 Facts re-checked in the code that the design depends on

| #   | Fact                                                                                                                                                                                                                                   | Where                                                                                                                      |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| V1  | A write issues two TigerBeetle requests, `create_accounts` then `create_transfers`, under one idempotency record and one reconciler path. They are not one TigerBeetle request (see conflict K23).                                     | `internal/app/writes.go:398` (`tb.CreateAccounts`), `writes.go:161-175` (`submit`)                                         |
| V2  | The use-case layer stores HTTP status numbers (201, 200, 400, 409, 422, 500, 503) on the idempotency record.                                                                                                                           | `internal/app/writes.go:24-32`, `writes.go:61-74`                                                                          |
| V3  | The ledger summary query reads tables that the money side owns (`ledger_assets`, `addresses`).                                                                                                                                         | `apps/api/queries/ledgers.sql:11-30`                                                                                       |
| V4  | `EventTypeUsage` unions `transactions` and `holds`; `MatchRule` reads addresses.                                                                                                                                                       | `queries/settings.sql:41-48`; `internal/app/settings.go:95-122`                                                            |
| V5  | An empty Idempotency-Key reaching the domain already yields `missing-idempotency-key`, the same problem REST answers today for a missing header.                                                                                       | `internal/domain/ledger/write.go:12-15`; `internal/adapters/httpapi/router.go:70-78`                                       |
| V6  | Today there is no validation at the HTTP edge: page `limit` above 100 is clamped to 100, history `limit` above 200 is reset to 50, and the body is decoded whatever the `Content-Type`.                                                | `internal/app/cursor.go:20-30`; `internal/app/accounts.go:249-251`; `httpapi/gen/api.gen.go:5497` (`runtime.BindJSONBody`) |
| V7  | The `Dependencies` schema lists `postgres` and `tigerbeetle` as required and does not forbid other members, so adding optional broker members is additive.                                                                             | `packages/openapi/openapi.yaml:1011-1018`                                                                                  |
| V8  | The project already reads "no globals" as "no mutable package state": lookup maps such as `problemTitles` and `kindNames` exist. Huma's settings are mutable package variables, a different thing.                                     | `httpapi/problem.go:31`; `internal/app/writes.go:44`                                                                       |
| V9  | `scripts/check-versions.sh` requires exactly one TigerBeetle image tag in compose, so a CDC job that reuses the server image is checked for free.                                                                                      | `scripts/check-versions.sh:9-15`                                                                                           |
| V10 | The API image is built with context `apps/api`, so any generated Go code must live inside `apps/api`; schema and proto sources outside it are read only at generate time.                                                              | `deploy/compose/compose.yaml:75-76`, `apps/api/Dockerfile:12-15`                                                           |
| V11 | The depguard globs (`**/internal/domain/**`, `**/internal/app/**`, `**/internal/adapters/...`) match no file under `internal/modules`, and every rule is `list-mode: lax`, so the current config would enforce nothing after the move. | `apps/api/.golangci.yml:28-84`                                                                                             |
| V12 | `golang.org/x/sync` v0.24.0 is already in the module graph (indirect), so `errgroup` adds no new module.                                                                                                                               | `apps/api/go.mod:87`                                                                                                       |

## 2. Decisions at a glance

| Topic              | Decision                                                                                                                                                                                                                                                       | Section |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| Modules            | `ledgers` (namespaces and their configuration: metadata, account rules, event types), `bookkeeping` (assets, accounts, transactions, holds, write requests, reconciler, stats, search), `health` (interfaces only). Shared kernel at `internal/kernel/ledger`. | 4       |
| Cross-module calls | Interfaces declared by the consuming module's application layer, satisfied by the providing module's query use cases, wired in `internal/bootstrap`. Modules never import each other.                                                                          | 4.5     |
| Postgres           | One schema, one goose sequence, cross-module foreign keys kept; one sqlc block per module.                                                                                                                                                                     | 4.7     |
| REST               | Huma v2.39.1 on `humago`; spec written by `cmd/openapi`; validation stays in the domain for good (Huma documents constraints, never enforces them); problem model customised.                                                                                  | 7       |
| REST client        | orval 8.44.0, one folder per tag, thin hand-written layer; not hey-api.                                                                                                                                                                                        | 15      |
| GraphQL            | In-repo generator `internal/platform/graphqlschema` writes `packages/graphql/schema.graphql`; gqlgen v0.17.98 generates the executor; module resolvers are embedded into one root.                                                                             | 8       |
| gRPC               | Protos in `packages/proto`, one proto package, buf v1.75.0 with local plugins; services per module.                                                                                                                                                            | 11.1    |
| AMQP and NATS RPC  | One shared request queue (competing consumers) and NATS `micro` with a queue group; bounded concurrency; one typed envelope; controllers shared by both brokers.                                                                                               | 11.2    |
| Kafka              | Outbound ledger event stream fed by TigerBeetle CDC through RabbitMQ and a relay; no Postgres outbox for ledger events; no inbound Kafka commands for now.                                                                                                     | 12      |
| Errors             | One transport-neutral mapping (`internal/platform/problem`), rendered per transport; gRPC uses the standard `ErrorInfo`, `RetryInfo` and `BadRequest` details.                                                                                                 | 9       |
| Idempotency        | Header on REST and on broker RPC; input field on GraphQL and gRPC; same application flow.                                                                                                                                                                      | 10      |
| Task runner        | Root justfile; Go recipes call `go` directly, TypeScript recipes call `pnpm turbo`.                                                                                                                                                                            | 16      |
| Console            | GraphQL through graphql-codegen typed documents and a small fetch client; the system slice and Copy as curl stay REST.                                                                                                                                         | 17      |
| Readiness          | `/readyz` answers 503 only when Postgres or TigerBeetle is down; enabled brokers are reported, and a broker that is down makes the status `degraded` with a 200.                                                                                               | 13.2    |
| Exposure           | gRPC, AMQP RPC, NATS RPC and Kafka are off by default and development-only until authentication is decided; GraphQL has the same status as REST.                                                                                                               | 13.1    |
| ADR statuses       | A new ADR-0015 lets an accepted ADR carry `Accepted, amended by ADR-NNNN`; every ADR this work amends gets that status.                                                                                                                                        | 18      |

## 3. Target directory tree

Representative files only. `(gen)` marks generated, committed, never edited files. Phase numbers
say when a folder appears.

```
apps/api/
  cmd/
    api/main.go                    flags, config, logger, then bootstrap.Build and App.Run
    api/probe.go                   container healthcheck (unchanged)
    api/api_integration_test.go    end-to-end test, now built on bootstrap.Build (phase 3)
    openapi/main.go                registers every REST operation, writes the spec (phase 2)
    graphqlschema/main.go          reads the GraphQL registrations, writes the schema (phase 4)
  gqlgen.yml                       (phase 4)
  sqlc.yaml                        one `sql` block per module (phase 3)
  migrations/                      one goose sequence, embedded; each file names its owning module
  testdata/depguard/               deliberate violations, linted only by `just lint-depguard-probe`
  internal/
    bootstrap/                     wiring and lifecycle, no business rule
      build.go                     Build(ctx, cfg, logger, opts...) (*App, error)
      modules.go                   use cases and cross-module ports, in dependency order
      servers.go                   enabled servers and workers from config
      run.go                       App.Run: errgroup, graceful drain, Close in reverse order
      options.go                   test-only options (TigerBeetle fault injection)
    kernel/ledger/                 shared kernel (was internal/domain/ledger)
      id.go amount.go address.go asset.go metadata.go rule.go event.go errors.go clock.go
    modules/
      health/
        interfaces/rest/v1/        /healthz, /readyz, /v1/_info
        interfaces/graphql/        Query.serviceInfo (phase 4)
        interfaces/grpc/v1/        grpc.health.v1 and Info (phase 5)
      ledgers/
        domain/                    ports.go (LedgerRepository, RuleRepository, EventTypeRepository), records.go
        application/
          ports.go                 what ledgers needs from bookkeeping (Footprints, EventTypeUsage, AddressAssets)
          usecases.go              exported interfaces: LedgerCommands, LedgerQueries, SettingsCommands, SettingsQueries
          ledger_commands.go ledger_queries.go rule_commands.go rule_queries.go
          event_type_commands.go event_type_queries.go policy_queries.go directory.go
          *_test.go                hand-written fakes of the domain ports and of ports.go
        infrastructure/postgres/
          store.go ledgers.go settings.go
          queries/ledgers.sql queries/settings.sql
          gen/                     (gen) sqlc
        interfaces/
          dto/                     wire types shared by REST, GraphQL and RPC: Ledger, Rule, EventType
          rest/v1/                 register.go, ledgers.go, settings.go, *_test.go
          graphql/                 schema.go (registrations), query.go, mutation.go, types.go (phase 4)
          grpc/v1/                 ledger_service.go, settings_service.go (phase 5)
          rpc/v1/                  routes.go, shared by AMQP and NATS RPC (phase 6)
      bookkeeping/
        domain/                    posting.go chain.go (verdicts) write.go hold.go outcome.go
                                   ports.go (LedgerStore, AccountRepository, WriteRepository,
                                   TransactionRepository, HoldRepository, IDGenerator) records.go
        application/
          ports.go                 what bookkeeping needs from ledgers (LedgerDirectory, AccountPolicies, EventTypes)
          usecases.go              TransactionCommands, TransactionQueries, HoldCommands, HoldQueries,
                                   AccountCommands, AccountQueries, RequestQueries, OverviewQueries
          writeflow.go             the ADR-0007 protocol: begin, submit, pending, decline, commit
          ensure_accounts.go       ADR-0005 and ADR-0006 lazy creation
          transaction_commands.go transaction_queries.go hold_commands.go hold_queries.go
          account_commands.go account_queries.go request_queries.go overview_queries.go
          footprint_queries.go     answers the ledgers module's ports
          reconciler.go            SettlePending command and the Run loop
          relay_ledger_event.go    TigerBeetle CDC event to Kafka (phase 7)
        infrastructure/
          postgres/                store.go accounts.go writes.go transactions.go holds.go queries/ gen/ (gen)
          tigerbeetle/             ledger_store.go id.go results.go
          kafka/                   event_publisher.go (phase 7)
        interfaces/
          dto/                     Account, AssetBalance, Transaction, PostingRecord, Hold, WriteRequest, LedgerStats, ...
          rest/v1/                 register.go accounts.go transactions.go holds.go requests.go overview.go reconciler.go
          graphql/                 schema.go query.go mutation.go ledger_fields.go connections.go inputs.go
          grpc/v1/                 account_service.go transaction_service.go hold_service.go request_service.go
          rpc/v1/                  routes.go
          cdc/                     consumer of TigerBeetle CDC messages (phase 7)
    platform/
      config/ logging/ httpserver/ existing
      postgres/                    pgxpool, Ready, Migrate (goose), server and schema versions, InTx
      tigerbeetle/                 the cgo client: New, Ready, ResolveAddresses, bounded calls
      readiness/                   named probes run concurrently, one timeout each
      requestid/                   request ID in context: generate, validate, read
      problem/                     kernel error -> kind, reason, title, detail, retry (transport neutral)
      scalar/                      wire scalars: Amount, SignedAmount, ID, Timestamp, Metadata, LedgerName, ...
      restapi/                     Huma API builder, Problem model, Register helper, middleware
      graphqlschema/               the SDL generator and its golden tests (phase 4)
      grpcapi/platformv1/          (gen) buf output (phase 5)
      grpcserver/                  grpc.Server with Run(ctx) and interceptors (phase 5)
      amqp/                        connection supervisor: reconnect and topology (phase 6)
      rpc/                         envelope, method router, bounded worker pool (phase 6)
      amqprpc/ natsrpc/            broker RPC servers and test clients (phase 6)
      kafka/                       franz-go producer and ping (phase 7)
    transport/                     composition of every module for one transport; imports modules
      rest/register.go             RegisterAll(api, Handlers): the one list of REST operations
      graphql/schema/register.go   RegisterAll(*graphqlschema.Schema); never imports gqlgen
      graphql/exec/                (gen) gqlgen executor and models
      graphql/resolver.go          resolver root built by embedding module resolvers
      graphql/ledger_fields.go     fields one module adds to another module's type
      graphql/handler.go           gqlgen handler, error presenter, limits
      grpc/register.go             registers every module's gRPC services
      rpc/register.go              registers every module's RPC routes
    devstack/                      existing; gains the broker image pins (phases 6 and 7)
packages/
  openapi/
    openapi.yaml                   (gen) written by cmd/openapi; linted by Redocly
    redocly.yaml package.json README.md
  graphql/
    schema.graphql                 (gen) written by cmd/graphqlschema (phase 4)
    package.json README.md
  proto/                           (phase 5)
    buf.yaml buf.gen.yaml
    tigerbeetle/platform/v1/
      ledgers.proto settings.proto accounts.proto transactions.proto holds.proto
      requests.proto health.proto paging.proto
  api-client/                      tree in section 15
justfile                           section 16
```

Removed by the end of phase 3: `internal/domain`, `internal/app`, `internal/adapters`
(including `httpapi/gen`), `apps/api/queries`, `apps/api/oapi-codegen.yaml`.

Go note: `internal/` is enforced by the Go toolchain, not by convention: a package under
`apps/api/internal` can only be imported from inside `apps/api`. `testdata` folders are skipped by
`./...` patterns, which is why the depguard probe fixtures can live there without breaking builds.

## 4. Modules

### 4.1 How the boundaries were derived

Three sources, as the owner asked: the domain package (`internal/domain/ledger`), the use cases
and ports (`internal/app`, audit 2.2 and 2.3) and the 35 operations (audit 3.1). The rule that
decides most of it: anything committed together, in one TigerBeetle chain or in one Postgres
transaction, belongs to one module (audit 4.2, rows F1 to F7). Reads that cross concepts (F10 to
F16) are allowed to cross modules, through application interfaces.

### 4.2 The modules

| Module        | Owns (tables)                                                                                               | Operations (REST)                                                                                                                                                                  | Why this boundary                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ------------- | ----------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ledgers`     | `ledgers`, `account_rules`, `event_types`                                                                   | listLedgers, getLedger, createLedger, replaceLedgerMetadata, listRules, putRule, deleteRule, matchRule, listEventTypes, createEventType (10)                                       | A ledger and its configuration have their own life cycle (created and edited by an operator) and are never written in a TigerBeetle chain. The write path only reads them: it checks the ledger exists, resolves the rule that creates an account (the result is frozen on the account row, ADR-0006) and resolves an event type to the `code` copied into each transfer. Rules and event types are kept here rather than in a separate `settings` module because they have the same consumer, the same coupling pattern and the same owner as the ledger itself; a third module would add two cross-module ports and no isolation. |
| `bookkeeping` | `ledger_assets`, `addresses`, `accounts`, `write_requests`, `transactions`, `transaction_postings`, `holds` | the 6 accounts operations, the 5 transactions operations, the 5 holds operations, listWriteRequests, getWriteRequest, getLedgerStats, listAssets, search, getReconcilerStatus (22) | One write allocates ledger numbers (ADR-0005), creates accounts lazily (ADR-0006), inserts the idempotency record, submits one linked chain, then commits the record and the transaction or hold row in one Postgres transaction (F1 to F7). The reconciler re-runs the same code (F9). Stats and search read only these tables (F13, F14). Splitting accounts, transactions, holds or requests apart would turn every write into a cross-module saga.                                                                                                                                                                              |
| `health`      | none                                                                                                        | getLiveness, getReadiness, getInfo (3)                                                                                                                                             | Interfaces layer only, as required. It reads platform probes (Postgres, TigerBeetle, enabled brokers) through `internal/platform/readiness` and build information; it has no use case of its own.                                                                                                                                                                                                                                                                                                                                                                                                                                   |

Operation tags do not move with modules: `getLedgerStats`, `listAssets` and `search` keep the tag
`ledgers` and `getReconcilerStatus` keeps `operations` although bookkeeping registers them, because
tags drive `packages/api-client` and must not change (requirement F).

### 4.3 Splits considered and rejected

- `settings` as its own module: rejected above (same consumer and coupling as ledgers).
- `accounts`, `transactions`, `holds`, `requests` as separate modules: one write spans all of them
  (F1 to F5) and the requirement says anything that must commit together stays together.
- `overview` (stats, search) as a read-model module: every table it reads is bookkeeping's; a
  separate module would only add ports that forward calls.
- `system` for getInfo and getReconcilerStatus: getInfo is part of health; the reconciler status is
  bookkeeping state, registered by bookkeeping under the `operations` tag.

### 4.4 The shared kernel

`internal/domain/ledger` moves to `internal/kernel/ledger` (package name stays `ledger`, so
`ledger.ErrNotFound` keeps its spelling in code and docs). It keeps what two or more modules use:
`ID`, `Amount`, `Address` and `Pattern`, `Asset`, `Metadata`, ledger name rules, `Rule` and
`MatchRule`, `EventType`, the six sentinel errors and `Problem`, plus `Clock` and two small value
types used across modules (`Footprint`, below). Bookkeeping-only types move to
`modules/bookkeeping/domain`: `Posting`, write `Operation` and `RequestStatus`, `HoldStatus` and
`HoldStatusAt`, the chain verdicts (`classify` is pure domain logic), the TigerBeetle result names
and the stored outcome codes (conflict K19).

Why a kernel and not "every module owns its types": `Rule` is configured in ledgers and frozen on
accounts in bookkeeping; `EventType` is registered in ledgers and copied into transfers; IDs and
amounts are everywhere. A kernel named by its content (`ledger`) satisfies the "no `common` or
`utils`" rule. The kernel imports nothing from `internal`, like the domain package today.

### 4.5 Application interfaces between modules

Modules never import each other. A consuming module declares, in its own `application/ports.go`,
the interface it needs; a query use case of the providing module happens to have that method set;
`internal/bootstrap` passes one to the other. The method signatures only use kernel types.

Go note: a Go type satisfies an interface implicitly, by having the methods. No `implements`
keyword and no import of the interface is needed, which is what lets two modules talk without
importing each other. A mismatch is still a compile error, at the line in bootstrap that passes
the value.

```go
// apps/api/internal/modules/bookkeeping/application/ports.go
package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerDirectory is answered by the ledgers module.
type LedgerDirectory interface {
	// Require returns an error matching ledger.ErrNotFound when the ledger does not exist.
	Require(ctx context.Context, name string) error
}

// AccountPolicies resolves the rule that creates each address (ADR-0006). Bookkeeping freezes the
// result on the account row; the ledgers module owns the rules.
type AccountPolicies interface {
	PoliciesFor(ctx context.Context, ledgerName string, addresses []ledger.Address) (map[ledger.Address]ledger.Rule, error)
}

// EventTypes resolves an event type name to its code. An empty name is the built-in default.
type EventTypes interface {
	Resolve(ctx context.Context, ledgerName, name string) (ledger.EventType, error)
}
```

```go
// apps/api/internal/modules/ledgers/application/ports.go
package application

// Footprints is answered by bookkeeping: the assets and address count shown with a ledger, for a
// whole page of ledgers in one round trip (today one SQL query, queries/ledgers.sql:11-30).
type Footprints interface {
	Footprints(ctx context.Context, ledgerNames []string) (map[string]ledger.Footprint, error)
}

// EventTypeUsage counts the transactions and holds that use each event type name.
type EventTypeUsage interface {
	EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error)
}

// AddressAssets lists the assets an address already has accounts in, for matchRule.
type AddressAssets interface {
	AddressAssets(ctx context.Context, ledgerName string, address ledger.Address) ([]string, error)
}
```

| Consumer needs                             | Provider use case              | Replaces today                                                                          |
| ------------------------------------------ | ------------------------------ | --------------------------------------------------------------------------------------- |
| bookkeeping: `LedgerDirectory.Require`     | ledgers `Directory` query      | `requireLedger` (`internal/app/accounts.go:72`)                                         |
| bookkeeping: `AccountPolicies.PoliciesFor` | ledgers `PolicyQueries`        | `rules.ListRules` plus `ledger.MatchRule` inside `ensureAccounts` (`writes.go:360-365`) |
| bookkeeping: `EventTypes.Resolve`          | ledgers `PolicyQueries`        | `resolveEventType` (`settings.go:170`)                                                  |
| ledgers: `Footprints`                      | bookkeeping `FootprintQueries` | the subqueries of `GetLedgerSummary` and `ListLedgers`                                  |
| ledgers: `EventTypeUsage`                  | bookkeeping `FootprintQueries` | `EventTypeUsage` SQL (`settings.sql:41-48`)                                             |
| ledgers: `AddressAssets`                   | bookkeeping `AccountQueries`   | `accounts.GetAddress` inside `MatchRule`                                                |

The modules depend on each other in both directions, but no use case does: the three ledgers
queries bookkeeping needs read only ledgers tables, and the bookkeeping queries ledgers needs read
only bookkeeping tables. Commands and queries being separate use cases is what makes the graph
acyclic, and it fixes the construction order in bootstrap:

```go
// apps/api/internal/bootstrap/modules.go (sketch)
func buildModules(r resources, cfg config.Config, logger *slog.Logger) (modules, error) {
	ledgerStore := ledgerspg.NewStore(r.pool)
	directory := ledgersapp.NewDirectory(ledgerStore)
	policies := ledgersapp.NewPolicyQueries(ledgerStore)

	bk, err := bookkeepingapp.New(bookkeepingapp.Deps{
		Store:       bookkeepingpg.NewStore(r.pool),
		Ledger:      bookkeepingtb.NewLedgerStore(r.tigerbeetle),
		IDs:         bookkeepingtb.IDGenerator{},
		Clock:       ledger.ClockFunc(time.Now),
		Ledgers:     directory,
		Policies:    policies,
		EventTypes:  policies,
		GracePeriod: cfg.ReconcilerGracePeriod,
		Logger:      logger,
	})
	if err != nil {
		return modules{}, fmt.Errorf("bookkeeping: %w", err)
	}

	lg := ledgersapp.New(ledgersapp.Deps{
		Store:      ledgerStore,
		Directory:  directory,
		Policies:   policies,
		Footprints: bk.Footprints,
		Usage:      bk.Footprints,
		Addresses:  bk.Accounts,
		Clock:      ledger.ClockFunc(time.Now),
	})
	return modules{ledgers: lg, bookkeeping: bk}, nil
}
```

Behaviour note: getLedger becomes two queries (ledger row, then footprint) instead of one;
listLedgers stays two round trips per page (one per module), never one per ledger.

### 4.6 Inside a module

- `domain`: entities, value types, rules and the ports that infrastructure implements (repository
  and store interfaces plus the record types they return). Imports only the kernel and the
  standard library. Ports move here from `internal/app/ports.go` because the owner's direction is
  "infrastructure to domain": infrastructure may not import application, so the interfaces and the
  record types it implements must be in domain (conflict K5).
- `application`: one type per command family and one per query family, never mixed, for example
  `TransactionCommands` (Create, Revert, ReplaceMetadata) and `TransactionQueries` (Get, List).
  `usecases.go` exports these as interfaces; every transport depends on the interfaces, so each
  transport's adapter tests use hand-written fakes of them. `ports.go` holds the cross-module
  interfaces. No transport type, no driver, no `net/http`.
- `infrastructure`: Postgres (sqlc and pgx), TigerBeetle (over `internal/platform/tigerbeetle`) and,
  in phase 7, the Kafka publisher. Imports its own domain, the kernel and platform.
- `interfaces`: one folder per transport, versioned (`rest/v1`, `grpc/v1`, `rpc/v1`); `graphql` is
  not versioned because a GraphQL schema evolves by deprecation, not by version (conflict K22);
  `dto` holds wire types shared across transports where shapes match.

Commands and queries are separate types, not separate packages: they share view-building helpers,
and one package per module keeps the import graph simple for a Go newcomer.

### 4.7 Postgres: one schema, owned tables

- One schema and one goose sequence in `apps/api/migrations` (goose keeps one version table; a
  sequence per module would need several). Each migration file gets a header comment naming the
  owning module. The cross-module foreign keys to `ledgers(name)` stay (F17): they protect data, and
  splitting schemas would buy nothing in one process.
- One `sql` block per module in `sqlc.yaml`, each reading the whole schema (`schema: migrations`)
  and only its own query folder, writing to its own `infrastructure/postgres/gen`. A module's
  queries touch only its tables; the queries that cross today (V3, V4) move to bookkeeping and are
  reached through the ports of 4.5.
- `ledger_assets.tb_ledger` stays one global identity (F18), owned by bookkeeping.

## 5. Library, tool and image choices

Versions are the ones research.md verified on 2026-10-25 (its sections (c), (d), (f) and
Appendix A); none was changed. The phase column says when each enters the repository. Pins are
re-checked at the start of each phase and bumped only through a normal dependency commit.

### 5.1 Go modules (`apps/api/go.mod`)

| Module                                                                                      | Pin            | Phase | Source                                                                   | Notes                                                                                                                                        |
| ------------------------------------------------------------------------------------------- | -------------- | ----- | ------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------- |
| github.com/danielgtaylor/huma/v2 (root and `adapters/humago`)                               | v2.39.1        | 2     | https://github.com/danielgtaylor/huma/releases/tag/v2.39.1               | MIT, go 1.25 floor, no cgo. The one web framework exception (ADR-0018).                                                                      |
| github.com/99designs/gqlgen                                                                 | v0.17.98       | 4     | https://github.com/99designs/gqlgen/releases/tag/v0.17.98                | MIT, go 1.26 floor. Runtime library plus `tool` directive for `go tool gqlgen`. gqlparser comes with it through go.mod.                      |
| google.golang.org/grpc                                                                      | v1.84.0        | 5     | https://github.com/grpc/grpc-go/releases/tag/v1.84.0                     | Includes `health` and `reflection`; no extra module.                                                                                         |
| google.golang.org/protobuf                                                                  | v1.36.12       | 5     | https://github.com/protocolbuffers/protobuf-go/releases/tag/v1.36.12     | Not v1.37.0-rc.1 (prerelease).                                                                                                               |
| github.com/rabbitmq/amqp091-go                                                              | v1.16.0        | 6     | https://github.com/rabbitmq/amqp091-go/releases/tag/v1.16.0              | Pure Go. No auto-reconnect: `internal/platform/amqp` supervises the connection.                                                              |
| github.com/nats-io/nats.go                                                                  | v1.55.0        | 6     | https://github.com/nats-io/nats.go/releases/tag/v1.55.0                  | Pure Go; the `micro` package gives queue groups and service errors.                                                                          |
| github.com/twmb/franz-go                                                                    | v1.22.2        | 7     | https://github.com/twmb/franz-go/tags                                    | Pure Go, idempotent producer. `kfake` is not used (no tagged release).                                                                       |
| golang.org/x/sync                                                                           | v0.24.0        | 3     | already in `apps/api/go.mod:87` (indirect)                               | `errgroup` for starting and stopping servers.                                                                                                |
| github.com/testcontainers/testcontainers-go/modules/rabbitmq, `.../modules/nats`            | v0.44.0        | 6     | https://github.com/testcontainers/testcontainers-go/releases/tag/v0.44.0 | Same version as the core module already used. Kafka uses a generic container (research (c): the kafka module only accepts Confluent images). |
| Removed: github.com/oapi-codegen/oapi-codegen/v2 (tool) and github.com/oapi-codegen/runtime | v2.8.1, v1.8.0 | 2     |                                                                          | With `oapi-codegen.yaml` and `internal/adapters/httpapi/gen`.                                                                                |

Unchanged: tigerbeetle-go v0.17.10, pgx v5.11.0, goose v3.29.0, caarlos0/env v11.4.1, go-cmp
v0.7.0, testcontainers-go v0.44.0. Rejected: segmentio/kafka-go (last release April 2026, 271 open
issues and PRs) and confluent-kafka-go (cgo, research (c)).

### 5.2 Tool binaries (`scripts/install-tools.sh`, sha256 per platform from research Appendix A)

| Tool               | Pin      | Phase | Source                                                                                                 |
| ------------------ | -------- | ----- | ------------------------------------------------------------------------------------------------------ |
| just               | 1.59.0   | 1     | https://github.com/casey/just/releases/tag/1.59.0 (checksums cross-checked with `SHA256SUMS`)          |
| oasdiff            | v1.34.1  | 2     | https://github.com/oasdiff/oasdiff/releases/tag/v1.34.1 (`checksums.txt`; darwin archive is universal) |
| buf                | v1.75.0  | 5     | https://github.com/bufbuild/buf/releases/tag/v1.75.0 (`sha256.txt`)                                    |
| protoc-gen-go      | v1.36.12 | 5     | https://github.com/protocolbuffers/protobuf-go/releases/tag/v1.36.12 (must equal the protobuf runtime) |
| protoc-gen-go-grpc | v1.6.2   | 5     | https://github.com/grpc/grpc-go/releases/tag/cmd/protoc-gen-go-grpc/v1.6.2                             |

Unchanged: golangci-lint v2.15.0, sqlc v1.31.1, govulncheck v1.9.0. Each new pin gets a
`# renovate:` comment like the existing ones and a `customManagers` entry in `renovate.json`.
buf runs the two protoc plugins from `$PATH` (`local:` plugins), so nothing is fetched from the
Buf Schema Registry at generate time.

### 5.3 npm packages

| Package                                                                   | Pin                  | Phase | Where                                                                                                                    | Source                                                               |
| ------------------------------------------------------------------------- | -------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------- |
| orval                                                                     | 8.44.0               | 2     | `packages/api-client` devDependency                                                                                      | https://www.npmjs.com/package/orval/v/8.44.0                         |
| @graphql-codegen/cli                                                      | 7.4.6                | 4     | `apps/console` devDependency                                                                                             | https://www.npmjs.com/package/@graphql-codegen/cli/v/7.4.6           |
| @graphql-codegen/client-preset                                            | 6.2.3                | 4     | `apps/console` devDependency                                                                                             | https://www.npmjs.com/package/@graphql-codegen/client-preset/v/6.2.3 |
| graphql                                                                   | 17.0.2               | 4     | `apps/console` devDependency (the codegen reads the schema with it; `documentMode: 'string'` keeps it out of the bundle) | https://www.npmjs.com/package/graphql/v/17.0.2                       |
| Removed: openapi-typescript, typescript 5.9.3 (in `packages/openapi`), ky | 7.13.0, 5.9.3, 2.2.0 | 2     |                                                                                                                          |                                                                      |

Unchanged: TypeScript 7.0.2, @redocly/cli 2.71.0, turbo 2.11.10, vite-plus 1.4.0, Node 24.22.1,
pnpm 12.15.0.

pnpm age rule (lead point 11): pnpm's default `minimumReleaseAge` is 1440 minutes and the repo
sets no override (`pnpm-workspace.yaml` only lists `minimumReleaseAgeExclude` for
`@redocly/cli@2.71.0`). Every npm pin is chosen as the newest version published more than 24
hours before its commit. orval 8.45.0 was published 2026-10-25 12:33 UTC and is blocked until
2026-10-26 12:33 UTC, hence 8.44.0 today. No new entry goes into `minimumReleaseAgeExclude`
without a written reason in the commit.

### 5.4 Container images (`deploy/compose/compose.yaml`, profile `brokers`)

| Image               | Pin                                                                                                 | Phase | Source                                                                                                                  |
| ------------------- | --------------------------------------------------------------------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------- |
| RabbitMQ            | `rabbitmq:4.4.0-management@sha256:c200fd496482cda18c412be4e6f244830e1e75949b8cf5f0e887cea9ea456109` | 6     | https://hub.docker.com/v2/repositories/library/rabbitmq/tags/4.4.0-management (fallback 4.3.7-management, research (d)) |
| NATS                | `nats:2.15.1-alpine@sha256:7ae34c25dfd6fe2e01c6483eabef20d796690fdcac6c4d94ca780a6cf9877fde`        | 6     | https://hub.docker.com/v2/repositories/library/nats/tags/2.15.1-alpine (no `-js`: JetStream is not used)                |
| Kafka               | `apache/kafka:4.3.2@sha256:852a0d49fc4e656b022e7f23785411802c9a088c78a7c5e39f33969de9c227a7`        | 7     | https://hub.docker.com/v2/repositories/apache/kafka/tags/4.3.2 (Apache-2.0; fallback Redpanda v26.2.6 for tests only)   |
| TigerBeetle CDC job | `ghcr.io/tigerbeetle/tigerbeetle:0.17.9` (same image as the server, `amqp` subcommand)              | 7     | https://github.com/tigerbeetle/tigerbeetle/releases/tag/0.17.9                                                          |

`internal/devstack` learns the three new image patterns so testcontainers use exactly the compose
pins, as it does for TigerBeetle and Postgres today (`internal/devstack/images.go:18-21`).

## 6. Where generated code lives

| Generator                               | Reads                                                                                                   | Writes                                                                               | Per module?                                                                                              | Lint                               |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| sqlc                                    | `migrations/` (whole schema) and `modules/<m>/infrastructure/postgres/queries/*.sql`                    | `modules/<m>/infrastructure/postgres/gen/`                                           | Yes, one `sql` block per module                                                                          | Excluded (`Code generated` header) |
| Huma registrations                      | Hand-written Go in `modules/<m>/interfaces/rest/v1/register.go` and `health/interfaces/rest/v1`         | Nothing at build time; `cmd/openapi` renders them to `packages/openapi/openapi.yaml` | Registrations per module, one spec                                                                       | Spec linted by Redocly             |
| cmd/graphqlschema                       | `modules/<m>/interfaces/graphql/schema.go` through `transport/graphql/schema`                           | `packages/graphql/schema.graphql`                                                    | Registrations per module, one schema                                                                     | n/a                                |
| gqlgen                                  | `packages/graphql/schema.graphql`, autobind packages                                                    | `internal/transport/graphql/exec/` (executor and any leftover models)                | No: gqlgen writes one executor package and one resolver root. Resolvers stay per module (8.9).           | Excluded                           |
| buf (protoc-gen-go, protoc-gen-go-grpc) | `packages/proto/tigerbeetle/platform/v1/*.proto`                                                        | `internal/platform/grpcapi/platformv1/`                                              | No: one proto package gives one Go package. Services are implemented per module in `interfaces/grpc/v1`. | Excluded                           |
| orval                                   | `packages/openapi/openapi.yaml`                                                                         | `packages/api-client/src/gen/<tag>/` and `src/gen/model/`                            | Per OpenAPI tag                                                                                          | Ignored by Oxfmt and Oxlint        |
| graphql-codegen (client preset)         | `packages/graphql/schema.graphql` and the `graphql()` documents in `apps/console/src/features/*/api.ts` | `apps/console/src/graphql/gen/`                                                      | n/a                                                                                                      | Ignored                            |
| TanStack Router                         | `apps/console/src/routes`                                                                               | `apps/console/src/route-tree.gen.ts`                                                 | n/a                                                                                                      | Ignored (unchanged)                |

Why the protobuf output sits in `platform` and the gqlgen output in `transport`: module gRPC
controllers import the protobuf types, and modules may import platform but not transport. The
gqlgen executor imports module types (it binds to them), so it must live in a package that is
allowed to import modules: `transport`.

## 7. REST spec generated from code

### 7.1 Huma setup in `internal/platform/restapi`

Huma reads three of its settings from package-level variables: `huma.NewError`,
`huma.NewErrorWithContext` (the error model) and `huma.DefaultArrayNullable` (defaults to true,
which would make every slice `type: [array, null]` in the spec). AGENTS.md forbids globals and
`init()` (`AGENTS.md:18`). Resolution (lead point 5): the assignments happen in exactly one
function, guarded by a `sync.Once`, called by the one constructor every caller must use
(`restapi.NewAPI`). No `init()`. ADR-0018 records the exception; the `sync.Once` is the only
package-level variable of the REST code. The same pattern is used once more, in phase 5, for
grpc-go's global logger (conflict K6), and nowhere else.

Alternative considered: leave `DefaultArrayNullable` alone and clear `nullable` on array schemas
after registration (a walk over the document, like the `additionalProperties` clean-up of 7.4).
It removes one assignment but not the need for `NewError`, so the `sync.Once` stays either way.

Go note: `sync.Once` runs a function at most once even when called from many goroutines at the same
time. Tests build several APIs in parallel; without it the race detector (`-race`) would report
concurrent writes to Huma's variables.

```go
// Package restapi builds the Huma API shared by the server and cmd/openapi. It opens no
// connection and registers no operation itself.
package restapi

// hooks guards Huma's package-level settings (ADR-0018). It is the only package-level variable of
// the REST code; the values it sets never change after the first call.
var hooks sync.Once

func configureHuma() {
	hooks.Do(func() {
		huma.DefaultArrayNullable = false // slices are arrays, never null, as in the committed spec
		huma.NewError = newHumaError
		huma.NewErrorWithContext = newHumaErrorWithContext
	})
}

// NewAPI returns the Huma API on mux with the settings the committed spec depends on.
func NewAPI(mux *http.ServeMux, version string) huma.API {
	configureHuma()
	cfg := huma.DefaultConfig("TigerBeetle Platform API", version)
	cfg.OpenAPIPath = "" // no spec route: the committed file is the published spec (7.7)
	cfg.DocsPath = ""    // no docs page: it would load JavaScript from a CDN
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil // no $schema member in bodies, no Link header
	cfg.Info.Description = specDescription
	cfg.Info.License = &huma.License{Name: "Apache-2.0", Identifier: "Apache-2.0"}
	cfg.Servers = []*huma.Server{{URL: "http://localhost:8080", Description: "Local compose stack"}}
	cfg.Tags = specTags()
	api := humago.New(mux, cfg)
	registerComponents(api.OpenAPI()) // Problem response, named scalars, allOf schemas (7.4)
	return api
}

// Register adds one operation. Errors from the handler go through platform/problem, so no
// TigerBeetle or Postgres text reaches a client, and every operation documents them with the
// shared default problem response.
func Register[I, O any](api huma.API, op huma.Operation, h func(context.Context, *I) (*O, error)) {
	op.Responses = withDefaultProblem(op.Responses)
	// Validation stays in the domain, as today, so every input error is still a 400 with the
	// domain's detail text (7.3). It stays off for good: five transports share the domain's
	// validation, and Huma's would apply to REST only (ADR-0018).
	op.SkipValidateParams = true
	op.SkipValidateBody = true
	huma.Register(api, op, func(ctx context.Context, in *I) (*O, error) {
		out, err := h(ctx, in)
		if err != nil {
			return nil, toProblem(ctx, err)
		}
		return out, nil
	})
}
```

A module registers its operations with typed input and output structs; the tags carry
descriptions (`doc`), examples (`example`), formats and validation (`pattern`, `minLength`,
`maxItems`), which become the spec:

```go
// apps/api/internal/modules/bookkeeping/interfaces/rest/v1/transactions.go (sketch)
type createTransactionInput struct {
	Ledger         scalar.LedgerName `path:"ledger"`
	IdempotencyKey string            `header:"Idempotency-Key" required:"true" minLength:"1" maxLength:"255" doc:"Makes the write safe to retry (ADR-0007)."`
	Body           dto.CreateTransactionRequest
}

type transactionOutput struct {
	Body dto.Transaction
}

func registerTransactions(api huma.API, commands application.TransactionCommands) {
	restapi.Register(api, huma.Operation{
		OperationID:   "createTransaction",
		Method:        http.MethodPost,
		Path:          "/v1/ledgers/{ledger}/transactions",
		Summary:       "Create a transaction",
		Tags:          []string{"transactions"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createTransactionInput) (*transactionOutput, error) {
		view, err := commands.Create(ctx, toCreateCommand(ctx, in))
		if err != nil {
			return nil, fmt.Errorf("create transaction: %w", err)
		}
		return &transactionOutput{Body: dto.TransactionFrom(view)}, nil
	})
}
```

The stdlib middleware stays around the `ServeMux` exactly as today (`internal/adapters/httpapi/router.go:61-64`):
request ID, access log, panic recovery and the 404 and 405 problem conversion. humago registers
`METHOD path` patterns on the same mux, so Huma does no routing of its own.

### 7.2 `cmd/openapi`

`cmd/openapi` builds `restapi.NewAPI(http.NewServeMux(), version)`, calls
`transport/rest.RegisterAll(api, rest.Handlers{})` with zero-value (nil) use cases, renders
`api.OpenAPI()` to YAML and writes `packages/openapi/openapi.yaml`. Registration never calls a use
case, so nil is safe; the command opens no socket and touches neither TigerBeetle nor Postgres.

Two guarantees that it stays that way:

- It must not link the cgo TigerBeetle client (audit X16). depguard denies `tigerbeetle-go`,
  `internal/platform/tigerbeetle`, pgx and every `infrastructure` package in `cmd/openapi` and
  `cmd/graphqlschema` (section 14), and `just generate` builds both with `CGO_ENABLED=0`, which fails
  loudly if a cgo import sneaks in.
- The file starts with `# Generated by cmd/openapi from the Go registrations. Do not edit.`, and
  the spec's `info.description` says the same, so nobody edits it by hand; the drift check catches
  anyone who does.
- Output is byte-stable: the command runs twice in its unit test and compares the bytes. The first
  generated file reorders the committed YAML (paths sorted, components sorted), so phase 2 has one
  large, purely mechanical diff; oasdiff (7.5) is what proves it is the same API.

### 7.3 Keeping 400 for input errors (lead point 4)

Huma answers 422 when schema validation fails, 400 for an unparsable body and 415 for an unknown
content type. Today every input error is `400 /problems/invalid-request` (or
`/problems/missing-idempotency-key`), and 422 means a decline or a reused key (audit 2.5).

1. Phase 2 registers every operation with `SkipValidateParams` and `SkipValidateBody` (7.1). The
   domain keeps validating, exactly as today, so status, type and `detail` text do not change.
   This also preserves V6: `limit` above 100 is still clamped, history `limit` above 200 still
   falls back to 50.
2. A missing `Idempotency-Key` reaches the use case as an empty string; the domain already answers
   `missing-idempotency-key` for it (V5). Same type, same status as today.
3. Errors Huma still raises itself (unparsable JSON, missing required body) go through
   `newHumaErrorWithContext`, which maps 422 and 400 to `400 /problems/invalid-request` with a
   `detail` built from Huma's messages, and to `missing-idempotency-key` when the operation
   declares the header and the request lacks it. The Problem body has no `errors` array, as in the
   committed schema.
4. Huma's edge validation stays off, now and later (Q4). Five transports call the same use
   cases; validation at the REST edge would make REST answer differently from GraphQL, gRPC and the
   brokers. The tags still document constraints in the spec without enforcing them: `minimum: 1`
   and `maximum: 100` on the page limit, `maximum: 200` on history, `maxItems: 64` on postings,
   exactly as the committed spec documents them today. If rejection ever replaces clamping, it is a
   domain change that every transport gets at once.

Both Huma fields and the `NewErrorWithContext` hook are documented for v2.39.1 in research (c); the
Huma spike (Appendix, S1) confirms them before phase 2 code is written.

### 7.4 Spec features Huma may not reproduce (lead point 6, audit 3.3)

| Feature in today's spec                                                                                                                                                                                                            | How it is kept                                                                                                                                                                                                                                           | If the spike shows it cannot be                                                                                                          |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| 14 named scalar components (`LedgerName`, `Address`, `AddressPrefix`, `Asset`, `Amount`, `SignedAmount`, `Id`, `TigerBeetleTimestamp`, `Metadata`, `BalanceLimit`, `HoldStatus`, `Operation`, `RequestStatus`, `DependencyStatus`) | Each is a named Go type in `internal/platform/scalar` implementing `huma.SchemaProvider`: it registers its schema once under the component name and returns a `$ref` to it. The TypeScript names the console imports stay.                               | Accept inlined schemas; oasdiff reports no semantic change; the hand-written client layer re-exports the 14 names as aliases.            |
| `propertyNames` pattern on `Metadata`                                                                                                                                                                                              | `scalar.Metadata` provides its schema by hand with `propertyNames` written through `huma.Schema.Extensions` (Huma has no exported field for it). The domain still validates keys (`internal/domain/ledger/metadata.go:16-24`).                           | Accept the removed keyword (a relaxed constraint; oasdiff reports it); the domain check is unchanged.                                    |
| `allOf` on `Account`, `PostingRecord`, `WriteRequestDetail`                                                                                                                                                                        | The three Go types embed their base struct (so JSON is flat, as today) and implement `SchemaProvider` returning `allOf: [$ref base, {extra properties}]` with Huma's exported `AllOf` field.                                                             | Accept flattened objects: same members, TypeScript intersection becomes one object type; listed in the oasdiff report.                   |
| 503 body on `/readyz`                                                                                                                                                                                                              | The operation declares `200` and `503` responses explicitly, both `Readiness`; the output struct has a `Status int` field that the handler sets to 503. The default problem response is added by `Register`.                                             | None expected; Huma documents dynamic status fields.                                                                                     |
| `security: []`, `info.summary`, `license.identifier`, tag descriptions, `servers`                                                                                                                                                  | Config fields where Huma has them; `security: []` and `info.summary` through `Extensions` if no field exists. Redocly's `security-defined` rule needs `security: []`.                                                                                    | `cmd/openapi` post-processes the YAML document for these two keys only.                                                                  |
| `components.responses.Problem` and `default` on every operation                                                                                                                                                                    | `Register` sets `default: $ref '#/components/responses/Problem'`; `registerComponents` adds the response and the `Problem` schema (hand-written `SchemaProvider`: `uri-reference`, `default: about:blank`, `minimum: 100`, `maximum: 599`, no `errors`). | None.                                                                                                                                    |
| Plain `integer` without `format` on 5 fields (`code`, `attempts`, `segments`, `chunks`, `eventType.code`)                                                                                                                          | Go `int` and `uint16` may emit `format: int64`.                                                                                                                                                                                                          | Accept `format` additions (TypeScript stays `number`); listed.                                                                           |
| `additionalProperties` unset on objects                                                                                                                                                                                            | If Huma sets `additionalProperties: false` on struct schemas, `registerComponents` clears it after all registrations so the spec matches and unknown members stay ignored at run time, as today.                                                         | `cmd/openapi` clears it in the YAML document. Never accepted: with validation off it would document a rejection that never happens (Q4). |
| Anonymous nested objects (`DeclinedStats.reasons` items, `SearchResults` items)                                                                                                                                                    | Named Go types; if Huma registers them as components, the shape is unchanged.                                                                                                                                                                            | Accept new component names (oasdiff resolves `$ref`s).                                                                                   |
| Optional bodies (`revertTransaction`, `captureHold`), no body (`voidHold`), required body with optional fields (`createLedger`)                                                                                                    | Pointer `Body` for optional, no `Body` field for void, value `Body` for createLedger. A test pins today's answer to an empty createLedger body (400).                                                                                                    | None expected.                                                                                                                           |
| Shared parameter components (`LedgerName`, `Cursor`, `Limit`, ...)                                                                                                                                                                 | Huma inlines parameters per operation; the six components disappear.                                                                                                                                                                                     | Accepted: semantically equal after `$ref` resolution; Redocly `no-unused-components` stays green.                                        |
| `examples` arrays on 16 schemas or properties                                                                                                                                                                                      | `example` tags.                                                                                                                                                                                                                                          | Accept format differences (documentation only).                                                                                          |
| `const: ok` on `Liveness.status`                                                                                                                                                                                                   | Hand-written schema on the type if the tag is not enough.                                                                                                                                                                                                | Accept `enum: [ok]`.                                                                                                                     |
| `$schema` member and `Link` header                                                                                                                                                                                                 | Removed by clearing `CreateHooks`.                                                                                                                                                                                                                       | None.                                                                                                                                    |

### 7.5 oasdiff procedure against today's hand-written spec

Run once at the switch commit of phase 2, recorded in the commit body and in
`docs/plans/ddd-transports/oasdiff-phase2.md`, then kept as a CI gate.

```sh
# 1. The hand-written spec, before the switch.
git show HEAD:packages/openapi/openapi.yaml > "$TMPDIR/base.yaml"
# 2. The generated spec.
just generate-contracts            # cd apps/api && CGO_ENABLED=0 go run ./cmd/openapi -out ...
# 3. Breaking changes only. Must print nothing, or only items listed in the table below.
oasdiff breaking "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --fail-on WARN
# 4. Every consumer-visible change, documentation excluded, then included.
oasdiff changelog "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --exclude-elements description,examples,title,summary
oasdiff changelog "$TMPDIR/base.yaml" packages/openapi/openapi.yaml
# 5. Full structural diff, kept as the evidence file.
oasdiff diff "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --format yaml > "$TMPDIR/oasdiff.yaml"
# 6. The spec still lints.
pnpm --filter @tigerbeetle-platform/openapi lint
```

Beyond oasdiff: the end-to-end test (`cmd/api/api_integration_test.go`) runs unchanged against
the Huma server; a new table test replays a fixed list of requests (bad cursor, missing key, empty
createLedger body, unknown route, 405, declined chain, in-flight replay) and compares status,
`Content-Type`, `Retry-After` and the problem body with go-cmp; the generated TypeScript of the
client is type-checked against the console.

Expected differences and how each is resolved:

| #   | Difference                                                                                                                  | Resolution                                                                                       |
| --- | --------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| D1  | Whole-file reordering                                                                                                       | Not a change; oasdiff ignores order.                                                             |
| D2  | Parameter components inlined                                                                                                | Accepted (7.4).                                                                                  |
| D3  | Possible `format: int64` on 5 integers                                                                                      | Accepted (7.4).                                                                                  |
| D4  | Possible new component names for anonymous objects                                                                          | Accepted (7.4).                                                                                  |
| D5  | `allOf`, `propertyNames`, `const` if the spike fails                                                                        | Accepted with the fallback of 7.4.                                                               |
| D6  | `info.description` text ("Edit the spec first, then run `pnpm generate`" becomes "generated from the Go code; do not edit") | Intended.                                                                                        |
| D7  | Description and example wording                                                                                             | Fixed until the documentation-excluding changelog is empty; wording differences left are listed. |
| D8  | Anything else                                                                                                               | A bug in the registrations: fixed before the switch commit.                                      |

One additive change is deliberately not part of the parity commit and comes as a separate commit
right after it (approved, Q3): declaring `X-Request-Id` (request and response header) and
`Retry-After` (response header of the problem response), both behaviours that exist today but are
undocumented (audit X13). It lands before the client is generated, so the generated client gives
typed access to `Retry-After` (K34). Its oasdiff lines are INFO only. The optional broker members of `Dependencies` arrive
in phases 6 and 7 (section 13).

From phase 2 on, CI runs `oasdiff breaking <base branch spec> <generated spec> --fail-on ERR` on
every pull request, so a breaking change must be acknowledged explicitly.

### 7.6 Answer to ADR-0008's rejection of code-first (lead point 1)

ADR-0008 rejected code-first because it "makes the spec a build output that reviewers rarely read"
(`docs/adr/0008-openapi-first-generated-server-and-client.md:39-40`). ADR-0018 answers it:

- The spec stays a committed, reviewed file. `just generate` writes it; CI fails when the committed
  file differs from what the code generates, so every API change shows a YAML hunk in the same pull
  request, exactly as the oapi-codegen output did.
- `oasdiff breaking` runs on every pull request against the base branch (7.5): a reviewer does not
  have to spot a breaking change by reading YAML; CI names it.
- Redocly `recommended-strict` keeps linting the file.
- The drift ADR-0008 feared in the other direction (handlers that disagree with the spec) cannot
  happen: the spec is derived from the very structs the handlers use.
- What is lost: the spec is no longer the place where an API change starts. The new "Adding an
  endpoint" workflow (section 19, phase 2) starts from the input and output structs, and
  CONTRIBUTING.md asks reviewers to read the spec hunk first, as ADR-0008 already did for the
  regenerated files.

### 7.7 Spec and docs page at run time

The running API serves neither the spec nor a docs page (`OpenAPIPath`, `DocsPath` and
`SchemasPath` empty). Reasons: new routes would change the REST surface; Huma's docs page loads
JavaScript from a CDN at run time; the committed file is the published contract. Integrators read
`packages/openapi/openapi.yaml` or render it with Redocly. Decided (Q5): the committed file is
the contract, and the API does not serve it.

### 7.8 Behaviour differences oasdiff cannot see

| Behaviour                                                                                                               | Today                            | With Huma                                                                | Resolution                                                                                                                                                                                                                                                             |
| ----------------------------------------------------------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Validation errors                                                                                                       | Domain, 400                      | 422 unless skipped                                                       | Skipped in phase 2 (7.3).                                                                                                                                                                                                                                              |
| Non-JSON `Content-Type` on a body (for example `curl -d` without `-H`, which sends `application/x-www-form-urlencoded`) | Body decoded as JSON anyway (V6) | Expected 415                                                             | Phase 2 adds a five-line middleware that treats a body on an operation that consumes JSON as JSON, keeping today's behaviour (decided, Q6); the spike confirms Huma's actual answer first. Revisited only with a future strictness change, in a new API minor version. |
| Unknown JSON members                                                                                                    | Ignored                          | Rejected if `additionalProperties: false`                                | Cleared (7.4).                                                                                                                                                                                                                                                         |
| 404 and 405                                                                                                             | problem+json via `unknownRoutes` | Same: the wrapper stays                                                  | Covered by the replay test.                                                                                                                                                                                                                                            |
| `Retry-After`                                                                                                           | Set by the HTTP adapter          | Set through `GetHeaders` on the problem error (Huma's headers interface) | Covered by the replay test.                                                                                                                                                                                                                                            |

## 8. GraphQL schema generated from code

### 8.1 Why a small in-repo generator

gqlgen is schema-first: it needs an SDL file and generates Go interfaces from it. graphql-go
builds the schema in Go but hands resolvers untyped `map[string]any` arguments. The owner wants
both: the schema derived from Go types, and typed resolvers checked by the compiler. A small
generator closes the gap: modules register typed Go structs, `cmd/graphqlschema` turns them into
SDL, gqlgen turns the SDL into typed resolver interfaces, and the modules implement them. If a
registration changes, the schema changes, gqlgen's interfaces change, and the module's resolver no
longer compiles until it is updated.

The generator lives in `internal/platform/graphqlschema`, stays under about 600 lines plus tests,
uses only the standard library (`reflect`, `sort`, `strings`) and validates its output with
gqlparser (already a gqlgen dependency) in tests. ADR-0020 explains it.

### 8.2 Registration API

Go note: Go methods cannot have their own type parameters, so the registration calls are generic
functions that take the schema as their first argument. `reflect.TypeFor[T]()` (Go 1.22+) turns a
type parameter into a `reflect.Type` without needing a value.

```go
// Package graphqlschema renders a GraphQL SDL document from Go types registered by the modules.
// It imports neither gqlgen nor any generated code, so cmd/graphqlschema compiles even when the
// gqlgen output is stale.
package graphqlschema

// Schema collects registrations. It has no package-level state: build one with New.
type Schema struct {
	roots   map[string][]field // "Query" and "Mutation"
	extra   map[string][]field // resolver fields added to an object type, by GraphQL type name
	scalars map[reflect.Type]scalarDef
	docs    map[reflect.Type]string
}

type field struct {
	name, doc    string
	args, result reflect.Type
}

// NoArgs marks a field without arguments.
type NoArgs struct{}

// New returns an empty schema that already maps time.Time to DateTime and int64 to Int64.
func New() *Schema { ... }

// Query registers Query.<name>(args): Result.
func Query[Args, Result any](s *Schema, name, doc string) {
	s.add("Query", name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Mutation registers Mutation.<name>(args): Result.
func Mutation[Args, Result any](s *Schema, name, doc string) {
	s.add("Mutation", name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Field registers a resolver field on an object type named by its GraphQL name. Naming the parent
// by string lets one module extend a type another module owns (Ledger.accounts) without importing
// it; SDL fails when the parent type does not exist.
func Field[Args, Result any](s *Schema, parent, name, doc string) {
	s.add(parent, name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Scalar maps a Go type to a custom scalar.
func Scalar[T any](s *Schema, name, doc string) { ... }

// Describe sets the description of an object, input or enum type.
func Describe[T any](s *Schema, doc string) { ... }

// SDL checks every registration and renders the schema. The output is deterministic. All
// problems are returned together, joined with errors.Join, each naming the Go type and field.
func (s *Schema) SDL() ([]byte, error) { ... }
```

A module's registrations, in a package that imports neither gqlgen nor the executor:

```go
// apps/api/internal/modules/bookkeeping/interfaces/graphql/schema.go
package graphql

// Register adds the bookkeeping queries, mutations and fields to the schema.
func Register(s *graphqlschema.Schema) {
	graphqlschema.Mutation[CreateTransactionArgs, dto.Transaction](s, "createTransaction",
		"Applies the postings as one linked chain of TigerBeetle transfers: all apply or none.")
	graphqlschema.Field[graphqlschema.NoArgs, dto.LedgerStats](s, "Ledger", "stats",
		"Counts and amounts of the ledger, read from Postgres and TigerBeetle.")
	graphqlschema.Field[graphqlschema.NoArgs, []dto.AssetSummary](s, "Ledger", "assets",
		"Assets used so far, with their TigerBeetle ledger numbers and issued amounts.")
	graphqlschema.Field[AccountsArgs, AccountConnection](s, "Ledger", "accounts", "Accounts in address order.")
	graphqlschema.Field[TransactionsArgs, TransactionConnection](s, "Ledger", "transactions",
		"Committed transactions, newest first.")
}

// CreateTransactionArgs follows the GraphQL convention of one input argument per mutation.
type CreateTransactionArgs struct {
	Input CreateTransactionInput `json:"input"`
}

// CreateTransactionInput is the REST body plus the ledger and the Idempotency-Key.
type CreateTransactionInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	IdempotencyKey string            `json:"idempotencyKey" doc:"Makes the write safe to retry: the same key and input replay the first answer (ADR-0007)."`
	Postings       []PostingInput    `json:"postings" doc:"1 to 64 postings, applied atomically."`
	Reference      *string           `json:"reference"`
	EventType      *string           `json:"eventType"`
	BusinessTime   *time.Time        `json:"businessTime"`
	Metadata       *scalar.Metadata  `json:"metadata"`
}

// PostingInput is the input form of dto.Posting. GraphQL forbids one type name for an input and
// an output, so the input side gets its own name over the same fields.
type PostingInput dto.Posting

// AccountsArgs are the arguments of Ledger.accounts.
type AccountsArgs struct {
	First  *int32  `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After  *string `json:"after" doc:"The endCursor of the previous page."`
	Prefix *string `json:"prefix"`
	Search *string `json:"search"`
	Asset  *string `json:"asset"`
}
```

### 8.3 Reflection rules

| Go                                                                    | GraphQL                                                                                        |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Struct used as a result                                               | `type <GoName>`; one field per exported struct field                                           |
| Struct used inside an argument                                        | `input <GoName>`; a struct reachable both as input and as output is an error                   |
| Field name                                                            | the `json` tag name; `json:"-"` and unexported fields are skipped; `omitempty` changes nothing |
| Embedded struct                                                       | its fields are flattened, as `encoding/json` does                                              |
| `doc:"..."` tag                                                       | field or argument description; `Describe[T]` for type descriptions                             |
| `default:"..."` tag on an argument                                    | default value                                                                                  |
| `deprecated:"reason"` tag                                             | `@deprecated(reason: ...)`                                                                     |
| Args struct fields                                                    | one argument each; `NoArgs` means none                                                         |
| Named string type with an `EnumValues() []string` method              | `enum <GoName>`; values must be valid GraphQL names                                            |
| Type registered with `Scalar[T]`                                      | that scalar, whatever its kind                                                                 |
| `string`, `bool`                                                      | `String`, `Boolean`                                                                            |
| `int`, `int32`, `int16`, `int8`, `uint16`, `uint8`                    | `Int` (32-bit)                                                                                 |
| `int64`, `uint32`                                                     | `Int64` (JSON number; counts and TigerBeetle ledger numbers only)                              |
| `time.Time`                                                           | `DateTime`                                                                                     |
| `uint64`, `float32`, `float64`, maps, interfaces, channels, functions | error, naming the field: money and IDs must use a named scalar type, never a number            |

### 8.4 Scalars

| Scalar         | Go type (`internal/platform/scalar`) | Wire form                                                          | REST component with the same type   |
| -------------- | ------------------------------------ | ------------------------------------------------------------------ | ----------------------------------- |
| `Amount`       | `scalar.Amount`                      | unsigned 128-bit integer as a decimal string                       | `Amount`                            |
| `SignedAmount` | `scalar.SignedAmount`                | signed decimal string (balances)                                   | `SignedAmount`                      |
| `U128`         | `scalar.ID`                          | 32 lowercase hexadecimal characters, big-endian, time ordered      | `Id`                                |
| `U64`          | `scalar.Timestamp`                   | TigerBeetle nanoseconds as a decimal string                        | `TigerBeetleTimestamp`              |
| `Metadata`     | `scalar.Metadata`                    | JSON object of strings, at most 64 entries                         | `Metadata`                          |
| `JSON`         | `scalar.JSON`                        | free-form JSON object (the stored request and response of a write) | inline `additionalProperties: true` |
| `DateTime`     | `time.Time`                          | RFC 3339                                                           | `format: date-time`                 |
| `Int64`        | `int64`                              | JSON number                                                        | `format: int64`                     |

Each scalar type has `MarshalGQL(io.Writer)` and `UnmarshalGQL(any) error` methods, which is all
gqlgen needs; they are plain method signatures, so the scalar package does not import gqlgen.
`UnmarshalGQL` for `Amount`, `SignedAmount`, `U128` and `U64` accepts JSON strings only and rejects
numbers, so no client can lose precision through a float. The same types implement
`huma.SchemaProvider` for REST (7.4), which is how REST and GraphQL share one definition per scalar.
Addresses, asset codes and ledger names are `String` in GraphQL: they are text, and the domain
validates them.

### 8.5 Nullability

- Non-pointer field: non-null (`T!`). Pointer field: nullable (`T`).
- Slice: `[T!]!`. Pointer to slice: `[T!]`. Slice of pointers: `[T]!`.
- Arguments follow the same rule; a pointer argument with a `default` tag is nullable with a
  default (`first: Int = 25`).
- A query for one object by key returns a nullable type and `null` when it does not exist
  (`ledger(name: String!): Ledger`), the GraphQL convention; the console turns `null` into the
  router's not-found view. Mutations return non-null types and errors.

### 8.6 Connections

Lists use a cursor connection: `type <Item>Connection { nodes: [<Item>!]! pageInfo: PageInfo! }`
with `PageInfo { endCursor: String, hasNextPage: Boolean! }`, and the arguments `first` (default
25, at most 100, as REST) and `after`. `endCursor` is exactly the opaque cursor REST returns as
`next`, so the use cases do not change and a cursor taken from one transport works on the other.
`edges` with a cursor per item are left out: the API's cursors are page cursors (ADR-0004); adding
per-item cursors later is additive. Connections are concrete structs (`AccountConnection`), not a
generic type, because gqlgen binds by type name; `SDL()` checks that every type whose name ends in
`Connection` has exactly `nodes` and `pageInfo`.

### 8.7 Keeping registrations free of gqlgen

- Registrations live in `modules/<m>/interfaces/graphql` and are collected by
  `internal/transport/graphql/schema` (`RegisterAll`). `cmd/graphqlschema` imports only that package
  and `internal/platform/graphqlschema`.
- depguard denies `github.com/99designs/gqlgen` and `internal/transport/graphql/exec` in all four
  places (section 14). The module resolvers in the same package do not need gqlgen either: they take
  and return module types, and errors are converted by the error presenter.
- Consequence: when a registration changes and the executor is stale, `just generate` still builds
  `cmd/graphqlschema`, writes the new schema, then regenerates the executor. The order is in
  section 16.

### 8.8 gqlgen configuration

```yaml
# apps/api/gqlgen.yml
schema:
  - ../../packages/graphql/schema.graphql
exec:
  layout: single-file
  filename: internal/transport/graphql/exec/exec.go
  package: exec
model:
  filename: internal/transport/graphql/exec/models_gen.go # stays empty when every type is bound
  package: exec
# No `resolver:` block: gqlgen generates the resolver interfaces only; the implementations are
# hand-written (8.9), so regeneration never rewrites hand-written code.
autobind:
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/graphql
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/graphql
  - github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/graphql
models:
  U128:
    model: github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar.ID
  U64:
    model: github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar.Timestamp
  DateTime:
    model: github.com/99designs/gqlgen/graphql.Time
  Int64:
    model: github.com/99designs/gqlgen/graphql.Int64
omit_slice_element_pointers: true
skip_mod_tidy: true
```

`autobind` makes gqlgen bind a schema type to the Go type of the same name in the listed packages,
so `Transaction` binds to `bookkeeping/interfaces/dto.Transaction`, the very struct REST returns.
Only scalars whose GraphQL name differs from the Go name (`U128`, `U64`) and gqlgen's built-ins
need a `models` entry. The generator guarantees that no two registered Go types share a GraphQL
name, so autobind is never ambiguous.

Shared with REST where shapes match: `Transaction`, `PostingRecord` (embeds `Posting`), `Hold`,
`AccountSummary`, `AssetBalance`, `AssetSummary`, `LedgerStats`, `HoldStats`, `Rule`,
`EventType`, `WriteRequest`. GraphQL-only: the connections, `PageInfo`, the inputs, and `Ledger`,
whose GraphQL shape differs (REST's `assets: [Asset]` and `accounts: int64` become nested
`assets: [AssetSummary!]!` and an `accounts` connection, and the count becomes `accountCount`).

Two gqlgen behaviours are confirmed in the GraphQL spike (S6) before phase 4: that a schema file
outside `apps/api` is inlined into the generated executor (Go's `embed` cannot reach outside the
package directory, and the Docker build context is `apps/api`, V10), and that autobind resolves
promoted fields of embedded structs (`PostingRecord`). Fallbacks: `cmd/graphqlschema` also writes
a copy next to the executor; `PostingRecord` lists its fields instead of embedding.

### 8.9 Per-module resolvers, one gqlgen root

gqlgen generates one `ResolverRoot` with one `QueryResolver`, one `MutationResolver` and one
`<Type>Resolver` per type with resolver fields. Each module writes its resolver methods on its own
types in `modules/<m>/interfaces/graphql`; `internal/transport/graphql` builds the root by
embedding them.

Go note: embedding a pointer to a struct in another struct promotes the embedded type's methods.
If two modules define the same method, the selector is ambiguous and the root no longer satisfies
gqlgen's interface: a compile error. If a module forgets a method, the same compile error names it.

```go
// apps/api/internal/modules/bookkeeping/interfaces/graphql/mutation.go
// Mutation holds the bookkeeping mutation resolvers.
type Mutation struct {
	transactions application.TransactionCommands
}

func NewMutation(transactions application.TransactionCommands) *Mutation {
	return &Mutation{transactions: transactions}
}

// CreateTransaction resolves Mutation.createTransaction. The signature is the one gqlgen generates
// from the schema; the use case is the one REST calls.
func (m *Mutation) CreateTransaction(ctx context.Context, input CreateTransactionInput) (*dto.Transaction, error) {
	view, err := m.transactions.Create(ctx, application.CreateTransactionCommand{
		Ledger:    string(input.Ledger),
		Key:       input.IdempotencyKey,
		RequestID: requestid.From(ctx),
		Postings:  postingsFrom(input.Postings),
		// reference, event type, business time and metadata mapped the same way
	})
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	t := dto.TransactionFrom(view)
	return &t, nil
}
```

```go
// apps/api/internal/transport/graphql/resolver.go
// Resolver is the gqlgen root. Each module contributes its resolvers by embedding.
type Resolver struct {
	query    *queryResolver
	mutation *mutationResolver
	ledger   *ledgerResolver
}

type queryResolver struct {
	*ledgersgql.Query
	*bookkeepinggql.Query
	*healthgql.Query
}

type mutationResolver struct {
	*ledgersgql.Mutation
	*bookkeepinggql.Mutation
}

func (r *Resolver) Query() exec.QueryResolver       { return r.query }
func (r *Resolver) Mutation() exec.MutationResolver { return r.mutation }
func (r *Resolver) Ledger() exec.LedgerResolver     { return r.ledger }
```

Fields that one module adds to another module's type (bookkeeping's `Ledger.stats`,
`Ledger.assets`, `Ledger.accounts`, `Ledger.transactions`, `Ledger.holds`, `Ledger.requests`,
`Ledger.search`) receive the parent object, which is a ledgers type. To keep bookkeeping from
importing ledgers, the transport package delegates with one line each:

```go
// apps/api/internal/transport/graphql/ledger_fields.go
type ledgerResolver struct {
	*ledgersgql.LedgerFields              // Ledger.rules, Ledger.eventTypes: same module
	bookkeeping *bookkeepinggql.LedgerFields
}

// Stats resolves Ledger.stats, registered by bookkeeping on a type owned by ledgers.
func (r *ledgerResolver) Stats(ctx context.Context, obj *ledgersgql.Ledger) (*bookkeepingdto.LedgerStats, error) {
	return r.bookkeeping.Stats(ctx, string(obj.Name))
}
```

The transport package may import modules (it composes them); modules may not import it.
A list of ledgers with nested stats costs one stats call per ledger; the console asks for stats of
one ledger at a time, so no data loader is added now.

### 8.10 Golden tests

- `internal/platform/graphqlschema/schema_test.go`: a table of cases, each a small set of fixture
  types registered in the test and a golden file `testdata/<case>.graphql`. Cases cover every row
  of 8.3 and 8.5, connections, enums, scalars, embedded structs, `Field` on another module's type,
  descriptions with quotes and newlines, and determinism (two renders, byte-equal).
- Error cases compare the joined error text with go-cmp: unsupported kind, money as `uint64`, a
  struct used as input and output, duplicate GraphQL names, unknown parent type, connection shape.
- Every golden file is parsed and validated with gqlparser in the test.
- `UPDATE_GOLDEN=1 go test ./internal/platform/graphqlschema` rewrites the golden files; an
  environment variable rather than a test flag keeps package-level `flag` variables out of the
  test.
- CI's drift check covers the real schema: `just generate` must leave
  `packages/graphql/schema.graphql` unchanged.

### 8.11 Serving GraphQL

- `POST /graphql` on the same `ServeMux` and middleware as REST (request ID, access log, panic
  recovery), behind `GRAPHQL_ENABLED` (default true). Only the POST transport is enabled.
- `handler.New(exec.NewExecutableSchema(...))` with: the error presenter of section 9, a recover
  function that logs the panic and answers `INTERNAL`, a fixed complexity limit and a depth limit,
  introspection on (the schema is public in the repository), the playground off (it loads React
  from a CDN). Confirmed (Q11).
- The console reaches it as `/api/graphql` through the existing `/api` proxy rewrite
  (`apps/console/vite.config.ts:25-31`); production proxies map `/api` the same way.

### 8.12 Sample schema

The opening of `packages/graphql/schema.graphql` as the generator would write it, abridged to the
fields the console's overview and new-transaction pages use.

```graphql
# Generated by cmd/graphqlschema from the modules' registrations. Do not edit.
# Abridged for this plan: types show only some of their fields.

"Unsigned 128-bit integer in the asset's smallest unit, as a decimal string."
scalar Amount
"Signed balance in the asset's smallest unit, as a decimal string."
scalar SignedAmount
"TigerBeetle 128-bit ID: 32 lowercase hexadecimal characters, big-endian, time ordered."
scalar U128
"RFC 3339 date-time."
scalar DateTime
"64-bit integer as a JSON number. Counts only, never money."
scalar Int64
"Key/value pairs for your own systems: a JSON object of strings, at most 64 entries."
scalar Metadata

type Query {
  "A ledger by name, or null when no ledger has that name."
  ledger(name: String!): Ledger
}

type Mutation {
  "Applies the postings as one linked chain of TigerBeetle transfers: all apply or none."
  createTransaction(input: CreateTransactionInput!): Transaction!
}

"A ledger is a namespace. Money never crosses ledgers."
type Ledger {
  name: String!
  metadata: Metadata!
  accountCount: Int64!
  "Assets used so far, with their TigerBeetle ledger numbers and issued amounts."
  assets: [AssetSummary!]!
  stats: LedgerStats!
  "Accounts in address order."
  accounts(first: Int = 25, after: String, prefix: String, asset: String): AccountConnection!
  "Committed transactions, newest first."
  transactions(first: Int = 25, after: String, address: String): TransactionConnection!
}

type AssetSummary {
  asset: String!
  "The TigerBeetle ledger number of this (ledger, asset) pair."
  tbLedger: Int64!
  "Money issued, minus the balance of world, read from TigerBeetle."
  issued: SignedAmount!
}

type LedgerStats {
  accounts: Int64!
  accountsToday: Int64!
  holds: HoldStats!
}

type HoldStats {
  open: Int64!
  captured: Int64!
}

type PageInfo {
  "Pass as after to read the next page. Null on the last page."
  endCursor: String
  hasNextPage: Boolean!
}

type AccountConnection {
  nodes: [AccountSummary!]!
  pageInfo: PageInfo!
}

type TransactionConnection {
  nodes: [Transaction!]!
  pageInfo: PageInfo!
}

type AccountSummary {
  address: String!
  balances: [AssetBalance!]!
}

type AssetBalance {
  asset: String!
  accountId: U128!
  "Credits posted minus debits posted."
  balance: SignedAmount!
}

type Transaction {
  id: U128!
  postings: [PostingRecord!]!
  committedAt: DateTime!
  idempotencyKey: String!
}

type PostingRecord {
  source: String!
  destination: String!
  asset: String!
  amount: Amount!
  transferId: U128!
}

input CreateTransactionInput {
  ledger: String!
  "Makes the write safe to retry: the same key and input replay the first answer (ADR-0007)."
  idempotencyKey: String!
  "1 to 64 postings, applied atomically."
  postings: [PostingInput!]!
  reference: String
  metadata: Metadata
}

input PostingInput {
  "The debited account."
  source: String!
  destination: String!
  asset: String!
  amount: Amount!
}
```

The console's overview page becomes one request:
`ledger(name: $name) { name accountCount assets { ... } stats { ... } transactions(first: 10) { nodes { ... } } }`.
GraphQL filters reuse the REST filters, which the existing `sqlc.narg(x) IS NULL OR` queries
already express (`apps/api/queries/transactions.sql:28-48`); no dynamic SQL and no query builder
are needed, so no query-builder ADR is written (requirement B leaves it conditional).

## 9. Error mapping across transports

### 9.1 One neutral mapping, rendered per transport

Today the mapping from domain errors to problems lives in the HTTP adapter
(`internal/adapters/httpapi/problem.go:62-127`). It moves to `internal/platform/problem`, which
turns any `error` into a transport-neutral value:

```go
// Package problem maps kernel errors to what a client may see. It never copies the text of a
// wrapped error, except the domain-written detail of an ErrInvalid (as today).
package problem

type Kind string // INVALID, NOT_FOUND, ALREADY_EXISTS, CONFLICT, REJECTED, UNAVAILABLE, INTERNAL

type Problem struct {
	Kind       Kind
	Reason     string        // stable kebab-case reason: "exceeds-credits", "request-in-flight"
	Type       string        // "/problems/<reason>" or "about:blank", as REST today
	Title      string        // from the same title table as today
	HTTPStatus int           // the REST status; also sent to GraphQL and RPC clients as "status"
	Detail     string
	Posting    int           // -1 when none
	Results    []string
	RetryAfter time.Duration
}

// From maps err. Anything that is not a kernel error is INTERNAL with no detail.
func From(err error) Problem { ... }
```

Each transport renders that value: REST as `application/problem+json` (same members as today),
GraphQL as an error with `extensions`, gRPC as a status with standard `google.rpc` details, AMQP and
NATS RPC as the `error` member of the reply envelope. Everything else (the log line with the full
error, the request ID) is per transport.

### 9.2 Domain errors

| Kernel error (examples of reasons)                                                                                                   | REST status and type                                                         | GraphQL `extensions.code`                                       | gRPC code                            | AMQP and NATS RPC `error.code`       |
| ------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------- | --------------------------------------------------------------- | ------------------------------------ | ------------------------------------ |
| `ErrInvalid` (`invalid-request`, `missing-idempotency-key`, `unknown-event-type`, `built-in-rule`, `capture-exceeds-hold`)           | 400, `/problems/<reason>`                                                    | `INVALID`                                                       | `InvalidArgument`                    | `INVALID`                            |
| `ErrNotFound` (`not-found`)                                                                                                          | 404, `/problems/not-found`                                                   | `NOT_FOUND` (a single-object query returns `null` instead, 8.5) | `NotFound`                           | `NOT_FOUND`                          |
| `ErrAlreadyExists` (`already-exists`)                                                                                                | 409, `/problems/already-exists`                                              | `ALREADY_EXISTS`                                                | `AlreadyExists`                      | `ALREADY_EXISTS`                     |
| `ErrConflict` (`already-reverted`, `hold-already-resolved`, `hold-expired`)                                                          | 409, `/problems/<reason>`                                                    | `CONFLICT`                                                      | `FailedPrecondition`                 | `CONFLICT`                           |
| `ErrConflict` `request-in-flight` (has `RetryAfter`)                                                                                 | 409 with `Retry-After`                                                       | `CONFLICT` with `retryAfter`                                    | `Aborted` with `RetryInfo`           | `CONFLICT` with `retryAfter`         |
| `ErrRejected` (`exceeds-credits` and other TigerBeetle results, `idempotency-key-reused`, `prefix-too-broad`, `history-not-enabled`) | 422, `/problems/<reason>`, with `posting` and `results` for a declined chain | `REJECTED` with `posting`, `results`                            | `FailedPrecondition`                 | `REJECTED` with `posting`, `results` |
| `ErrUnavailable` (`dependency-unavailable`; `write-pending` has `RetryAfter`)                                                        | 503, with `Retry-After` for `write-pending`                                  | `UNAVAILABLE` (with `retryAfter`)                               | `Unavailable` (with `RetryInfo`)     | `UNAVAILABLE` (with `retryAfter`)    |
| Anything else, panics                                                                                                                | 500, `about:blank`, no detail                                                | `INTERNAL`, no detail                                           | `Internal`, message "internal error" | `INTERNAL`, no detail                |

Members carried by every non-REST error, with the REST member names so the console and other
clients share one parser: `code` (the kind), `reason`, `type`, `title`, `status` (the REST status,
informative), `detail` (when the domain wrote one), `requestId`, `retryAfter` (whole seconds,
rounded up as the `Retry-After` header is today), `posting`, `results`.

gRPC carries them in the standard detail messages of `google.rpc` attached to the status
(decided, Q12), so any gRPC client library can read them without a project proto:

- `ErrorInfo`, following Google's AIP-193 (decided, R3):
  - `reason`: the problem type's slug in UPPER_SNAKE_CASE (`/problems/exceeds-credits` becomes
    `EXCEEDS_CREDITS`). One function in `internal/platform/problem` derives it from the type, so
    the two can never drift; `about:blank` has no slug and uses the kind (`INTERNAL`).
  - `domain`: `tigerbeetle-platform-api`, one fixed service identifier, the same string as the
    `service` field of `/v1/_info`; ADR-0021 records it.
  - `metadata` (string to string): `type` (the problem type URI, unchanged), `code`, `title`,
    `status`, `detail`, `requestId`, `posting` and `results` (TigerBeetle result names,
    comma-separated).
- `RetryInfo`: `retry_delay` for `request-in-flight` and `write-pending`, the same delay REST sends
  as `Retry-After`.
- `BadRequest`: one field violation for an input error whose domain error names the field (the
  posting of an invalid posting, the missing `idempotency_key`), with the domain's detail text as
  the description. Input errors that name no field carry no `BadRequest`.

No new module: these messages are in `google.golang.org/genproto/googleapis/rpc/errdetails`, the
module grpc-go's `status` package already imports. Phase 5 verifies it with `go mod graph` before
the first import. A unit test walks every problem type the `problem` package knows and checks
that its derived reason matches AIP-193's pattern `[A-Z][A-Z0-9_]+[A-Z0-9]` and is at most 63
characters long, so a new problem type with an unusable slug fails the build.

### 9.3 Errors raised by a transport itself

| Situation                                            | REST                                           | GraphQL                                                                                                                       | gRPC                                           | AMQP and NATS RPC                                               |
| ---------------------------------------------------- | ---------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | --------------------------------------------------------------- |
| Unparsable request                                   | 400 `invalid-request` (today's binding errors) | gqlgen `GRAPHQL_PARSE_FAILED`                                                                                                 | `InvalidArgument`                              | `INVALID`, reason `invalid-request`                             |
| Unknown field, wrong type, missing required argument | domain decides, as today (7.3)                 | gqlgen `GRAPHQL_VALIDATION_FAILED`; a bad scalar value (an `Amount` sent as a number) becomes `INVALID` through the presenter | `InvalidArgument`                              | `INVALID`                                                       |
| Unknown route or method                              | 404 or 405 problem, `about:blank`              | n/a                                                                                                                           | `Unimplemented`                                | `NOT_FOUND`, reason `unknown-method`                            |
| Client deadline passed before the handler started    | n/a                                            | n/a                                                                                                                           | `DeadlineExceeded`                             | `UNAVAILABLE`, reason `deadline-exceeded`, request not executed |
| Missing Idempotency-Key on a money write             | 400 `missing-idempotency-key`                  | same problem via `INVALID` (the field is `String!`, so only an empty string reaches the domain)                               | `InvalidArgument` from the domain, same reason | `INVALID`, same reason                                          |

### 9.4 No driver text, tested per transport

Each transport's adapter tests include one case that returns an error wrapping a fake driver
message (`fmt.Errorf("postgres: %w: secret driver text", ledger.ErrUnavailable)`) and asserts the
text appears in no response byte. The rule from `AGENTS.md:49-50` thus has a test in REST, GraphQL,
gRPC and RPC, not only in the HTTP adapter.

## 10. Idempotency across transports

The protocol does not move: `writeflow.go` in bookkeeping's application layer (today
`internal/app/writes.go`) still inserts the pending record with every minted ID before any
TigerBeetle call, classifies the chain and resolves the record. What each transport adds is only
how the key and the request ID arrive.

| Transport                  | Idempotency-Key                                                       | Request ID                                         | Where it is checked             |
| -------------------------- | --------------------------------------------------------------------- | -------------------------------------------------- | ------------------------------- |
| REST                       | `Idempotency-Key` header on the 5 money writes (unchanged)            | `X-Request-Id` header, echoed                      | domain `ValidateIdempotencyKey` |
| GraphQL                    | `idempotencyKey: String!` field of the mutation input                 | `X-Request-Id` HTTP header, echoed                 | same                            |
| gRPC                       | `string idempotency_key` field of the request message (AIP-155 style) | `x-request-id` metadata, echoed in header metadata | same                            |
| AMQP RPC                   | `idempotency-key` message header                                      | `x-request-id` message header, echoed in the reply | same                            |
| NATS RPC                   | `Idempotency-Key` message header                                      | `X-Request-Id` header, echoed                      | same                            |
| Kafka (inbound, not built) | would be the record key                                               | record header                                      | same                            |

Why a field on GraphQL and gRPC and a header elsewhere: GraphQL can carry several mutations in
one HTTP request, and a header cannot say which mutation it belongs to; in GraphQL and gRPC the
field is also typed and documented by the schema, so codegen makes forgetting it a compile error in
the console. REST and the brokers already have per-message headers, and keeping the REST JSON body
as the RPC payload means no new body shape.

Rules that hold on every transport, enforced by the shared application code:

- A key maps to exactly one outcome, per ledger. The body hash is computed over the normalised
  command (`canonicalRequest`, `writes.go:106-113`), not over wire bytes, so the same write sent
  once over REST and retried over GraphQL with the same key is a replay, not a conflict. A
  different operation or different content under the same key is `idempotency-key-reused`.
- Replay of a committed write re-reads the resource (unchanged, audit X6) and each transport
  renders it as a success. Replay of a declined write returns the stored problem, mapped by kind.
- An in-flight key answers `request-in-flight` with a retry delay on every transport (9.2).
- The stored `response_status` stays the REST number (201, 422 and so on): it is exposed as
  `responseStatus` and read by the console's stored-problem view. It moves from the use-case code
  to `bookkeeping/domain/outcome.go` as named constants documented as "numbered like HTTP statuses".
  GraphQL and RPC expose it as a data field of a write request, but no transport derives its own
  status or code from it; they map from the problem kind (conflict K19).
- Broker redelivery is safe: a write redelivered by RabbitMQ carries its key and replays; reads
  have no side effect.

## 11. gRPC, AMQP RPC and NATS RPC

### 11.1 gRPC (phase 5)

- Protos in `packages/proto/tigerbeetle/platform/v1/`, one proto package
  `tigerbeetle.platform.v1`, `go_package` pointing at `internal/platform/grpcapi/platformv1`.
  Services mirror the REST tags: `LedgerService`, `SettingsService`, `AccountService`,
  `TransactionService`, `HoldService`, `WriteRequestService`, `OperationsService` (Info,
  ReconcilerStatus); liveness and readiness use the standard `grpc.health.v1` service from
  `google.golang.org/grpc/health`, fed by `internal/platform/readiness`.
- Money and IDs are `string` fields (`string amount = 4; // u128, decimal`,
  `string id = 1; // 32 hex`). Paging follows AIP-158: `page_size`, `page_token`, `next_page_token`,
  carrying the same opaque cursor as REST and GraphQL.
- `buf.yaml` with `lint: STANDARD` and `breaking: FILE`; CI runs `buf lint` and
  `buf breaking --against` the base branch, the protobuf counterpart of the oasdiff gate.
- `internal/platform/grpcserver` owns the listener and four interceptors: request ID, access log
  with slog, panic recovery, and error mapping through `platform/problem`. `Run(ctx)` calls
  `GracefulStop` on cancellation and falls back to `Stop` when `SHUTDOWN_TIMEOUT` passes (evrone
  waits forever, research (b)). Server reflection is on when gRPC is enabled (confirmed, Q11).
- Module services live in `modules/<m>/interfaces/grpc/v1` and depend on the application
  interfaces only; adapter tests run over `bufconn` (in-memory listener, part of grpc-go).
- Generated protobuf code registers itself in `init()` functions. That code is generated and
  excluded from lint; the "no `init()`" rule applies to hand-written code (conflict K6).

### 11.2 AMQP RPC and NATS RPC (phase 6), and why evrone's code is not copied (lead point 9)

research (b) found four defects in evrone's RPC code: requests handled one at a time; every
replica handles every request (fanout exchange, plain NATS subscribe); handler errors collapse into
three strings; a NATS timeout is never recognised. The design fixes each.

Shared by both brokers (`internal/platform/rpc`):

- Method names are `v1.<operationId>`, one per REST operation (`v1.createTransaction`,
  `v1.getLedger`), so there is nothing new to learn and a v2 is a new folder and prefix.
- Request body: `{"params": {...path and query parameters, REST names...}, "body": {...REST body...}}`.
  Reply body, always JSON:

  ```json
  {"ok": true, "data": {"id": "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e", "...": "..."}}
  {"ok": false, "error": {"code": "REJECTED", "reason": "exceeds-credits",
    "type": "/problems/exceeds-credits", "title": "Declined: exceeds credits", "status": 422,
    "detail": "...", "requestId": "4f1c...", "posting": 1,
    "results": ["linked_event_failed", "exceeds_credits"]}}
  ```

- A router maps method names to handlers `func(ctx context.Context, req rpc.Request) (any, error)`;
  module controllers in `modules/<m>/interfaces/rpc/v1` register their routes. The same controllers
  serve both brokers, because the envelope and the semantics are identical; only delivery differs
  (decided, Q15; conflict K21 records the deviation from "one folder per transport").
- A bounded worker pool (`RPC_CONCURRENCY`, default 16) per server; each request gets a context
  whose deadline is the earlier of the client's deadline header (`x-deadline`, Unix milliseconds)
  and `RPC_TIMEOUT_MAX` (default 30 s). A request whose deadline has passed is answered
  `UNAVAILABLE deadline-exceeded` without running.

AMQP specifics (`internal/platform/amqprpc`, over `internal/platform/amqp`):

- One durable request queue per version, `tbp.rpc.v1`, bound to a durable direct exchange
  `tbp.rpc`. Every replica consumes the same queue (competing consumers), so each request runs
  once. `basic.qos` prefetch equals the worker pool size, which is the concurrency bound.
- Replies go to the client's `reply_to`, normally RabbitMQ's direct reply-to pseudo-queue
  `amq.rabbitmq.reply-to` (https://www.rabbitmq.com/docs/direct-reply-to), with the request's
  `correlation_id`. The method is the `type` property; `expiration` mirrors the deadline so an
  unanswered request does not outlive its client.
- Manual ack after the reply is published (or after a failed publish, logged). A malformed or
  unknown request is answered with an error and acked, never requeued forever.
- `internal/platform/amqp` watches `NotifyClose`, redials with capped backoff, redeclares the
  topology and resumes consuming; readiness reports RabbitMQ down meanwhile. amqp091-go has no
  automatic reconnect (research (c)).

NATS specifics (`internal/platform/natsrpc`):

- A `micro` service (`github.com/nats-io/nats.go/micro`) with queue group `tbp-api`: NATS delivers
  each request to one member of the group. Endpoints are `tbp.rpc.v1.<operationId>`.
- micro calls a handler serially per subscription, so the handler acquires a worker-pool slot and
  answers from a goroutine (`Request.Respond` may be called later); the pool bounds concurrency.
- Error replies also set micro's `Nats-Service-Error` and `Nats-Service-Error-Code` headers (reason
  and REST status) so `nats micro` tooling shows them; the body is still the envelope.
- The test client maps `nats.ErrTimeout` to a timeout error and `nats.ErrNoResponders` to
  unavailable, the two cases evrone confuses.
- Core NATS has no redelivery; a client retries a money write with the same key.

### 11.3 Starting and stopping every server from one place

Every server and worker implements one interface; `internal/bootstrap` starts the enabled ones and
stops them together.

```go
// apps/api/internal/bootstrap/run.go (sketch)

// Runner is a server or a background worker. Run blocks until ctx is cancelled, then drains
// within its own shutdown timeout, and returns nil on a clean stop.
type Runner interface {
	Name() string
	Run(ctx context.Context) error
}

// Run starts every runner and returns when ctx is cancelled or one runner fails; the others are
// then cancelled too. Resources close afterwards, in reverse order of construction.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, r := range a.runners {
		g.Go(func() error {
			if err := r.Run(gctx); err != nil {
				return fmt.Errorf("%s: %w", r.Name(), err)
			}
			return nil
		})
	}
	err := g.Wait()
	a.close()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
```

Go note: `errgroup.WithContext` returns a context that is cancelled as soon as one goroutine
returns an error, which is how a failed NATS connection stops the HTTP server too instead of
leaving a half-working process. Since Go 1.22 each loop iteration has its own `r`, so the closure
captures the right runner.

Runners: the HTTP server (REST, GraphQL and health), the gRPC server, the AMQP RPC server, the
NATS RPC server, the reconciler (today a goroutine in `cmd/api/main.go:82-84`) and, in phase 7,
the CDC relay. Constructors return errors instead of exiting, so deferred cleanup always runs
(evrone calls `log.Fatal`, research (b)).

### 11.4 Tests per transport

- Use cases: unit tests in each module's `application` package, with hand-written fakes of the
  domain ports and of the cross-module ports; no transport package is imported.
- REST: `httptest` against a Huma API on a fresh mux with fakes of the application interfaces,
  plus the replay test of 7.5.
- GraphQL: resolver tests on fakes; handler tests through gqlgen's executor for the error
  presenter (codes and extensions of 9.2, no driver text).
- gRPC: services over `bufconn` with fakes; interceptor tests for status and details.
- AMQP and NATS RPC: router and controller tests on fakes with no broker; envelope encoding tests;
  then testcontainers integration tests per broker (tag `integration`, `-race`).
- Kafka relay: application test with a fake publisher; integration test of 12.4.
- Full stack: `just test-e2e` (tag `e2e`) calls REST, GraphQL, gRPC and both RPC transports of the
  compose stack from the host, through its published ports.

## 12. Kafka

### 12.1 Facts that decide it (lead point 8, research (e))

- TigerBeetle CDC exists since 0.16.43 and is in the pinned 0.17.9 server; the job is the
  `tigerbeetle amqp` subcommand of the same binary, so it runs from the image the stack already
  pins, which also satisfies "the CDC job must not be newer than the cluster" and the one-tag check
  of `scripts/check-versions.sh` (V9).
- It publishes only to AMQP 0.9.1 (RabbitMQ). Kafka needs a bridge.
- Delivery is at-least-once, in ascending TigerBeetle timestamp order, by a single publisher
  (enforced with an exclusive RabbitMQ queue). Progress lives in a RabbitMQ queue; a restart
  resumes after the last acknowledged timestamp.
- `--host` takes `ip:port` only, no DNS names; TigerBeetle addresses are IP-only too (handoff
  fact 3). The job uses io_uring, so its container needs the same seccomp setting as the server.
- The docs say u128 and u64 values are JSON strings, but their example shows numbers. The relay
  must decode both without going through a float.
- The Postgres outbox of ADR-0002 was never built (audit X7), and it could not close the gap
  anyway: the commit mark and the outbox row are written after the TigerBeetle commit, so a crash in
  between leaves a committed transfer with no outbox row until the reconciler runs.

### 12.2 Decision

An outbound stream of ledger events only. No inbound Kafka command transport for now.

- Ledger events (every transfer TigerBeetle commits: single-phase, pending, posted, voided,
  expired) flow TigerBeetle CDC -> RabbitMQ durable queue -> relay in the API process -> Kafka
  topic `tbp.ledger-events.v1`. The source is TigerBeetle's own committed state, read by timestamp,
  so there is no dual-write gap: a crash after a commit delays an event, it cannot lose it, as long
  as RabbitMQ keeps what it accepted (durable queue, persistent messages, the job publishes with
  `mandatory`).
- No outbox for ledger events. Events born in Postgres (ledger created, metadata replaced, rules
  and event types changed) are not published (decided, Q8): nothing consumes them yet. The
  ADR-0002 outbox stays the planned path for the day a consumer needs them.
- Inbound commands over Kafka are not built: AMQP RPC and NATS RPC already carry commands with
  replies, Kafka has no reply channel (a reply topic plus correlation is the RPC pattern rebuilt by
  hand), and the idempotency key already makes broker redelivery safe. The design is ready if a
  real integrator needs it: a consumer group on `tbp.commands.v1`, the record key as the
  Idempotency-Key, the RPC envelope as the value, replies on the topic named in a `reply-topic`
  header, offsets committed after the reply. Confirmed: not built (Q9).

### 12.3 The relay

- `modules/bookkeeping/interfaces/cdc`: consumes the CDC queue `tbp.ledger-events.relay`, declared
  with `x-single-active-consumer: true`, so every API replica can run the relay but only one is
  active at a time and order is kept. Decodes the message with `json.Decoder.UseNumber`, accepting
  strings or numbers for u128 and u64 values, into exact integers.
- `modules/bookkeeping/application/relay_ledger_event.go`: a command that enriches the event with
  facts Postgres already holds before the transfer exists (ledger name and asset by
  `tb_ledger`, addresses by account ID: both written before `create_transfers`, ADR-0005 and
  ADR-0006), and publishes through a `LedgerEventPublisher` port. It does not join transaction
  metadata or reference: those rows are written after the commit and could race (research (e)).
  Consumers that need them call the API with the transaction ID.
- `modules/bookkeeping/infrastructure/kafka`: franz-go idempotent producer, `acks=all`. The
  RabbitMQ delivery is acknowledged only after Kafka acknowledges the record. Kafka or Postgres
  down: the delivery is requeued after a capped backoff and readiness reports the dependency down.
- Record key: the transaction ID (`user_data_128` as 32 hex), so the legs of one transaction land
  in one partition in order. Value: JSON with every u128 and u64 as a string (amounts, IDs,
  timestamps, balances). Headers: `event-type`, `tb-timestamp`. Consumers deduplicate on
  `tb-timestamp`, which TigerBeetle makes unique per event.

### 12.4 Compose and tests

- Compose profile `brokers` adds `tigerbeetle-cdc` (image `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`,
  `command: ['amqp', '--addresses=<tigerbeetle ip>:3000', '--host=<rabbitmq ip>:5672', ...]`,
  `seccomp=unconfined`). The compose network gets a fixed subnet and the `tigerbeetle` and
  `rabbitmq` services fixed `ipv4_address` values, because neither the job nor the client accepts
  names. RabbitMQ loads a definitions file that declares the CDC exchange, the relay queue and the
  binding, so the job never publishes to a missing exchange.
- Integration test (tag `integration`): testcontainers start TigerBeetle, Postgres, RabbitMQ, the
  CDC job (same image, container IPs read at run time) and Kafka (`apache/kafka:4.3.2`, generic
  container, fixed advertised listener); one transaction yields one Kafka record per leg with
  string amounts; killing the relay mid-stream and restarting it produces duplicates only, never
  gaps.

### 12.5 Spikes (research Appendix B)

- S3, CDC: before ADR-0022 is merged (phase 0). Run `tigerbeetle amqp` from the 0.17.9 image
  against RabbitMQ 4.4.0 with fixed IPs, capture one message of each type, record how `id` and
  `amount` are encoded, time a kill-and-restart replay. If CDC cannot be used, ADR-0022 falls back
  to a relay that calls the Go client's experimental `GetChangeEvents` with its checkpoint in
  Kafka (research (e), option 3), and the owner is asked again.
- S4, Kafka: at the start of phase 7. `apache/kafka:4.3.2` through a generic testcontainer with a
  fixed port and advertised listener; start time against `apache/kafka-native:4.3.2` and the
  Redpanda module.

## 13. Config switches and readiness

### 13.1 Switches

| Variable                                                                                   | Default                                                                | Effect                                                                                                                                                                                                             |
| ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `REST_ENABLED`                                                                             | `true`                                                                 | Serves the REST operations of ledgers and bookkeeping. The health module's routes (`/healthz`, `/readyz`, `/v1/_info`) are always served. `cmd/openapi` ignores the switch: the spec always lists every operation. |
| `GRAPHQL_ENABLED`                                                                          | `true`                                                                 | Serves `POST /graphql`.                                                                                                                                                                                            |
| `GRPC_ENABLED`, `GRPC_ADDR`                                                                | `false`, `:9090`                                                       | gRPC server.                                                                                                                                                                                                       |
| `AMQP_RPC_ENABLED`, `AMQP_URL`, `AMQP_RPC_QUEUE`                                           | `false`, none, `tbp.rpc.v1`                                            | AMQP RPC server. Enabled without a URL is a startup error.                                                                                                                                                         |
| `NATS_RPC_ENABLED`, `NATS_URL`, `NATS_RPC_QUEUE_GROUP`                                     | `false`, none, `tbp-api`                                               | NATS RPC server.                                                                                                                                                                                                   |
| `RPC_CONCURRENCY`, `RPC_TIMEOUT_MAX`                                                       | `16`, `30s`                                                            | Both RPC servers.                                                                                                                                                                                                  |
| `KAFKA_EVENTS_ENABLED`, `KAFKA_BROKERS`, `KAFKA_EVENTS_TOPIC`, `CDC_AMQP_URL`, `CDC_QUEUE` | `false`, none, `tbp.ledger-events.v1`, none, `tbp.ledger-events.relay` | CDC relay to Kafka.                                                                                                                                                                                                |
| `RECONCILER_ENABLED`                                                                       | `true`                                                                 | Runs the reconciler in the API process (decided, Q10). A `cmd/worker` comes only once a production deployment exists, which is out of scope.                                                                       |

All variables go into `internal/platform/config` (caarlos0/env, as today) and `.env.example`. The
API variables leave the `dev` env list of `turbo.json` with the Go tasks in phase 1. Compose passes
them through with every broker switch off by default, so the default dev stack
(`just compose-up`) starts no broker and enables none.

Exposure (decided, Q14): gRPC, AMQP RPC, NATS RPC and Kafka stay off by default and are
development-only until authentication is decided (`AGENTS.md:93`); ADR-0021 states it, and the
compose `brokers` profile is the only place that turns them on. GraphQL has the same status as REST
today: on by default, served next to REST, unauthenticated like it.

Background work (decided, Q10): the reconciler and the CDC relay both run inside the API process
as bootstrap runners, each behind its own switch (`RECONCILER_ENABLED`, `KAFKA_EVENTS_ENABLED`).

### 13.2 Readiness

- `internal/platform/readiness.Checker` holds named probes and runs them concurrently, each with
  `READINESS_TIMEOUT`, as `app.Status.Check` does today for two (`internal/app/status.go:67-83`).
  Postgres and TigerBeetle are always registered; `rabbitmq` when AMQP RPC or the CDC relay is
  enabled, `nats` when NATS RPC is, `kafka` when the relay is.
- Probes: Postgres ping; TigerBeetle round trip (existing single-flight probe); RabbitMQ: the
  supervised connection is open and a passive declare of the request queue answers; NATS: connected
  and a flush answers; Kafka: franz-go `Ping`.
- `/readyz` answers 503 only when Postgres or TigerBeetle is down (decided, Q7). A broker that is
  down is reported in the body, and the overall `status` becomes `degraded` with a 200. Why:
  readiness gates HTTP traffic from the load balancer, while broker consumers pull their work and
  never pass through it. Every replica shares the same broker, so failing readiness on a broker
  outage would take REST and GraphQL down everywhere without helping the broker path.
- `Readiness.status` is `enum: [ready, not_ready]` today; phase 6 adds `degraded`. oasdiff reports
  an enum value added to a response, a warning: a client that switches on the status must handle
  the new value. The console never calls `getReadiness`, so only the generated client's type
  changes. The commit body records the warning as intended.
- REST shape: the `Dependencies` object gains optional `rabbitmq`, `nats` and `kafka` members
  (`DependencyCheck`), present only when that dependency is registered. With brokers off the
  responses are byte-identical to today. The spec change is additive (V7) and lands in phases 6 and
  7 with its oasdiff line (INFO, optional response property added). GraphQL `serviceInfo` and gRPC
  `Info` expose the same list; `grpc.health.v1` reports SERVING when ready or degraded, and
  NOT_SERVING only when Postgres or TigerBeetle is down.
- `/healthz` stays liveness only. The container healthcheck keeps using `-probe .../readyz`,
  which a broker outage does not fail.

## 14. depguard rules

### 14.1 Sketch

`M` abbreviates `github.com/wilfriedago/tigerbeetle-platform/apps/api/internal`. depguard matches
`files` globs against absolute paths and `pkg` entries as import-path prefixes; every rule uses
`list-mode: lax` (everything allowed unless denied), as today. A `pkg` entry cannot contain a
wildcard, so the rules that name "another layer of any module" list each module; adding a module
means adding its lines, and the probe test (14.2) fails if one is forgotten.

```yaml
depguard:
  rules:
    kernel:
      list-mode: lax
      files: ['**/internal/kernel/**']
      deny:
        - { pkg: M, desc: the shared kernel imports nothing from internal }
        - &io-pgx { pkg: github.com/jackc/pgx, desc: no I/O here }
        - &io-tb { pkg: github.com/tigerbeetle/tigerbeetle-go, desc: no I/O here }
        - &io-http { pkg: net/http, desc: no transport here }
        - &fw-huma { pkg: github.com/danielgtaylor/huma, desc: no framework here }
        - &fw-gql { pkg: github.com/99designs/gqlgen, desc: no framework here }
        - &fw-grpc { pkg: google.golang.org/grpc, desc: no transport here }
        - &br-amqp { pkg: github.com/rabbitmq/amqp091-go, desc: no broker here }
        - &br-nats { pkg: github.com/nats-io/nats.go, desc: no broker here }
        - &br-kafka { pkg: github.com/twmb/franz-go, desc: no broker here }
    module-domain:
      list-mode: lax
      files: ['**/internal/modules/*/domain/**']
      deny:
        - { pkg: M/modules, desc: domain depends on the kernel only }
        - { pkg: M/platform, desc: domain depends on the kernel only }
        - { pkg: M/transport, desc: domain depends on the kernel only }
        - { pkg: M/bootstrap, desc: domain depends on the kernel only }
        - *io-pgx
        - *io-tb
        - *io-http
        - *fw-huma
        - *fw-gql
        - *fw-grpc
        - *br-amqp
        - *br-nats
        - *br-kafka
    module-application:
      list-mode: lax
      files: ['**/internal/modules/*/application/**']
      deny:
        - { pkg: M/modules/ledgers/infrastructure, desc: application -> domain only }
        - { pkg: M/modules/ledgers/interfaces, desc: application -> domain only }
        - { pkg: M/modules/bookkeeping/infrastructure, desc: application -> domain only }
        - { pkg: M/modules/bookkeeping/interfaces, desc: application -> domain only }
        - { pkg: M/modules/health, desc: health has no application layer to call }
        - { pkg: M/platform, desc: use cases get platform services through ports }
        - { pkg: M/transport, desc: use cases know no transport }
        - { pkg: M/bootstrap, desc: use cases know no wiring }
        - *io-pgx
        - *io-tb
        - *io-http
        - *fw-huma
        - *fw-gql
        - *fw-grpc
        - *br-amqp
        - *br-nats
        - *br-kafka
    module-infrastructure:
      list-mode: lax
      files: ['**/internal/modules/*/infrastructure/**']
      deny:
        - { pkg: M/modules/ledgers/application, desc: infrastructure -> domain only }
        - { pkg: M/modules/ledgers/interfaces, desc: infrastructure -> domain only }
        - { pkg: M/modules/bookkeeping/application, desc: infrastructure -> domain only }
        - { pkg: M/modules/bookkeeping/interfaces, desc: infrastructure -> domain only }
        - { pkg: M/transport, desc: infrastructure knows no transport }
        - { pkg: M/bootstrap, desc: infrastructure knows no wiring }
        - *io-http
        - *fw-huma
        - *fw-gql
        - *fw-grpc
    module-interfaces:
      list-mode: lax
      files: ['**/internal/modules/*/interfaces/**']
      deny:
        - { pkg: M/modules/ledgers/infrastructure, desc: interfaces call use cases, never drivers }
        - { pkg: M/modules/bookkeeping/infrastructure, desc: interfaces call use cases, never drivers }
        - { pkg: M/transport, desc: transports compose modules, not the reverse }
        - { pkg: M/bootstrap, desc: no wiring in interfaces }
        - { pkg: M/platform/postgres, desc: no driver in interfaces }
        - { pkg: M/platform/tigerbeetle, desc: no driver in interfaces }
        - *io-pgx
        - *io-tb
    # One rule per module: no file outside a module imports it. Bootstrap and transport are
    # outside internal/modules, so they are not matched and may compose modules.
    isolate-ledgers:
      list-mode: lax
      files: ['**/internal/modules/**', '!**/internal/modules/ledgers/**']
      deny: [{ pkg: M/modules/ledgers, desc: modules talk through ports wired in bootstrap }]
    isolate-bookkeeping:
      list-mode: lax
      files: ['**/internal/modules/**', '!**/internal/modules/bookkeeping/**']
      deny: [{ pkg: M/modules/bookkeeping, desc: modules talk through ports wired in bootstrap }]
    isolate-health:
      list-mode: lax
      files: ['**/internal/modules/**', '!**/internal/modules/health/**']
      deny: [{ pkg: M/modules/health, desc: modules talk through ports wired in bootstrap }]
    platform:
      list-mode: lax
      files: ['**/internal/platform/**']
      deny:
        - { pkg: M/modules, desc: platform has no business rule }
        - { pkg: M/transport, desc: platform has no business rule }
        - { pkg: M/bootstrap, desc: platform has no business rule }
    graphql-registrations:
      list-mode: lax
      files:
        - '**/internal/modules/*/interfaces/graphql/**'
        - '**/internal/transport/graphql/schema/**'
        - '**/internal/platform/graphqlschema/**'
        - '**/internal/platform/scalar/**'
        - '**/cmd/graphqlschema/**'
      deny:
        - { pkg: github.com/99designs/gqlgen, desc: registrations must compile when gqlgen output is stale }
        - { pkg: M/transport/graphql/exec, desc: registrations must compile when gqlgen output is stale }
    spec-commands:
      list-mode: lax
      files: ['**/cmd/openapi/**', '**/cmd/graphqlschema/**']
      deny:
        - { pkg: M/platform/tigerbeetle, desc: spec commands build without cgo }
        - { pkg: M/platform/postgres, desc: spec commands open no connection }
        - { pkg: M/modules/ledgers/infrastructure, desc: spec commands build without drivers }
        - { pkg: M/modules/bookkeeping/infrastructure, desc: spec commands build without drivers }
        - { pkg: M/bootstrap, desc: spec commands build without drivers }
        - *io-pgx
        - *io-tb
```

In the real file `M` is spelled out (golangci-lint YAML has no variables; the anchors above are
plain YAML and are kept if golangci-lint's loader accepts them, otherwise the lists are written
out). The `!` negation in `files` and the `*` inside a glob are documented depguard v2 features;
the probe test proves they behave as expected with golangci-lint v2.15.0.

### 14.2 Proving the rules fire (lead point 3)

The audit found that today's globs would silently match nothing after the move (V11). A lint rule
that matches nothing looks exactly like a rule that is obeyed, so the rules get their own test:

- `apps/api/testdata/depguard/` mirrors the real paths with one deliberate violation per rule, for
  example `testdata/depguard/internal/modules/ledgers/domain/bad.go` importing `net/http`,
  `.../modules/ledgers/application/bad.go` importing `M/modules/ledgers/infrastructure/postgres`,
  `.../modules/bookkeeping/interfaces/rest/v1/bad.go` importing `M/modules/ledgers/application`,
  `.../platform/scalar/bad.go` importing gqlgen, `.../cmd/openapi/bad.go` importing
  `M/platform/tigerbeetle`. Each file compiles (the imports exist) and is ignored by `./...`
  because it is under `testdata`.
- `just lint-depguard-probe` runs golangci-lint with only depguard enabled on each fixture package,
  named explicitly, and fails unless every fixture yields exactly one depguard finding naming the
  expected rule. CI runs it in the `go` job. A rule renamed, a glob mistyped or a module added
  without its lines makes the probe fail.

## 15. packages/api-client

### 15.1 Generator choice (lead point 7)

orval 8.44.0, not @hey-api/openapi-ts. The requirement asks for hey-api unless the ADR argues
otherwise; ADR-0019 argues:

- hey-api 0.99.0 (the latest stable) imports the TypeScript compiler API at run time
  (`ts.factory`, `createPrinter`, research (c)), which TypeScript 7.0.2 does not ship. Its peer
  range (`>=5.5.3 || >=6.0.0 || 6.0.1-rc`) accepts 7.0.2, so pnpm would not warn: the generator
  would fail when run. The workaround, `"typescript": "npm:@typescript/typescript6@6.0.2"` aliased
  inside the generator package, works but keeps a second TypeScript for one tool, the situation
  the repo is leaving with openapi-typescript and its 5.9.3 pin.
- hey-api splits by tag only into classes or namespaces inside one `sdk.gen.ts`; the requirement
  is one module per tag. orval's `mode: 'tags-split'` writes one folder per tag.
- hey-api's stable line is four months old while a rewrite ships as `next` prereleases (research
  (c)); adopting it now means a migration soon.
- orval's core, fetch and query packages do not import `typescript` (research (c)), run on Node 24,
  and its `override.mutator` sends every request through one hand-written function, which is
  exactly the thin layer the requirement describes.
- openapi-typescript plus openapi-fetch was the zero-change option, but it gives no per-tag
  modules and still needs TypeScript 5.9.

Spike S2 (research Appendix B) runs orval 8.44.0 against the Huma-generated spec at the start of
phase 2 and checks OpenAPI 3.1 handling, non-null arrays, the 14 named scalars and
`exactOptionalPropertyTypes`. If orval fails it, the fallback is hey-api with the TypeScript 6
alias confined to `packages/api-client`, recorded in ADR-0019.

### 15.2 Tree

```
packages/api-client/
  package.json            devDependencies orval 8.44.0, typescript 7.0.2; scripts generate, typecheck, test, lint
  orval.config.ts         input ../openapi/openapi.yaml; mode tags-split; client fetch;
                          override.mutator ./src/client/fetcher.ts; includeHttpResponseReturnType false
  README.md
  src/
    index.ts              exports only: the tag modules, the model types, the client layer
    gen/                  orval output, committed, never edited, ignored by Oxfmt and Oxlint
      operations/operations.ts
      ledgers/ledgers.ts
      accounts/accounts.ts
      transactions/transactions.ts
      holds/holds.ts
      settings/settings.ts
      model/              one file per schema, plus index.ts
    client/
      config.ts           base URL, fetch, timeout (15 s), retries (2), request ID generator; configureClient()
      errors.ts           ApiError (problem, status, requestId, retryAfter, type), HttpStatusError, parseRetryAfter
      headers.ts          X-Request-Id and Idempotency-Key names, randomRequestId(), request header assembly
      fetcher.ts          the orval mutator: URL, headers, timeout and abort signal, retry policy, problem decoding
      config.test.ts errors.test.ts headers.test.ts fetcher.test.ts
```

### 15.3 Behaviour kept from today's client (audit 6)

- A request ID on every call (`X-Request-Id`, 32 hex from `crypto.getRandomValues`); the
  Idempotency-Key on the five money writes, as the typed header parameter orval generates from the
  spec, so a call without it does not compile.
- Retries only for GET, HEAD and OPTIONS, on network errors and 502, 503, 504, at most `retries`
  times. A 503 with `Retry-After` waits that long (capped at 10 s) before retrying; writes never
  retry. 409 is never retried; its `Retry-After` is surfaced on the error.
- `ApiError.retryAfter` parses both forms of `Retry-After` (seconds and HTTP date; today only
  seconds). `ApiError.status` is the problem's status, `requestId` the response header.
- The abort signal TanStack Query passes is forwarded to `fetch`; the timeout aborts through
  `AbortSignal.any([signal, AbortSignal.timeout(ms)])`.
- Bodies are read once and inspected by status, the lesson of handoff fact 7 (no `throwHttpErrors`
  equivalent is needed without ky).
- `configureClient()` is module state set once at start-up by the console (`lib/api.ts`); tests
  pass their own `fetch` through it.

Unit tests (`vp test`, imports from `vite-plus/test`) cover every rule above with a fake `fetch`:
header assembly, retry and no-retry cases, `Retry-After` parsing, problem decoding, abort and
timeout. The package gets a `test` script; today it has no tests at all (audit 2.7).

## 16. Justfile

### 16.1 Why a justfile despite ADR-0009 (lead point 2)

ADR-0009 rejected "a Makefile at the root calling `go` and `pnpm` by hand" because it "recomputes
everything on every run and encodes the dependency graph in shell"
(`docs/adr/0009-pnpm-turborepo-monorepo-with-go-module.md:36,40-41`). ADR-0016 answers:

- Nothing TypeScript loses its cache: TypeScript recipes call `pnpm turbo run <task>`, so
  Turborepo still orders and caches the TypeScript packages, as ADR-0011 wants.
- Go never needed Turborepo's cache: `go build` and `go test` keep their own content-addressed
  caches, and golangci-lint caches its analysis. Turborepo's Go caching existed only through the
  `apps/api/package.json` shim, which ADR-0009 itself listed as a cost, and which is already deleted.
- The cross-language graph is now a short fixed order (sqlc, then the Go emitters, then gqlgen and
  buf, then the TypeScript generators). Turborepo cannot express it without a fake package for Go;
  four recipe lines can, and they are read top to bottom.
- just is not make: no file targets or implicit rules, recipes take arguments, `just --list`
  prints every verb with its comment, one static binary pinned with a checksum.
- ADR-0009's driver "one command per verb at the root" stays true: `just generate`, `just build`,
  `just test`.

### 16.2 Recipes

| Recipe                                                                  | Does                                                                                               | Phase                      |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------------------------- |
| `generate`                                                              | the four steps of 16.4, in order                                                                   | 1 (steps grow with phases) |
| `generate-sql`, `generate-contracts`, `generate-servers`, `generate-ts` | one step each                                                                                      | 1, 2, 4                    |
| `generate-check`                                                        | `generate`, then fails if `git status --porcelain` is not empty (CI)                               | 1                          |
| `lint`                                                                  | `lint-go`, `lint-ts`, `lint-spec` (and `lint-proto` from phase 5)                                  | 1                          |
| `lint-go`                                                               | `cd apps/api && golangci-lint run ./...`                                                           | 1                          |
| `lint-ts`                                                               | `pnpm exec vp check` and `pnpm turbo run lint`                                                     | 1                          |
| `lint-spec`                                                             | Redocly on the committed spec                                                                      | 1                          |
| `lint-proto`                                                            | `buf lint`                                                                                         | 5                          |
| `lint-depguard-probe`                                                   | 14.2                                                                                               | 3                          |
| `format`                                                                | `golangci-lint fmt` in `apps/api`, `pnpm exec vp check --fix`                                      | 1                          |
| `fmt-go-files *files`                                                   | `golangci-lint fmt` on the given files (used by the pre-commit hook)                               | 1                          |
| `typecheck`                                                             | `pnpm turbo run typecheck` (Go type-checks when it builds)                                         | 1                          |
| `test`                                                                  | `test-go` (`go test -race ./...`) and `test-ts` (`pnpm turbo run test`, now including the console) | 1                          |
| `test-integration`                                                      | `go test -race -tags integration -count=1 ./...`                                                   | 1                          |
| `test-e2e`                                                              | tests tagged `e2e` against the running compose stack with brokers                                  | 6                          |
| `build`                                                                 | `build-go` (`go build -trimpath -o bin/api ./cmd/api`) and `build-ts` (`pnpm turbo run build`)     | 1                          |
| `dev`                                                                   | the API with `go run` against the compose services, and the console dev server                     | 1                          |
| `vulncheck`                                                             | `govulncheck ./...`                                                                                | 1                          |
| `compose-up`, `compose-down`, `compose-logs`, `compose-reset`           | today's `stack:*` scripts                                                                          | 1                          |
| `compose-up-brokers`                                                    | `compose-up` with `--profile brokers` and the broker switches on                                   | 6                          |
| `spec-diff base="main"`                                                 | oasdiff `breaking` and `changelog` against the spec on `base`                                      | 2                          |
| `tools-install`                                                         | `scripts/install-tools.sh`                                                                         | 1                          |
| `versions-check`                                                        | `scripts/check-versions.sh`                                                                        | 1                          |

### 16.3 Sketch (final form, after phase 5)

```just
# Single entry point for every task (ADR-0016). `just --list` shows the recipes.
set shell := ["bash", "-euo", "pipefail", "-c"]

api := "apps/api"
compose := "docker compose --project-directory . -f deploy/compose/compose.yaml"
# Tools from scripts/install-tools.sh live in $(go env GOPATH)/bin.
export PATH := `go env GOPATH` + "/bin:" + env("PATH")

# List the recipes.
default:
    @just --list

# Regenerate everything, in dependency order.
[group('generate')]
generate: generate-sql generate-contracts generate-servers generate-ts

# 1. Postgres accessors.
[group('generate')]
generate-sql:
    cd {{api}} && sqlc generate

# 2. The REST spec and the GraphQL schema, from the Go registrations, without cgo.
[group('generate')]
generate-contracts:
    cd {{api}} && CGO_ENABLED=0 go run ./cmd/openapi -out ../../packages/openapi/openapi.yaml
    cd {{api}} && CGO_ENABLED=0 go run ./cmd/graphqlschema -out ../../packages/graphql/schema.graphql

# 3. The GraphQL executor and the protobuf code, from the files step 2 and packages/proto hold.
[group('generate')]
generate-servers:
    cd {{api}} && go tool gqlgen generate --config gqlgen.yml
    cd packages/proto && buf generate

# 4. The TypeScript client, the console's GraphQL documents and its route tree.
[group('generate')]
generate-ts:
    pnpm turbo run generate

# Fail when generated files differ from the committed ones (CI).
[group('generate')]
generate-check: generate
    test -z "$(git status --porcelain)" || { git status --short; exit 1; }

[group('quality')]
lint: lint-go lint-ts lint-spec lint-proto

[group('quality')]
lint-go:
    cd {{api}} && golangci-lint run ./...

# Format the given Go files (the pre-commit hook passes the staged ones).
[group('quality')]
fmt-go-files +files:
    cd {{api}} && golangci-lint fmt {{files}}

[group('test')]
test: test-go test-ts

[group('test')]
test-go:
    cd {{api}} && go test -race ./...

[group('test')]
test-ts:
    pnpm turbo run test

[group('stack')]
compose-up:
    {{compose}} up --build --detach --wait

[group('stack')]
compose-up-brokers:
    AMQP_RPC_ENABLED=true NATS_RPC_ENABLED=true KAFKA_EVENTS_ENABLED=true {{compose}} --profile brokers up --build --detach --wait
```

### 16.4 Generate order

1. `sqlc generate` (per-module query packages; needs only SQL).
2. `cmd/openapi` and `cmd/graphqlschema` (need the Go registrations and the sqlc packages to
   compile; must not need gqlgen or protobuf output, section 8.7).
3. `go tool gqlgen generate` and `buf generate` (gqlgen reads the schema from step 2).
4. `pnpm turbo run generate`: Redocly lint then orval in `packages/api-client`, then the console's
   graphql-codegen and `tsr generate`. Turborepo orders and caches these; their `inputs` include
   `$TURBO_ROOT$/packages/openapi/openapi.yaml` and `$TURBO_ROOT$/packages/graphql/schema.graphql`.

Phase 1 has only steps 1 (sqlc and still oapi-codegen) and 4; steps 2 and 3 fill in with phases 2,
4 and 5.

### 16.5 Hooks, CI, pins

- Vite+ hooks stay (ADR-0012); only commands change in the root `vite.config.ts` `staged` block:
  Go files run `just fmt-go-files <files>` then `just lint-go`; `packages/openapi/**/*.yaml` runs
  `just lint-spec`; from phase 5 `packages/proto/**/*.proto` runs `just lint-proto`. `.githooks/`
  is unchanged and the root `prepare` script stays (it installs the hooks).
- CI jobs call recipes: `go` runs `just lint-go`, `just lint-depguard-probe`, `just test-go`,
  `just vulncheck`; `go-integration` runs `just test-integration`; `codegen-drift` runs
  `just generate-check` and, from phase 2, `just spec-diff ${{ github.base_ref }}`; `web` runs
  `just lint-ts`, `just typecheck`, `just test-ts` (console unit tests now run in CI) and the console
  build; `versions` runs `just versions-check`. Each job runs `scripts/install-tools.sh` first, which
  installs just itself.
- just 1.59.0 is pinned in `scripts/install-tools.sh` with the four sha256 values of research
  Appendix A. The first install on a new machine runs the script directly; after that
  `just tools-install` works.
- `.editorconfig` gains a `[justfile]` section (4-space indent).

## 17. Console migration path

### 17.1 Decisions

- Idempotency key: an input field (`input.idempotencyKey`), not a header (section 10).
  `useIdempotencyKey` is unchanged: `keyFor(body)` still decides the key, and the mutation passes it
  in the variables instead of the header. Renewal still happens only after a final answer.
- Copy as curl: keeps showing the REST request. REST is the stable third-party API with
  problem+json and a published spec; a curl of a GraphQL POST teaches integrators nothing they can
  build on. `src/lib/curl.ts` keeps its output; its comment changes from "the exact request the
  console sends" to "the equivalent REST request". The GraphQL input of a money mutation is the REST
  body plus `ledger` and `idempotencyKey`, so the page builds the REST body by dropping two fields.
  No "Copy as GraphQL" (decided, Q13): the curl serves integrators, who use REST; GraphQL is the
  console's own API.
- REST that remains: the system slice (`features/system/api.ts`: getInfo polled every 5 s by the
  shell's health indicator, getReconcilerStatus on the System page) stays on REST through
  `packages/api-client`. The health indicator must work even when GraphQL is disabled or broken.
- TanStack Query stays the only cache (ADR-0014). No Apollo or urql. Query keys do not change, so
  invalidation (`['ledger', name]` scope) keeps working; combined queries get keys inside the same
  scope.

### 17.2 Steps (phase 4, one commit each)

1. `packages/graphql/package.json` (`@tigerbeetle-platform/graphql`, `files: [schema.graphql]`)
   and the console devDependency on it, so Turborepo hashes the schema for `console#generate`.
2. `apps/console/codegen.ts`: schema `../../packages/graphql/schema.graphql`, documents
   `src/**/*.ts`, preset `client` with `documentMode: 'string'`, `enumsAsTypes: true`, scalars
   mapped to `string` (`Amount`, `SignedAmount`, `U128`, `U64`, `DateTime`), `number` (`Int64`) and
   `Record<string, string>` (`Metadata`); output `src/graphql/gen/` (committed, ignored by Oxfmt and
   Oxlint). The console's `generate` script becomes `graphql-codegen && tsr generate`.
3. `src/lib/graphql/client.ts`: a fetch-based client of about 80 lines. `request(document,
   variables, {signal})` POSTs `{query: document.toString(), variables}` to
   `${VITE_API_URL ?? '/api'}/graphql` with `Content-Type: application/json`, an
   `X-Request-Id` from `randomRequestId()` (from `packages/api-client`), the TanStack abort signal
   and a 15 s timeout. Any `errors` entry rejects the promise; data and errors are never mixed in
   the UI.
4. `src/lib/graphql/errors.ts`: `GraphQLProblemError` built from the first error's `extensions`
   (`code`, `reason`, `type`, `title`, `status`, `detail`, `requestId`, `retryAfter`, `posting`,
   `results`), with the same members as `ApiError`, plus a non-domain branch for gqlgen's parse
   and validation errors and for network failures.
5. Make `lib/problem.ts` (`toProblem`, `problemType`, `isNotFound`), `lib/idempotency.ts`
   (`keyIsSpent`), `lib/query-client.ts` (`shouldRetry`) and
   `features/requests/stored-problem.ts` accept either error through one small interface
   (`status`, `type`, `problem`, `requestId`, `retryAfter`). Behaviour is identical: a key is
   spent on any status below 500 except `request-in-flight` and `write-pending`; client errors are
   not retried. Existing tests extend to the GraphQL error.
6. Slices, one commit each, each keeping its query keys and tests: ledgers and search, settings,
   accounts and balances, transactions and the new-transaction flow (N+1 `getAccount` and
   `matchRule` calls become one query with nested fields), holds, requests, overview (one
   `ledger { stats assets transactions holds }` query feeding the existing loader). Each slice's
   `api.ts` declares its documents with the generated `graphql()` function next to its
   `queryOptions`.
7. `features/requests/stored-problem.ts` keeps rebuilding problems from the stored record
   (`response` is the `JSON` scalar, `responseStatus` and `problemType` are fields), now through
   the shared interface of step 5.
8. Retry-After: phase 2 already surfaced `retryAfter` on `ApiError` and the problem panel shows
   "retry in N s" for `request-in-flight` and `write-pending` (today the console never reads it,
   audit 5.3); step 4 carries it over from `extensions.retryAfter`.
9. Remove REST imports from the migrated slices; `packages/api-client` stays a dependency for the
   system slice and `randomRequestId`.
10. Compose: `deploy/compose/console.Dockerfile` copies `packages/graphql/package.json`; the
    console container mounts `packages/graphql`.

Tests: every slice keeps its unit tests (with a fake `fetch` for the GraphQL client); the
GraphQL client and error mapping get their own tests; the browser check of handoff fact 20
(agent-browser) walks the same pages as before the migration.

## 18. ADRs

ADR-0001 allows only `Accepted`, `Superseded by` and `Deprecated` and forbids changing an accepted
decision in place (`docs/adr/0001-record-architecture-decisions.md:74-77,96-100`). Decided (Q2):
a new short ADR-0015 amends ADR-0001's status rules, and every older ADR this work amends then
carries the new status, so a reader or an agent who opens it sees at once that parts of it no
longer apply.

ADR-0015, "Amended status for accepted ADRs", decides:

- A new status, `Accepted, amended by [ADR-NNNN](NNNN-slug.md)`. With several amending ADRs, the
  links follow in number order, separated by commas.
- The amending ADR names, in its context, the sections of the older ADR that no longer apply and
  what replaces them. The older ADR's text is not edited; only its `Status` line changes, exactly
  as for `Superseded by`.
- The amending ADR's commit edits the status lines of the ADRs it amends, in the same commit.
- ADR-0001 itself becomes `Accepted, amended by ADR-0015`, by the rule it introduces.
- Backfill, in the same commit (decided, R1 and R2): the amendments written before the status
  existed get it too. ADR-0011 replaced the Biome parts of ADR-0003 and ADR-0009
  (`docs/adr/0011-vite-plus-toolchain-for-typescript.md:17-18`); ADR-0012 replaced the lefthook
  part of ADR-0009 (`docs/adr/0012-vite-plus-git-hooks.md:17-18`); ADR-0013 narrowed ADR-0004's
  Idempotency-Key rule and added two sentinels to its four
  (`docs/adr/0013-ledger-api-implementation.md:68,87`), and changed what ADR-0007's idempotency
  record stores (`docs/adr/0013-ledger-api-implementation.md:38`).

| ADR  | Title                                                                                             | Relation                                                                                                                                                                                                                                 |
| ---- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0015 | Amended status for accepted ADRs                                                                  | Amends 0001 (status rules). Backfills 0003 and 0009 for ADR-0011, 0009 for ADR-0012, and 0004 and 0007 for ADR-0013.                                                                                                                     |
| 0016 | Justfile as the single entry point; Turborepo for TypeScript only                                 | Amends 0009 (task runner, shim, root scripts) and 0012 (commands in the `staged` block; mechanism unchanged). Answers 0009's rejection of a root Makefile (16.1).                                                                        |
| 0017 | Bounded-context modules, shared kernel, platform, transport composition, and their depguard rules | Amends 0003 (the ports-and-adapters layout and dependency rules); replaces the layout of `docs/architecture.md`.                                                                                                                         |
| 0018 | REST spec generated from Go with Huma v2                                                          | Supersedes 0008. Amends 0003 ("standard library only for HTTP"): Huma is the one web framework exception, with the Huma globals exception, the validation policy (domain only, Q4) and the oasdiff gate; answers 0008's rejection (7.6). |
| 0019 | REST client generated with orval                                                                  | Supersedes the client half of 0008 (with 0018). Amends 0003 (the ky choice); rejects hey-api for now (15.1).                                                                                                                             |
| 0020 | GraphQL schema generated from Go registrations; gqlgen executor                                   | New. Generator design, scalars, nullability, connections, resolver composition.                                                                                                                                                          |
| 0021 | Transports over one application layer: gRPC, AMQP RPC, NATS RPC                                   | New. Controllers per transport, broker RPC design, error model with the `google.rpc` details, idempotency carriage, config switches, readiness, exposure until authentication (Q14).                                                     |
| 0022 | Ledger events to Kafka through TigerBeetle CDC                                                    | Amends 0002 (the outbox is no longer the publication path for ledger events). Written after spike S3.                                                                                                                                    |
| 0023 | Console on GraphQL                                                                                | Amends 0014 (data access through GraphQL, key as input field, curl stays REST) and the "thin client of the OpenAPI contract" line of 0003.                                                                                               |
| 0024 | Transport-neutral write protocol in the bookkeeping module                                        | Amends 0013 (paths, stored outcome codes, Idempotency-Key per transport, reconciler as a bootstrap runner with a switch) and 0004 and 0007 (the Idempotency-Key becomes a field on GraphQL and gRPC).                                    |

Status lines at the end of phase 0:

| ADR  | Status                                                                |
| ---- | --------------------------------------------------------------------- |
| 0001 | Accepted, amended by ADR-0015                                         |
| 0002 | Accepted, amended by ADR-0022                                         |
| 0003 | Accepted, amended by ADR-0011, ADR-0017, ADR-0018, ADR-0019, ADR-0023 |
| 0004 | Accepted, amended by ADR-0013, ADR-0024                               |
| 0007 | Accepted, amended by ADR-0013, ADR-0024                               |
| 0008 | Superseded by ADR-0018 and ADR-0019                                   |
| 0009 | Accepted, amended by ADR-0011, ADR-0012, ADR-0016                     |
| 0012 | Accepted, amended by ADR-0016                                         |
| 0013 | Accepted, amended by ADR-0024                                         |
| 0014 | Accepted, amended by ADR-0023                                         |

In the files each `ADR-NNNN` is a link, as ADR-0001 requires.

No query-builder ADR (8.12). Each ADR follows ADR-0001's format (60 to 160 lines, ASCII, MADR
sections) and cites the TigerBeetle documentation it relies on (0022 cites
https://docs.tigerbeetle.com/operating/cdc/).

## 19. Delivery: phases and commits

Every commit is a Conventional Commit that leaves `just lint`, `just test` and `just build` green
and `just generate` diff-free (before phase 1, the `pnpm` equivalents). Every phase ends with a
review stop: the owner reviews the branch, `docs/handoff.md` records what was verified, and the
next phase starts only after approval. AGENTS.md changes in the commit that makes each sentence
true, never ahead of the code (conflict K12).

### Phase 0: decisions (documentation only)

Baseline (decided, Q1): `feat/console` at a82f2c2. 8f57d8d holds the tailwind-variants, devtools
and `utils.test.ts` work and tracks `skills-lock.json`; the owner then committed the shim deletion
(67af5db), the go.mod and go.sum update with `oapi-codegen/runtime` made direct (b89e9f7) and
`.vscode/settings.json` (a82f2c2). The working tree is clean apart from the untracked `_/`, which
stays untracked. Phase 0 touches only `docs/`.

1. `docs(adr): add 0015 amended status for accepted ADRs` (sets 0001's status and backfills 0003
   and 0009 for ADR-0011 and ADR-0012, 0004 and 0007 for ADR-0013, section 18)
2. `docs(plans): add the approved migration plan with its research and audit`
3. `docs(adr): add 0016 justfile as the single entry point` (sets 0009 and 0012)
4. `docs(adr): add 0017 bounded-context modules and dependency rules` (adds 0017 to 0003)
5. `docs(adr): add 0018 REST spec generated from code with huma` (0008 becomes
   `Superseded by ADR-0018`; adds 0018 to 0003)
6. `docs(adr): add 0019 generated REST client with orval` (0008 becomes
   `Superseded by ADR-0018 and ADR-0019`; adds 0019 to 0003)
7. `docs(adr): add 0020 GraphQL schema generated from Go registrations`
8. `docs(adr): add 0021 transports over one application layer`
9. Spike S3 (CDC), throwaway, results noted in the ADR; then
   `docs(adr): add 0022 ledger events to kafka through tigerbeetle cdc` (sets 0002)
10. `docs(adr): add 0023 console on GraphQL` (sets 0014; adds 0023 to 0003)
11. `docs(adr): add 0024 transport-neutral write protocol in the bookkeeping module` (sets 0013;
    adds 0024 to 0004 and 0007)
12. `docs: point architecture.md, AGENTS.md and handoff at the migration plan and ADRs`
    (pointers only, including AGENTS.md's ADR rules for the amended status; no command or layout
    text changes yet)

Review stop: the ADR set and the status lines of section 18.

### Phase 1: the justfile

Precondition met: go.mod, go.sum, `.vscode/settings.json` and the shim deletion are committed
(b89e9f7, a82f2c2, 67af5db). Until commit 4 lands, a staged Go file makes the pre-commit hook call
`pnpm --filter api lint`, which no longer exists; phase 0 stages no Go file.

1. `build(tools): pin just 1.59.0 with checksums in install-tools.sh`
2. `build: add the root justfile` (today's tasks: sqlc and oapi-codegen, `pnpm turbo run generate`,
   lint, format, typecheck, test, test-integration, build, dev, vulncheck, compose recipes,
   tools-install, versions-check)
3. `build(console): add a typecheck script` (`tsc --noEmit`, as ADR-0011 intended; audit X10)
4. `build: remove the api package shim from turbo.json, the staged hook and the console image`
   (turbo `api#*` tasks; `vite.config.ts:232` calls just; `deploy/compose/console.Dockerfile:11`
   loses its COPY; the file itself was already deleted in 67af5db, lead point 10)
5. `build: replace the root package.json scripts with just recipes` (keeps `prepare`)
6. `ci: run every job through just; run the console unit tests` (lead point 12)
7. `docs: commands point at just` (AGENTS.md commands and workflow lines, CONTRIBUTING.md:22-49,
   README.md, apps/console/README.md, `.editorconfig`)
8. `build: pin the caret ranges` (K29, decided Q1): the three root devDependencies
   (`package.json:27-29`) and the five TanStack devtools packages
   (`apps/console/package.json:42-46`) get exact versions, the ones in `pnpm-lock.yaml`
9. `docs(handoff): record phase 1`

Review stop checks: `just --list`; no `^` or `~` left in any `package.json`; `just generate-check`; `just lint test build`; the pre-commit
hook on a staged Go file and on the spec; the console image builds (`just compose-up`).

### Phase 2: REST spec from code with Huma, in the current layout

Spikes first, throwaway: S1 (Huma) and S2 (orval), Appendix.

1. `build(tools): pin oasdiff 1.34.1`
2. `feat(api): add huma 2.39.1 with the rest api builder, problem model and wire scalars`
   (`internal/platform/restapi`, `problem`, `scalar`, `requestid`; unit tests; nothing mounted)
3. `refactor(api): port the operations endpoints to huma handlers` (new handlers in
   `internal/adapters/httpapi/v1`, tested with httptest on their own mux, not yet mounted;
   the existing httpapi depguard rule already covers the subfolder)
4. `refactor(api): port the ledgers endpoints to huma handlers`
5. `refactor(api): port the accounts endpoints to huma handlers`
6. `refactor(api): port the transactions endpoints to huma handlers`
7. `refactor(api): port the holds endpoints to huma handlers`
8. `refactor(api): port the settings endpoints to huma handlers`
9. `feat(api): add cmd/openapi` (writes to a path given by flag; a test runs it twice and compares;
   builds with `CGO_ENABLED=0`)
10. `refactor(api): serve REST through huma and generate the spec from code` (mounts the Huma
    handlers, deletes `internal/adapters/httpapi/gen`, the strict server, `oapi-codegen.yaml`, the
    tool directive and the runtime; `packages/openapi/openapi.yaml` is now written by
    `just generate`; the commit body carries the oasdiff report of 7.5, also saved as
    `docs/plans/ddd-transports/oasdiff-phase2.md`)
11. `feat(openapi): declare the X-Request-Id and Retry-After headers` (additive, separate from the
    parity commit, decided Q3; oasdiff INFO lines in the body)
12. `test(api): replay test pinning status, headers and problem bodies`
13. `ci: oasdiff breaking gate against the base branch`
14. `build(api-client): generate per-tag modules with orval 8.44.0` (after 11, so `Retry-After` is
    typed in the generated client, K34)
15. `feat(api-client): hand-written config, errors and headers layer with unit tests`
16. `refactor(console): use the generated client; show Retry-After in the problem panel`
17. `build(openapi): remove openapi-typescript and the TypeScript 5.9.3 pin` (and the Renovate
    rule `renovate.json:27-31`)
18. `docs: adding an endpoint starts from the huma structs` (AGENTS.md "Adding an endpoint",
    CONTRIBUTING.md:63-80, packages/openapi/README.md, docs/architecture.md), then
    `docs(handoff): record phase 2`

Review stop checks: oasdiff report with every difference explained; the end-to-end integration
test unchanged and green; the replay test; Redocly lint; the console in a browser on every page.

### Phase 3: modules, REST only, no behaviour change

1. `refactor(api): move the shared kernel to internal/kernel/ledger` (mechanical import change)
2. `refactor(api): platform packages for postgres, tigerbeetle, readiness` (moved from adapters
   and app, same code)
3. `refactor(api): bootstrap builds the app and runs every server from one place` (errgroup,
   Runner, the reconciler as a runner; `cmd/api/main.go` shrinks; the end-to-end test builds
   through `bootstrap.Build` with the TigerBeetle fault-injection option, audit X17)
4. `refactor(api): health module with an interfaces layer only`
5. `refactor(api): ledgers module` (domain ports and records, commands and queries, its sqlc block
   and queries, `interfaces/dto` and `rest/v1`)
6. `refactor(api): bookkeeping domain and infrastructure`
7. `refactor(api): bookkeeping application: write flow, commands, queries, reconciler`
8. `refactor(api): bookkeeping REST interface`
9. `refactor(api): cross-module ports wired in bootstrap; remove internal/app, internal/adapters and internal/domain`
10. `test(api): use-case unit tests per module with hand-written fakes` (today's `app_test.go` and
    `writes_test.go` split by module; transport-free)
11. `build(lint): depguard rules for modules, kernel, platform and transports` and
    `test(lint): depguard probe fixtures and just lint-depguard-probe`
12. `ci: run the depguard probe`
13. `docs: layout and dependency rules` (AGENTS.md layout, dependency rules, the six sentinels,
    the id.go path; docs/architecture.md), then `docs(handoff): record phase 3`

Review stop checks: `packages/openapi/openapi.yaml` byte-identical to the end of phase 2 (no
diff at all); end-to-end test green; probe fixtures all caught; `go list -deps` shows no module
importing another.

### Phase 4: GraphQL generator, gqlgen server, console migration

Spike first: S6 (gqlgen), Appendix.

1. `feat(api): graphqlschema generator with golden tests`
2. `feat(api): cmd/graphqlschema and packages/graphql`
3. `build(api): gqlgen 0.17.98 as a go tool; executor in internal/transport/graphql/exec`
4. `feat(api): serve POST /graphql behind GRAPHQL_ENABLED with the problem error presenter`
5. `feat(api): health and ledgers GraphQL queries and mutations` (with resolver tests on fakes)
6. `feat(api): bookkeeping GraphQL queries, mutations and Ledger fields`
7. `test(api): GraphQL integration test through bootstrap` (including the idempotency replay
   across REST and GraphQL with one key)
8. Console steps 1 to 10 of 17.2, one commit each (`build(console): ...`, `feat(console): ...`,
   `refactor(console): <slice> on GraphQL`)
9. `docs: GraphQL in AGENTS.md (adding a console page, adding an endpoint), console.md`, then
   `docs(handoff): record phase 4`

Review stop checks: schema drift check; golden tests; every console page in a browser; a declined
transaction, an in-flight replay and a renewed key behave as on REST.

### Phase 5: gRPC

1. `build(tools): pin buf 1.75.0, protoc-gen-go 1.36.12 and protoc-gen-go-grpc 1.6.2`
2. `feat(proto): packages/proto with buf lint and breaking configuration`
3. `build(api): buf generate into internal/platform/grpcapi/platformv1`
4. `feat(api): grpc server with request id, logging, recovery and problem interceptors` (error
   details as `google.rpc.ErrorInfo`, `RetryInfo` and `BadRequest`, 9.2; the reason derived by one
   function and checked against AIP-193 by a test over every problem type; the commit body shows
   `go mod graph` proving `google.golang.org/genproto/googleapis/rpc` was already required by
   grpc-go, decided Q12)
5. `feat(api): ledgers gRPC services`
6. `feat(api): bookkeeping gRPC services`
7. `feat(api): grpc health and info services`
8. `test(api): gRPC adapter tests over bufconn and an integration test`
9. `ci: buf lint and buf breaking against the base branch`
10. `docs: gRPC`, then `docs(handoff): record phase 5`

Review stop checks: grpcurl against the compose stack with gRPC enabled; `ErrorInfo` and
`RetryInfo` details for a decline and for an in-flight replay; graceful stop under load.

### Phase 6: AMQP RPC and NATS RPC

Spike first: S5 (RPC), Appendix.

1. `build(compose): brokers profile with rabbitmq 4.4.0 and nats 2.15.1 pinned by digest`
   (`depends_on` with `required: false`; devstack learns the images)
2. `feat(api): rpc envelope, router and worker pool`
3. `feat(api): amqp connection supervisor and amqp rpc server`
4. `feat(api): nats rpc server on nats micro with a queue group`
5. `feat(api): module rpc controllers, method v1.<operationId>`
6. `feat(api): readiness probes and switches for rabbitmq and nats` (adds the optional
   `Dependencies` members and the `degraded` readiness status, 13.2, decided Q7; the oasdiff INFO
   lines and the enum warning are explained in the commit body; a broker outage keeps `/readyz` at 200)
7. `test(api): testcontainers integration tests for amqp and nats rpc` (two replicas: each request
   runs once; concurrency bound; deadline; redelivery of a write replays)
8. `test(e2e): full stack tests against compose with brokers` (`just test-e2e`)
9. `docs: broker RPC`, then `docs(handoff): record phase 6`

### Phase 7: Kafka

Spike first: S4 (Kafka); S3 re-run if the TigerBeetle image changed.

1. `build(compose): kafka 4.3.2 and the tigerbeetle cdc job in the brokers profile` (fixed IPs,
   RabbitMQ definitions file)
2. `feat(api): kafka producer and ping on franz-go 1.22.2`
3. `feat(api): bookkeeping relays tigerbeetle cdc events to kafka`
4. `feat(api): kafka readiness and switches`
5. `test(api): cdc to kafka integration test` (gaps never, duplicates only, string amounts)
6. `docs: ledger event format`, then `docs(handoff): record phase 7`

## 20. Conflicts with AGENTS.md, the ADRs and the code

Each row names the requirement or decision, the text or code it conflicts with, and the
recommended resolution. None is resolved silently: the ADR in the last column records it.

| #   | Conflict                                                                                                                                                                                                                                             | Where                                                                                                                                                                                         | Recommended resolution                                                                                                                                                                                                                                                                     |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| K1  | Huma is a web framework; the rules say "No web framework" and "standard library only for HTTP".                                                                                                                                                      | `AGENTS.md:44-45`; `docs/adr/0003-go-api-typescript-react-console.md:33,48-51`; `docs/adr/0008-openapi-first-generated-server-and-client.md:25,128-129`; `README.md:55`                       | ADR-0018 records Huma on `humago` as the single exception, confined by depguard to `platform/restapi`, `platform/scalar`, module `interfaces/rest`, `transport/rest` and `cmd/openapi`. Routing stays `net/http.ServeMux`; middleware stays stdlib. AGENTS.md amended in phase 2.          |
| K2  | ADR-0008 rejected code-first, naming Huma.                                                                                                                                                                                                           | `docs/adr/0008-...md:34-40`                                                                                                                                                                   | ADR-0018 supersedes 0008 and answers the objection (7.6): committed spec, drift check, oasdiff breaking gate, Redocly.                                                                                                                                                                     |
| K3  | "The spec is the contract; edit the spec first" becomes "the spec is generated".                                                                                                                                                                     | `AGENTS.md:25,86-87`; `CONTRIBUTING.md:63-80`; `packages/openapi/README.md:3-6`; spec `info.description` (`packages/openapi/openapi.yaml:6-8`); `docs/architecture.md` (contract paragraph)   | The committed spec stays the published contract for third parties; it is now an output. New workflow in phase 2. ADR-0004's "expressible in OpenAPI 3.1" still holds.                                                                                                                      |
| K4  | New layout and dependency rules replace `domain`, `app`, `adapters`.                                                                                                                                                                                 | `AGENTS.md:17-24,37-39`; `docs/architecture.md:53-75`; `docs/adr/0003-...md:57-72`; `apps/api/.golangci.yml:28-84`                                                                            | ADR-0017; AGENTS.md and architecture.md rewritten in phase 3; depguard rewritten with the probe of 14.2.                                                                                                                                                                                   |
| K5  | The owner's "infrastructure to domain" forces driven ports and their record types into the module domain; AGENTS.md says `internal/app/ports.go` declares them, and research suggested keeping ports in application and naming the layer `adapters`. | `AGENTS.md:20`; research (a) conflicts table; `internal/app/ports.go`, `internal/app/types.go`                                                                                                | The owner's binding requirement wins: driven ports and records go to `modules/<m>/domain`, cross-module ports to `application/ports.go`, the layer is named `infrastructure`. ADR-0017.                                                                                                    |
| K6  | "No globals, no `init()`" and "log/slog only" meet Huma's package settings, protobuf's generated `init()` registration and grpc-go's global logger.                                                                                                  | `AGENTS.md:18,44`; research finding 6                                                                                                                                                         | Huma: one `sync.Once` in `restapi.NewAPI` (7.1). Generated code is exempt, as it already is from lint (`apps/api/.golangci.yml:104,120`). grpc-go's logger is routed to slog once, with the same `sync.Once` pattern, in `platform/grpcserver`. All three listed in ADR-0018 and ADR-0021. |
| K7  | "Use the pnpm root commands, which go through Turborepo"; `pnpm lint/test/build/generate` everywhere.                                                                                                                                                | `AGENTS.md:55,62-64,88-89`; `docs/adr/0009-...md:21-22,33-42`; `CONTRIBUTING.md:22-49`; `README.md:26-47`; `.github/workflows/ci.yml:74`                                                      | ADR-0016. TypeScript recipes still go through Turborepo, so ADR-0011's "Turborepo stays the task runner" (`docs/adr/0011-...md:66`) remains true for TypeScript. Docs updated in phase 1.                                                                                                  |
| K8  | The deleted `apps/api/package.json` is still referenced.                                                                                                                                                                                             | `turbo.json:50-73`; `vite.config.ts:232`; `deploy/compose/console.Dockerfile:11`; `CONTRIBUTING.md:49-50`; `docs/adr/0009-...md:51-54`; `docs/adr/0012-...md:55`; `docs/adr/0008-...md:93-95` | Phase 1 commit 4 (lead point 10). Accepted ADR texts stay as history; ADR-0016 names what replaced the shim.                                                                                                                                                                               |
| K9  | Requirement H replaces the root `package.json` scripts, but `prepare` installs the git hooks (ADR-0012).                                                                                                                                             | `package.json:22`; `docs/adr/0012-...md` (Decision Outcome)                                                                                                                                   | Keep `prepare`; remove every other script.                                                                                                                                                                                                                                                 |
| K10 | The `staged` block commands change.                                                                                                                                                                                                                  | `docs/adr/0012-...md:55-56`; `vite.config.ts:230-234`                                                                                                                                         | Same mechanism, new commands (16.5); recorded in ADR-0016.                                                                                                                                                                                                                                 |
| K11 | The owner asks to "amend" ADR-0009, 0013 and 0014; ADR-0001 knows only Accepted, Superseded and Deprecated and forbids changing an accepted decision.                                                                                                | `docs/adr/0001-...md:74-77,96-100`                                                                                                                                                            | Decided (Q2): ADR-0015 amends ADR-0001 with the status `Accepted, amended by ADR-NNNN`; the amended ADRs carry it from phase 0 (section 18).                                                                                                                                               |
| K12 | ADR-0001 wants AGENTS.md updated in the ADR's pull request, and the owner orders ADRs, then AGENTS.md, then phases; AGENTS.md describing commands and paths that do not exist yet would mislead every agent during phase 1.                          | `docs/adr/0001-...md:96-100`                                                                                                                                                                  | Phase 0 adds pointers to the plan and ADRs only; each AGENTS.md statement changes in the commit that makes it true.                                                                                                                                                                        |
| K13 | ADR-0002 promises an outbox for publication; Kafka events use TigerBeetle CDC instead.                                                                                                                                                               | `docs/adr/0002-...md:63-66,81-91`; `docs/architecture.md:36`                                                                                                                                  | ADR-0022 amends the publication path for ledger events; the outbox remains the planned path for Postgres-born events, not built because nothing consumes them yet (decided, Q8).                                                                                                           |
| K14 | "The console is a thin client of the OpenAPI contract, through packages/api-client."                                                                                                                                                                 | `docs/adr/0003-...md:24-25`; `docs/adr/0014-console-architecture.md:8-9`; `docs/console.md:9,29`; `AGENTS.md:9-10,30`                                                                         | ADR-0023: the console still talks only to the platform API, now mostly through GraphQL; the system slice stays on the REST client.                                                                                                                                                         |
| K15 | "Cursor pagination with `cursor` and `next` on every list endpoint."                                                                                                                                                                                 | `docs/adr/0004-ledger-api-surface.md:78-80`                                                                                                                                                   | Same opaque cursor carried as `after`/`endCursor` (GraphQL) and `page_token` (gRPC); recorded in ADR-0020 and ADR-0021.                                                                                                                                                                    |
| K16 | Amounts are integer strings; GraphQL `Int` is 32-bit.                                                                                                                                                                                                | `docs/adr/0004-...md:86-93`; `AGENTS.md:68-69`                                                                                                                                                | String scalars for money, IDs and TigerBeetle timestamps; `Int64` (JSON number) for counts only; the generator rejects `uint64` and floats (8.3, 8.4).                                                                                                                                     |
| K17 | "Every write requires an Idempotency-Key header", narrowed to five writes, and carried as a field on GraphQL and gRPC.                                                                                                                               | `docs/adr/0004-...md:119`; `docs/adr/0007-...md:79`; `docs/adr/0013-...md:66-71`                                                                                                              | ADR-0024 states the five-write scope and the per-transport carriage (section 10).                                                                                                                                                                                                          |
| K18 | ADR-0007 says a replay returns the stored status and body; the code re-reads the resource.                                                                                                                                                           | `docs/adr/0007-...md:87`; `internal/app/transactions.go:222-224`                                                                                                                              | Behaviour unchanged on every transport; ADR-0024 records what the code does (audit X6).                                                                                                                                                                                                    |
| K19 | The use-case layer stores HTTP status numbers.                                                                                                                                                                                                       | `internal/app/writes.go:24-32,61-74`; `docs/adr/0013-...md:44-45`                                                                                                                             | Kept as stored values (`responseStatus` is part of REST); the constants move to `bookkeeping/domain/outcome.go`, documented as numbered like HTTP statuses; non-REST transports map from the problem kind. ADR-0024.                                                                       |
| K20 | "/readyz probes every enabled broker" versus "REST behaviour does not change".                                                                                                                                                                       | requirement C; `packages/openapi/openapi.yaml:1011-1018`                                                                                                                                      | Optional `rabbitmq`, `nats`, `kafka` members, absent unless enabled, and a `degraded` status when one is down; 503 stays reserved for Postgres and TigerBeetle (decided, Q7). Added in phases 6 and 7 with explained oasdiff lines (13.2).                                                 |
| K21 | "One controller folder per transport" versus AMQP and NATS RPC sharing one envelope.                                                                                                                                                                 | requirement A; research (b)                                                                                                                                                                   | Decided (Q15): one shared `interfaces/rpc/v1` per module serving both brokers; broker specifics in `platform/amqprpc` and `platform/natsrpc`. ADR-0021 records the deviation.                                                                                                              |
| K22 | "Versioned by folder (v1, v2)" versus GraphQL.                                                                                                                                                                                                       | requirement A                                                                                                                                                                                 | `interfaces/graphql` is unversioned; the schema evolves by deprecation. REST, gRPC and RPC folders are versioned.                                                                                                                                                                          |
| K23 | Requirement D says lazy account creation and linked chains put accounts and transfers into one TigerBeetle write. In the code they are two requests: `create_accounts`, then `create_transfers`.                                                     | `internal/app/writes.go:398` and `writes.go:161-175` (V1)                                                                                                                                     | Factual correction only: both run under one idempotency record and one reconciler path, and the account step heals on retry (ADR-0006), so the conclusion, one module, stands.                                                                                                             |
| K24 | AGENTS.md names four sentinel errors; the code has six.                                                                                                                                                                                              | `AGENTS.md:47-48`; `internal/domain/ledger/errors.go:10-26`                                                                                                                                   | Fix AGENTS.md in phase 3.                                                                                                                                                                                                                                                                  |
| K25 | The `id.go` path moves.                                                                                                                                                                                                                              | `AGENTS.md:76-77`; ADR-0002 and ADR-0006 cite `adapters/tigerbeetle/id.go`                                                                                                                    | New path `modules/bookkeeping/infrastructure/tigerbeetle/id.go`; AGENTS.md in phase 3; ADR path fixes are allowed by ADR-0001 without a new ADR.                                                                                                                                           |
| K26 | Huma's validation statuses and content-type checks versus unchanged REST behaviour.                                                                                                                                                                  | V6; `internal/app/cursor.go:20-30`; `internal/app/accounts.go:249-251`; `httpapi/gen/api.gen.go:5497`                                                                                         | Validation skipped per operation, for good (decided, Q4); constraints documented but not enforced; Huma's own errors mapped to 400; the JSON content-type middleware keeps today's behaviour (decided, Q6). 7.3, 7.8.                                                                      |
| K27 | AGENTS.md says `/readyz` probes Postgres and TigerBeetle.                                                                                                                                                                                            | `AGENTS.md:51-52`                                                                                                                                                                             | Updated in phase 6 to "and every enabled broker; 503 only for Postgres or TigerBeetle" (Q7).                                                                                                                                                                                               |
| K28 | Authentication is out of scope, and four new unauthenticated entry points appear.                                                                                                                                                                    | `AGENTS.md:93`; audit X18                                                                                                                                                                     | Decided (Q14): gRPC, AMQP RPC, NATS RPC and Kafka are off by default and development-only until authentication is decided; GraphQL has REST's status. ADR-0021 states it (13.1).                                                                                                           |
| K29 | "Pin everything" is already broken by caret ranges.                                                                                                                                                                                                  | `AGENTS.md:90-91`; `package.json:27-29`; `apps/console/package.json:42-46` (devtools)                                                                                                         | Pinned in phase 1 commit 8, root and console (decided, Q1).                                                                                                                                                                                                                                |
| K30 | ADR-0009 and the handoff say oapi-codegen is the only `go tool`.                                                                                                                                                                                     | `docs/adr/0009-...md:64-67`; `docs/handoff.md` fact 10                                                                                                                                        | oapi-codegen leaves in phase 2, gqlgen becomes the only `go tool` in phase 4; recorded in ADR-0016 and ADR-0020.                                                                                                                                                                           |
| K31 | `packages/openapi` pins TypeScript 5.9.3 for openapi-typescript; Renovate keeps it below 6.                                                                                                                                                          | `docs/adr/0008-...md:76-77`; `docs/handoff.md` fact 8; `renovate.json:27-31`                                                                                                                  | Removed with openapi-typescript in phase 2; no generator left needs the compiler API (orval and graphql-codegen do not).                                                                                                                                                                   |
| K32 | Generated Go for protos and GraphQL must build with the `apps/api` Docker context and no `go.work`.                                                                                                                                                  | `apps/api/Dockerfile:12-15`; `docs/adr/0009-...md:60-62`; audit X14                                                                                                                           | All generated Go lives inside `apps/api`; sources outside are read only by `just generate`. gqlgen's handling of an outside schema is spike S6. No `go.work`.                                                                                                                              |
| K33 | The spec commands must not link the cgo TigerBeetle client.                                                                                                                                                                                          | audit X16; `apps/api/Dockerfile:10`                                                                                                                                                           | depguard `spec-commands` rule and `CGO_ENABLED=0` in `just generate-contracts`.                                                                                                                                                                                                            |
| K34 | The console promises "retry after Retry-After" but never reads it; it has no typecheck script; CI never runs its unit tests.                                                                                                                         | `docs/console.md:148`; `apps/console/package.json` scripts; `.github/workflows/ci.yml` (`web` job)                                                                                            | Typecheck and CI tests in phase 1; Retry-After in phase 2 (REST) and phase 4 (GraphQL). Lead point 12.                                                                                                                                                                                     |
| K35 | The reconciler runs in every API replica.                                                                                                                                                                                                            | `docs/adr/0013-...md:53-56,106-107`                                                                                                                                                           | Unchanged; it becomes a bootstrap runner with `RECONCILER_ENABLED`, in the API process; a `cmd/worker` only once a production deployment exists (decided, Q10).                                                                                                                            |

## 21. The lead's review points, answered

| #   | Point                                                                                       | Answer                                                                                                       |
| --- | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 1   | ADR-0008 rejected code-first because reviewers rarely read a generated spec.                | 7.6; ADR-0018.                                                                                               |
| 2   | ADR-0009 rejected a root Makefile.                                                          | 16.1; ADR-0016.                                                                                              |
| 3   | Today's depguard globs match nothing under `internal/modules`; prove the rewrite fires.     | 14.1 and the probe of 14.2, run in CI.                                                                       |
| 4   | Huma answers 422 on validation failure; REST must keep 400.                                 | 7.3: validation stays in the domain for good (Q4); Huma's own errors map to 400.                             |
| 5   | Huma configures behaviour through package globals; avoid nullable slices.                   | 7.1: one `sync.Once` in `restapi.NewAPI`; `DefaultArrayNullable = false`; K6.                                |
| 6   | Spec features Huma may not reproduce (audit 3.3).                                           | 7.4, item by item, with a fallback and an oasdiff line for each.                                             |
| 7   | hey-api needs the TypeScript compiler API; decide hey-api with a TS 6 alias or orval.       | 15.1: orval 8.44.0; hey-api with the alias is the fallback if spike S2 fails.                                |
| 8   | CDC facts: AMQP only, no outbox, IP-only host, numbers in the example; schedule the spikes. | 12.1 to 12.5; spikes S3 (phase 0) and S4 (phase 7).                                                          |
| 9   | evrone's AMQP and NATS RPC is unsafe as is.                                                 | 11.2: competing consumers and queue groups, bounded concurrency, typed envelope, timeout mapping, reconnect. |
| 10  | The deleted shim is still referenced in four places.                                        | Phase 1 commit 4; K8.                                                                                        |
| 11  | pnpm's minimum release age blocks new packages.                                             | 5.3: orval 8.44.0, not 8.45.0; the rule for every pin.                                                       |
| 12  | Retry-After unused, no console typecheck, console tests not in CI.                          | Phase 1 (typecheck, CI tests), phases 2 and 4 (Retry-After); K34.                                            |

## 22. Owner decisions carried from audit 8.4

| #   | Question                                                          | Plan's answer                                                                                                                                                                                     |
| --- | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Is the dirty working tree the baseline, and who commits it?       | Decided (Q1): baseline a82f2c2; the owner committed the shim deletion, go.mod, go.sum and `.vscode/settings.json` (67af5db, b89e9f7, a82f2c2); phase 1 commit 4 removes the remaining references. |
| 2   | just as a wrapper over Turborepo or a replacement?                | Wrapper for TypeScript, direct for Go (16.1).                                                                                                                                                     |
| 3   | Does the generated OpenAPI file stay committed?                   | Yes, with a drift check and an oasdiff gate (7.5, 7.6).                                                                                                                                           |
| 4   | Where do the write protocol and `ensureAccounts` live?            | `modules/bookkeeping/application` (`writeflow.go`, `ensure_accounts.go`); ports in `bookkeeping/domain`; rules reached through `AccountPolicies` (4.5).                                           |
| 5   | One Postgres schema or one per module?                            | One schema, one goose sequence, cross-module foreign keys kept, one sqlc block per module (4.7).                                                                                                  |
| 6   | Reconciler in the API process or a worker?                        | In the process, as a bootstrap runner, with `RECONCILER_ENABLED` (11.3; decided, Q10).                                                                                                            |
| 7   | How do the transports carry key, request ID, cursors and reasons? | Sections 9 and 10, 8.6, 11.1.                                                                                                                                                                     |
| 8   | Does REST stay for operations, the probe, scripts and curl?       | Yes: REST is unchanged and stays the third-party API; the console keeps REST for the system slice and Copy as curl (17.1).                                                                        |
| 9   | Which generators need which TypeScript?                           | None after phase 2: orval and graphql-codegen do not use the compiler API; the 5.9.3 pin goes (K31).                                                                                              |
| 10  | Is the ADR-0002 outbox built for Kafka?                           | No. Ledger events come from TigerBeetle CDC (12.2); the outbox waits for a consumer of Postgres-born events (decided, Q8).                                                                        |

## 23. Owner decisions, and questions they raised

### 23.1 Decisions (answered 2026-10-25)

| #   | Question                                             | Decision                                                                                                                                                                                                                                                            | Applied in              |
| --- | ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| Q1  | Baseline and caret ranges                            | Baseline 8f57d8d (tailwind-variants, devtools, `utils.test.ts`, `skills-lock.json` tracked), then the owner's 67af5db (shim deletion), b89e9f7 (go.mod, go.sum) and a82f2c2 (`.vscode`); `_/` stays untracked. Caret ranges pinned in phase 1.                      | 19 (phases 0, 1), K29   |
| Q2  | How to amend an ADR                                  | A new short ADR-0015 amends ADR-0001 to allow `Accepted, amended by ADR-NNNN`; that status goes on 0002, 0003, 0009, 0012, 0013 and 0014. ADRs renumbered.                                                                                                          | 18, 19 (phase 0), K11   |
| Q3  | Declare `X-Request-Id` and `Retry-After` in the spec | Yes, as a separate additive commit after the parity commit, so the generated client types `Retry-After` (K34).                                                                                                                                                      | 7.5, 19 (phase 2)       |
| Q4  | Huma edge validation later                           | No, not later either. Validation stays in the domain because five transports share it. The spec may document constraints without enforcing them; rejection instead of clamping would be a domain change for every transport.                                        | 7.1, 7.3, 7.4, K26      |
| Q5  | Serve the spec from the API                          | No. The committed file is the contract.                                                                                                                                                                                                                             | 7.7                     |
| Q6  | Non-JSON `Content-Type` on a JSON body               | Keep today's behaviour with the middleware. Revisit only with a future strictness change, in a new API minor version.                                                                                                                                               | 7.8, K26                |
| Q7  | A broker that is down and `/readyz`                  | Report only. 503 only when Postgres or TigerBeetle is down; brokers shown down in the body with an overall `degraded` status.                                                                                                                                       | 13.2, 19 (phase 6), K20 |
| Q8  | Publish Postgres-born events                         | No. Nothing consumes them; the ADR-0002 outbox stays the planned path for when a consumer needs them.                                                                                                                                                               | 12.2, K13               |
| Q9  | Inbound Kafka commands                               | Confirmed: none.                                                                                                                                                                                                                                                    | 12.2                    |
| Q10 | Reconciler and relay placement                       | Both in the API process behind `RECONCILER_ENABLED` and `KAFKA_EVENTS_ENABLED`; `cmd/worker` only once a production deployment exists (out of scope).                                                                                                               | 13.1, K35               |
| Q11 | Playground, introspection, gRPC reflection           | Playground off, introspection on, gRPC reflection on when gRPC is enabled.                                                                                                                                                                                          | 8.11, 11.1              |
| Q12 | gRPC error details                                   | The standard `google.rpc.ErrorInfo` and `RetryInfo`, plus `BadRequest` for field errors; Retry-After in `RetryInfo`. `ErrorInfo` as amended by R3. No new module; verified with `go mod graph` in phase 5.                                                          | 9.1, 9.2, 19 (phase 5)  |
| Q13 | "Copy as GraphQL"                                    | No. The curl serves integrators, who use REST; GraphQL is the console's API.                                                                                                                                                                                        | 17.1                    |
| Q14 | Unauthenticated new transports                       | gRPC, AMQP RPC, NATS RPC and Kafka off by default and development-only until authentication is decided; GraphQL has REST's status. Stated in the transports ADR (0021).                                                                                             | 13.1, 18, K28           |
| Q15 | RPC controller folders                               | One shared `interfaces/rpc/v1`, broker specifics in `platform/amqprpc` and `platform/natsrpc`.                                                                                                                                                                      | 11.2, K21               |
| Q16 | Module names and kernel path                         | `ledgers`, `bookkeeping`, `health`; `internal/kernel/ledger`. "Bookkeeping" records transactions in accounts; "accounting" would suggest reporting it does not do. Keeping `internal/domain/ledger` next to `modules/*/domain` would make "domain" mean two things. | 3, 4                    |

### 23.2 Questions raised by the answers, decided

| #   | Question                      | Decision                                                                                                                                                                                                                                                                                                                | Applied in   |
| --- | ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| R1  | Statuses beyond the six named | Yes. ADR-0001 becomes `Accepted, amended by ADR-0015` under its own new rule; ADR-0003 is amended by ADR-0011, ADR-0009 by ADR-0011 and ADR-0012, in addition to the ADRs this work amends.                                                                                                                             | 18, phase 0  |
| R2  | ADR-0004 and ADR-0007         | Yes: `Accepted, amended by ADR-0013, ADR-0024` on both.                                                                                                                                                                                                                                                                 | 18, phase 0  |
| R3  | `ErrorInfo.reason` format     | Follow AIP-193 (this replaces the Q12 wording): `reason` in UPPER_SNAKE_CASE derived from the problem type slug by one function; `domain` one fixed service identifier recorded in ADR-0021; `metadata["type"]` the problem type URI. A test walks every problem type against AIP-193's pattern and 63-character limit. | 9.2, phase 5 |

## Appendix: spikes

Spikes are throwaway code in a scratch directory, never committed. Each ends with a short note in
`docs/handoff.md` and, where it changes a decision, an update to the ADR before code starts.

| #              | When                     | Question                                                                                                                                                                                                                                                                                                                                                                                                             | Pass condition                                                                                                   | If it fails                                                                                    |
| -------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| S1 Huma        | Start of phase 2         | Spec from `humago` on a bare `ServeMux`: byte-stable YAML; problem+json with custom members and no `errors`; `Idempotency-Key` header; `DefaultArrayNullable = false`; `CreateHooks` cleared; docs page off; `SkipValidateParams` and `SkipValidateBody`; `SchemaProvider` returning `$ref`, `allOf` and `propertyNames` (through `Extensions`); `security: []`; `additionalProperties`; 415 on a form content type. | Each item behaves as 7.1 to 7.8 assume.                                                                          | The fallbacks of 7.4 and 7.8; oasdiff lines accepted explicitly.                               |
| S2 REST client | Start of phase 2         | orval 8.44.0, `tags-split`, `fetch`, custom mutator, against the S1 spec: OpenAPI 3.1, non-null arrays, the 14 scalar names, `exactOptionalPropertyTypes`, TypeScript 7.0.2 type check. Compared with hey-api 0.99.0 under the `@typescript/typescript6` alias.                                                                                                                                                      | orval output type-checks and keeps the names the console imports.                                                | hey-api with the alias (ADR-0019 fallback).                                                    |
| S3 CDC         | Phase 0, before ADR-0022 | `tigerbeetle amqp` from the 0.17.9 image against RabbitMQ 4.4.0 with fixed IPs; one message of each type; encoding of `id` and `amount`; kill-and-restart replay time.                                                                                                                                                                                                                                               | Messages arrive in order; values decode exactly; replay resumes after the last acknowledged timestamp.           | Relay on the experimental `GetChangeEvents` with a Kafka checkpoint (12.5), owner asked again. |
| S4 Kafka       | Start of phase 7         | `apache/kafka:4.3.2` as a generic testcontainer with a fixed port and advertised listener; start time against `apache/kafka-native:4.3.2` and the Redpanda module.                                                                                                                                                                                                                                                   | Reliable start under `go test -race` in CI.                                                                      | Redpanda v26.2.6 module for tests only; compose keeps Apache Kafka.                            |
| S5 RPC         | Start of phase 6         | One durable request queue plus `amq.rabbitmq.reply-to`; NATS `micro` with a queue group and asynchronous `Respond`; deadlines; the envelope; two replicas.                                                                                                                                                                                                                                                           | Each request runs once; concurrency bounded; timeouts mapped.                                                    | Adjust the platform packages before controllers are written.                                   |
| S6 gqlgen      | Start of phase 4         | gqlgen v0.17.98 with a schema outside `apps/api` (inlined or embedded?), autobind to named scalar types and to promoted fields of embedded structs, no `resolver:` block, a root built by embedding.                                                                                                                                                                                                                 | Generated code builds with the `apps/api` Docker context; the embedding root satisfies the generated interfaces. | Write a schema copy next to the executor; list fields instead of embedding (8.8).              |
