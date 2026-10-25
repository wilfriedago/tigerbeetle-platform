# 17. Bounded-context modules, shared kernel, platform and their dependency rules

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

ADR-0003 laid the API out as ports and adapters: one domain package (`internal/domain/ledger`),
one use-case package (`internal/app`, eight use cases and `ports.go`), three adapters
(`httpapi`, `postgres`, `tigerbeetle`) and `internal/platform`, with depguard enforcing the
direction. That layout has one adapter per technology for the whole API. The API is about to
gain GraphQL, gRPC, AMQP RPC, NATS RPC and a Kafka event stream
(`docs/plans/ddd-transports/plan.md`), and every one of them must call the same use cases. With
one flat `app` package and one adapter per transport, each transport adapter would reach into
every concept, and nothing would say which code belongs together.

This ADR replaces only the layout and dependency-rule part of ADR-0003 (its "The layout is ports
and adapters" section and the depguard paragraph after it). Go, the standard library, wiring by
hand, typed errors and the cgo constraints stand.

## Decision Drivers

- Anything committed together, in one TigerBeetle chain or one Postgres transaction, belongs to
  one module. A write allocates ledger numbers (ADR-0005), creates accounts lazily (ADR-0006),
  inserts the idempotency record, submits one linked chain and commits the result (ADR-0007),
  and the reconciler replays the same code.
- Modules depend on each other only through application-layer interfaces, never through another
  module's domain or infrastructure package.
- Every transport calls the same use cases; no business rule in a transport.
- The direction is enforced by the linter, as ADR-0003 required, and the rules are proven to
  fire.
- The TigerBeetle client and the Postgres pool are shared resources, not owned by a module.

## Considered Options

1. Three modules, `ledgers`, `bookkeeping` and `health`, with a shared kernel (chosen).
2. One module per REST tag: ledgers, accounts, transactions, holds, settings, operations.
3. Keep ports and adapters and add one adapter per transport.

Option 2 splits what commits together: one transaction write touches assets, addresses,
accounts, the write request and the transaction row, so accounts, transactions, holds and
requests would turn every write into a cross-module saga. A separate `settings` module would add
two cross-module ports and no isolation, because rules and event types have the same consumer
and owner as the ledger. Option 3 keeps the problem described above.

## Decision Outcome

Chosen option: 1.

| Module        | Owns (tables)                                                                                               | REST operations                                                                                    |
| ------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `ledgers`     | `ledgers`, `account_rules`, `event_types`                                                                   | 10: the ledger CRUD and the six settings operations                                                |
| `bookkeeping` | `ledger_assets`, `addresses`, `accounts`, `write_requests`, `transactions`, `transaction_postings`, `holds` | 22: accounts, transactions, holds, write requests, ledger stats, assets, search, reconciler status |
| `health`      | none                                                                                                        | 3: liveness, readiness, info; an interfaces layer only                                             |

"Bookkeeping" names what the module does, recording transactions in accounts; "accounting" would
suggest reporting it does not do. REST tags do not move with modules: `getLedgerStats`,
`listAssets` and `search` keep the tag `ledgers`, because tags shape the generated client.

The tree under `apps/api/internal`:

- `kernel/ledger`: the shared kernel, moved from `internal/domain/ledger` (package name `ledger`
  unchanged). IDs, amounts, addresses and patterns, assets, metadata, rules and rule matching,
  event types, the six sentinel errors and `Clock`. It imports nothing from `internal`. It is not
  called `domain` because each module has its own `domain` folder, and one word must not mean two
  things.
- `modules/<name>/domain`: entities, rules and the driven ports (repository and store
  interfaces) with the record types they return. Imports the kernel and the standard library
  only. The ports live here, not in `application`, because infrastructure may import only the
  domain.
- `modules/<name>/application`: commands and queries as separate types per family, for example
  `TransactionCommands` (create, revert, replace metadata) and `TransactionQueries` (get, list),
  exported as interfaces in `usecases.go`. `ports.go` declares what the module needs from other
  modules. Imports its domain and the kernel.
- `modules/<name>/infrastructure`: Postgres through sqlc and pgx, TigerBeetle through the
  platform client. Imports its domain, the kernel and platform.
- `modules/<name>/interfaces`: one folder per transport, versioned (`rest/v1`, `grpc/v1`,
  `rpc/v1`), plus `graphql` (a schema evolves by deprecation, not by version) and `dto` for wire
  types several transports share. Calls the application interfaces only.
- `platform`: shared infrastructure without business rules, the role evrone's `pkg/` plays:
  config, logging, the HTTP server, the Postgres pool and migrations, the TigerBeetle client,
  readiness probes, request IDs, the transport-neutral problem mapping, wire scalars, and the
  per-transport plumbing added by later ADRs.
- `transport`: one package per transport that composes every module's interface layer (the one
  list of REST operations, the GraphQL resolver root). It may import modules; modules may not
  import it.
