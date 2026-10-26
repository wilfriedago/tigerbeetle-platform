# 13. Implementing the ledger API: schema, write protocol and reads

Status: Accepted, amended by [ADR-0024](0024-transport-neutral-write-protocol.md) and [ADR-0025](0025-problem-details-per-rfc-9457.md)
Date: 2026-10-25

## Context and Problem Statement

ADR-0004 to ADR-0010 describe the ledger surface and its rules: ledgers as namespaces, one
TigerBeetle ledger number per (ledger, asset), lazy accounts with frozen flags, linked chains with
minted IDs and idempotency records, holds as two-phase transfers. They leave open several choices
that an implementation must make, and `docs/console.md` lists endpoints no ADR covers (the ledger
list, filters, the request log, the reconciler status, rule resolution, the event type registry,
overview counts). This record fixes those choices as implemented in `apps/api` and
`packages/openapi/openapi.yaml`.

## Decision Drivers

- Stay inside ADR-0002: Postgres holds context and IDs, never an amount it would have to keep
  consistent with TigerBeetle.
- Every write that moves money is safe to retry and survives a crash at any step (ADR-0007).
- Answers that clients can branch on: stable problem types, not parsed titles.
- Reads that scale with the page, not with the ledger: no per-row TigerBeetle round trip.

## Decision Outcome

### Schema

Eight goose migrations in `apps/api/migrations`: `ledger_assets` (ADR-0005, keyed by
`(ledger, asset)` rather than `ledger_name`), `account_rules`, `event_types`, `addresses` and
`accounts`, `write_requests`, `transactions` with `transaction_postings`, and `holds`.

- Addresses use the `"C"` collation. A whole-segment prefix `p` becomes the exact range
  `[p || ':', p || ';')` (';' is the byte after ':'), served by the primary key index, and `_` in
  an address is never read as a `LIKE` wildcard.
- Transactions and holds store which transfers and accounts they consist of, never the amounts.
  Reads call `lookup_transfers` once per page. `committed_at` is a copy of the TigerBeetle
  timestamp for time filters; responses use the TigerBeetle timestamp itself (nanoseconds).
- The idempotency record stores the canonical request. ADR-0007 keeps a body hash; the reconciler
  also needs the body to rebuild a chain that never reached the cluster. It is context, not a
  balance.

### Write protocol

`internal/app/writes.go` implements ADR-0007 once for transactions, reverts, holds, captures and
voids: insert the pending record with every minted ID, ensure assets and accounts (ADR-0005,
ADR-0006), submit, resolve. Details the ADRs left open:

- A record ends `committed`, `declined` (TigerBeetle refused a valid request: a balance limit, a
  resolved hold) or `failed` (anything else, such as a burned ID whose reason is gone).
- A write interrupted by an unanswered store leaves its record pending and answers 503
  `write-pending`. A retry with the same key answers 409 `request-in-flight` with `Retry-After`
  until the reconciler settles it, then replays the outcome.
- The reconciler is a goroutine in the API process (`RECONCILER_INTERVAL`, default 10 s) that
  settles records older than `RECONCILER_GRACE_PERIOD` (default 30 s) exactly as ADR-0007 says.
  Every TigerBeetle call is bounded by `TIGERBEETLE_TIMEOUT` (default 5 s), which must stay below
  the grace period.
- "At most once" rules are unique indexes, checked before TigerBeetle is called: a transaction is
  reverted once (`write_requests_one_revert`) and a hold resolved once
  (`write_requests_one_resolution`), while the first attempt is pending or committed. A declined
  attempt frees the slot, so a corrected request can try again.
- Business time is stored in `user_data_64` in Unix milliseconds.
- When a write names no event type, the built-in `default` type (code 1) is used, because
  TigerBeetle rejects code 0. Event types are per ledger, which answers the open question of
  `docs/console.md`; codes start at 2 and are assigned in order unless the client picks one.

### Idempotency-Key scope

ADR-0004 says every write requires an `Idempotency-Key`. The implementation requires it on the
five writes that move money. Ledger creation, metadata replacement, rules and event types are
idempotent by their nature (a `PUT`, or a create that answers 409 on a second call) and take no
key.

### Rule patterns

A pattern is `*`, an exact address (`world`), or a prefix ending with a colon (`users:`), which
covers the addresses below it on whole segments. Specificity is the number of segments, so an
exact address and a prefix can never tie for one address. A stored `*` or `world` replaces the
built-in default; deleting it restores the default.

### Problems

Domain errors carry a stable reason (`ledger.Problem`), rendered as the problem type
`/problems/<reason>`: for example `exceeds-credits`, `idempotency-key-reused`,
`request-in-flight`, `write-pending`, `already-reverted`, `hold-already-resolved`,
`hold-expired`, `prefix-too-broad`, `history-not-enabled`. A declined chain adds `posting` (the
zero-based index of the failing posting) and `results` (the TigerBeetle result of every posting).
Two sentinels join the four of ADR-0004: `ErrRejected` (422) and `ErrConflict` (409).

### Endpoints added beyond the ADRs

`GET /v1/ledgers` (cursor by name), `GET /v1/ledgers/{ledger}/stats`,
`GET /v1/ledgers/{ledger}/assets`, `GET /v1/ledgers/{ledger}/aggregate/segments` (the chart of
accounts by segment), `GET /v1/ledgers/{ledger}/accounts/{address}/history`,
`GET /v1/ledgers/{ledger}/requests[/{key}]`, `GET /v1/ledgers/{ledger}/rules`,
`PUT|DELETE /v1/ledgers/{ledger}/rules/{pattern}`, `GET /v1/ledgers/{ledger}/rules/match`,
`GET|POST /v1/ledgers/{ledger}/event-types`, `GET /v1/ledgers/{ledger}/search` and
`GET /v1/_reconciler`.

### Consequences

- Good: one write path for five operations, verified end to end against real Postgres and
  TigerBeetle, including both crash windows of ADR-0007 (`cmd/api/api_integration_test.go`).
- Good: list pages cost one Postgres query and one TigerBeetle lookup.
- Bad: the stored request duplicates the amounts of in-flight and past writes in Postgres. They
  are request records, never read as balances, but they are money facts outside TigerBeetle.
- Bad: the reconciler runs in every API replica; concurrent replicas settle the same record
  safely (the commit is conditional on `status = 'pending'`), but do the lookups twice.
- Bad: hold expiry is derived from the API clock and the TigerBeetle timestamp; a hold may show
  as expired a moment before the cluster releases it, and the capture then succeeds.

## More Information

- ADR-0002, ADR-0004, ADR-0005, ADR-0006, ADR-0007, ADR-0010.
- `apps/api/internal/app/writes.go`, `transactions.go`, `holds.go`, `reconciler.go`;
  `apps/api/internal/adapters/postgres`, `apps/api/internal/adapters/tigerbeetle/ledger.go`;
  `apps/api/migrations`, `apps/api/queries`.
