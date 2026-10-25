# 24. Transport-neutral write protocol in the bookkeeping module

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

ADR-0007 defines the write protocol: mint IDs, record the attempt as pending, submit one linked
chain, resolve the record, and let a reconciler settle what a crash left pending. It speaks HTTP:
"every write endpoint requires an `Idempotency-Key` header", and a replay returns "the stored
status code and body". ADR-0004 says the same of the header. ADR-0013 narrowed the key to the five
writes that move money and implemented the protocol once, in `internal/app/writes.go`, storing
HTTP status numbers on the record (`writes.go:24-32`), with the reconciler as a goroutine started
by `cmd/api/main.go`.

The API is being split into modules (ADR-0017) and served over REST, GraphQL, gRPC, AMQP RPC and
NATS RPC (ADR-0021). The protocol must behave the same on every transport, so it must not depend
on any of them.

This record replaces only these parts: in ADR-0013, the location of the protocol, the meaning of
the stored status numbers and the reconciler's process wiring; in ADR-0004 and ADR-0007, the
Idempotency-Key as a header on every write and the replay of a stored status and body. The
protocol's steps, record states, ID minting, chain classification and reconciler rules stand.

## Decision Drivers

- One protocol in the application layer; transports only carry the key and the request ID.
- The same write sent twice with one key has one outcome, whichever transports carried it.
- Every transport answers an in-flight key, a reused key and a missing key the same way.
- REST behaviour does not change for third parties (ADR-0018).

## Considered Options

1. Keep the protocol in bookkeeping's application layer and let each transport carry the key in
   its own idiom (chosen).
2. Idempotency middleware per transport.
3. Keep the protocol HTTP-shaped and translate other transports into HTTP requests.

Option 2 repeats the protocol five times and lets them drift. Option 3 makes every transport
pretend to be REST and keeps status numbers as the protocol's language.

## Decision Outcome

Chosen option: 1.

- Location: `internal/modules/bookkeeping/application`: `writeflow.go` (begin, submit, pending,
  decline, commit), `ensure_accounts.go` (ADR-0005 and ADR-0006 inside the write),
  `reconciler.go` (the settle command and its loop). The record ports and types live in
  `bookkeeping/domain` (ADR-0017).
- Carrying the key: a header on REST (`Idempotency-Key`, unchanged), AMQP RPC
  (`idempotency-key`) and NATS RPC (`Idempotency-Key`); a field of the mutation or request
  message on GraphQL (`idempotencyKey`) and gRPC (`idempotency_key`), where one request can hold
  several writes and the field is typed by the schema. The scope stays the five money writes:
  transactions, reverts, holds, captures and voids.
- A missing or empty key is the domain's `missing-idempotency-key` on every transport: 400 on
  REST, `INVALID` elsewhere (ADR-0021).
- The body hash covers the normalised command the use case builds, not the bytes on the wire.
  A write sent over REST and retried over GraphQL with the same key is a replay; a different
  operation or different content under that key is `idempotency-key-reused`.
- Replays, as the code already does: a committed write is read back and answered as a success,
  so the client sees the resource as it is now; a declined or failed write returns its stored
  problem; a pending one answers `request-in-flight` with a retry delay, and a write left pending
  by an unanswered store answers `write-pending` with one. Each transport renders these through
  the shared problem mapping.
- Stored outcome numbers: the record keeps `response_status` as today (201, 200, 400, 409, 422,
  500, 503), because REST exposes it as `responseStatus` and the console's stored-problem view
  reads it. The constants move to `bookkeeping/domain/outcome.go`, documented as "numbered like
  HTTP statuses". No transport derives its own code from them; each maps from the problem kind.
- Reconciler: a bootstrap runner (ADR-0021) in the API process, behind `RECONCILER_ENABLED`
  (default true), with `RECONCILER_INTERVAL` and `RECONCILER_GRACE_PERIOD` unchanged. A separate
  worker command waits until a production deployment exists.
- Broker redelivery is safe: a redelivered write carries its key and replays; reads have no side
  effect.
- Tests: use-case tests use hand-written fakes and no transport; an integration test sends one
  write over REST and replays it over GraphQL with the same key.

### Consequences

- Good: one implementation of ADR-0007 for every transport, tested without any of them.
- Good: a client may retry over another transport without creating a second write.
- Good: the stored `responseStatus` keeps its meaning for REST and the console.
- Bad: the stored status numbers are HTTP-flavoured data in a transport-neutral module; the
  name and comment of the constants have to say why.
- Bad: GraphQL and gRPC clients put the key in the input while REST clients send a header; the
  API documentation of each transport must say where it goes.

## More Information

- [ADR-0007](0007-linked-transfers-ids-idempotency.md): the protocol, its states and the
  reconciler.
- [ADR-0004](0004-ledger-api-surface.md): the Idempotency-Key rule this record carries to every
  transport.
- [ADR-0013](0013-ledger-api-implementation.md): the implementation this record moves.
- [ADR-0017](0017-bounded-context-modules-and-dependency-rules.md),
  [ADR-0021](0021-transports-over-one-application-layer.md).
- `docs/plans/ddd-transports/plan.md`, section 10.
- Code today: `apps/api/internal/app/writes.go`, `apps/api/internal/app/reconciler.go`,
  `apps/api/internal/app/transactions.go` (the replay re-reads the transaction).