- `bootstrap`: builds resources, use cases and servers in order, and runs every server and
  background worker from one place. `cmd/api/main.go` reads config and calls it.

Modules never import each other. The consuming module declares the interface it needs in its
`application/ports.go`, using kernel types only; a query use case of the providing module has
those methods; `bootstrap` passes one to the other. Bookkeeping needs `LedgerDirectory`,
`AccountPolicies` and `EventTypes` from ledgers; ledgers needs `Footprints`, `EventTypeUsage` and
`AddressAssets` from bookkeeping. The modules depend on each other in both directions, but the
use cases do not: each side's queries read only its own tables, which fixes the construction
order. Go note: a type satisfies an interface by having its methods, with no `implements`
keyword, so neither module imports the other; a mismatch is a compile error in `bootstrap`.

Postgres keeps one schema and one goose sequence in `apps/api/migrations`, with a header comment
in each file naming the owning module. The foreign keys to `ledgers (name)` stay: they protect
data, and one process gains nothing from split schemas. `sqlc.yaml` gets one block per module
that reads the whole schema and only that module's queries.

depguard in `apps/api/.golangci.yml` enforces the direction with `list-mode: lax` rules: `kernel`
(no `internal` import, no driver, transport, framework or broker), `module-domain` (kernel only),
`module-application` (its domain only, no platform, no transport), `module-infrastructure` (no
application or interfaces), `module-interfaces` (no infrastructure, no driver), one
`isolate-<module>` rule per module (no file of another module imports it), `platform` (no
module, transport or bootstrap), plus the rules later ADRs add for the GraphQL registrations and
the spec commands. A `pkg` entry cannot hold a wildcard, so the rules name each module.

A rule that matches no file looks exactly like a rule that is obeyed; the current globs would
match nothing after the move. `apps/api/testdata/depguard` therefore holds one deliberate
violation per rule, under paths that mirror the real ones, and `just lint-depguard-probe` fails
unless each fixture yields exactly one finding from the expected rule. CI runs it.

### Consequences

- Good: each concept has one home, and every transport calls the same application interfaces;
  adapter tests use hand-written fakes of them.
- Good: the write protocol, lazy account creation and the reconciler stay in one module, so no
  write crosses a module boundary.
- Good: the linter proves the rules fire, not only that nothing breaks them.
- Bad: more packages and files than today; a reader follows `bootstrap` to see how modules meet.
- Bad: `getLedger` becomes two queries (the ledger row, then the footprint from bookkeeping), and
  `listLedgers` two per page.
- Bad: a new module needs its lines in every depguard rule that names modules; the probe fails
  until they are added.
- Bad: the kernel is shared by everything, so a change there touches every module.

## More Information

- [ADR-0003](0003-go-api-typescript-react-console.md): the ports-and-adapters layout this record
  replaces.
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md),
  [ADR-0006](0006-lazy-account-creation-and-address-rules.md),
  [ADR-0007](0007-linked-transfers-ids-idempotency.md): what one write commits together.
- `docs/plans/ddd-transports/plan.md`, sections 3, 4 and 14: the tree, the ports between modules,
  the depguard sketch and the probe.
- Reference layouts: https://github.com/meghrazchi/go-boilerplate (modules and layers),
  https://github.com/evrone/go-clean-template (transports and `pkg/`).
- depguard v2, the linter behind the rules: https://github.com/OpenPeeDeeP/depguard
