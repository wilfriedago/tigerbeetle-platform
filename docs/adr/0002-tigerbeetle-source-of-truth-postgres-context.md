# 2. TigerBeetle is the source of truth for money; Postgres holds context only

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

The platform exposes a Formance-style ledger API in front of TigerBeetle. TigerBeetle is a
purpose-built database for double-entry accounting: it stores accounts with debit and credit
totals and transfers between them, and it enforces balance limits inside the cluster. It is also
deliberately narrow. It has no string keys, no secondary indexes, no prefix queries and no free-form
metadata; an account is a fixed 128-byte record and a transfer a fixed 128-byte record, both
addressed by a 128-bit id.

The API, however, must offer names (ledgers as namespaces, addresses such as `users:1234:wallet`),
free-form metadata on accounts and transactions, idempotent writes and durable records of what
happened. Something has to hold that context, and the obvious candidate is Postgres.

Once two databases are involved the question is which one owns which fact. In particular: does
Postgres ever store a balance, even as a cache?

## Decision Drivers

- Correctness of money: a balance must have exactly one authoritative value, with limits enforced
  where the write happens.
- Simplicity for a small team: no background jobs whose only purpose is to keep two copies of a
  number in agreement.
- Operability: a clear answer to "which system do I trust when they disagree".
- Performance of TigerBeetle: it is optimised for time-ordered 128-bit ids, not for lookups by
  string.
- Developer ergonomics: ids must sort and compare the same way in Go, in SQL and in TigerBeetle.

## Considered Options

1. TigerBeetle owns money, Postgres owns context, Postgres never stores a balance. (Chosen.)
2. TigerBeetle owns money, Postgres keeps a denormalised balance cache for fast filters and sorts.
3. Postgres owns everything, TigerBeetle is a write-behind accelerator or is dropped.

Option 3 throws away the reason the project exists: TigerBeetle's balance invariants and
throughput. Option 2 is the tempting middle ground and is rejected below.

## Decision Outcome

Option 1. The two stores have disjoint responsibilities.

TigerBeetle holds every fact about money:
- accounts, one per (address, asset) pair, with their debit and credit totals (posted and
  pending) and their immutable flags;
- transfers, including pending transfers for holds, each with its ledger number, code and
  user data;
- the balance limits (`debits_must_not_exceed_credits` and friends), enforced atomically by the
  cluster at write time.

Postgres holds context, and only context:
- ledger names and metadata (`apps/api/migrations/00001_create_ledgers.sql` is the first table);
- the (ledger, asset) to TigerBeetle ledger number allocation;
- account rows: address, asset, the TigerBeetle account id, metadata, flags applied at creation;
- transaction rows: the API transaction id, its reference and metadata, and the leg ids;
- idempotency records: (ledger, Idempotency-Key), a hash of the request body, the minted ids,
  the state (pending, committed or failed) and the stored outcome;
- the outbox: rows describing committed transactions that still have to be published to
  consumers (events, webhooks, a future CDC feed), written in the same Postgres transaction as
  the commit mark so publication cannot be lost.

Postgres never stores a balance, not even as a cache. The ports in
`apps/api/internal/app/ports.go` encode this split: `LedgerStore` is the TigerBeetle port and is
described as the only store of balances and transfers; `MetadataStore` is the Postgres port.

### Why Postgres never stores a balance

A cached balance is a second source of truth. Every write would have to update TigerBeetle and
Postgres, and the two cannot be updated atomically: there is no distributed transaction across
them. A crash between the two writes, a retried request, or a reconciler replay leaves the
cache wrong, and a wrong cached balance is worse than a slow one because it looks right. Keeping
the cache honest means a reconciliation job that reads TigerBeetle anyway, which is the work we
wanted to avoid. Reading the balance from TigerBeetle with `lookup_accounts` is cheap, so the cache
would buy little and cost an invariant.

### What the idempotency records and the outbox are for

The write path for a transaction is: INSERT the idempotency record as pending with freshly
minted ids, create any missing accounts, submit the linked chain of transfers to TigerBeetle,
then UPDATE the record to committed. The record exists because the process can die between
these steps. On a retry with the same Idempotency-Key, or when the reconciler finds an old
pending record, the stored transfer ids let the service ask TigerBeetle with `lookup_transfers`
whether the chain already landed before deciding to resubmit. TigerBeetle itself treats the
transfer id as the idempotency key of `create_transfers`, which is why the ids are minted once and
stored, never regenerated. The outbox is the same idea for publication: it records "this
committed" durably so a publisher can deliver it later without the API request waiting.

### Identifier representation (decision 11, Postgres side)

TigerBeetle ids are 128-bit integers. Postgres has no native 128-bit integer, so they are stored in
`uuid` columns, which are 16 bytes and index and compare bytewise. For bytewise comparison to
equal numeric comparison the bytes must be big-endian (most significant byte first). The Go
client stores `tb.Uint128` little-endian, so the domain type `ledger.ID` in
`apps/api/internal/domain/ledger/id.go` is defined as big-endian and the adapter converts at the
boundary in `apps/api/internal/adapters/tigerbeetle/id.go` (`ToDomainID` and `FromDomainID`
reverse the 16 bytes). The payoff: time-ordered ids minted with the client's `ID()` helper sort
by creation time in a plain `ORDER BY id`, cursor pagination can use the id as the cursor, and a
B-tree index on the column appends at the right-hand side instead of writing randomly. The
JSON form is the 32-character lowercase hex of the same big-endian bytes, so the string seen
by API clients also sorts correctly.

Byte order is the kind of mistake that shows up as "pagination skips rows" months later, so it
is enforced in one place and covered by `apps/api/internal/adapters/tigerbeetle/id_test.go`.

### Consequences

- Good: one authoritative balance, enforced by the cluster, with no reconciliation of money
  between two databases.
- Good: a crash anywhere in the write path is recoverable by reading stored ids and asking
  TigerBeetle, because Postgres records intent and TigerBeetle records money.
- Good: Postgres stays a plain relational schema (names, jsonb metadata, uuid ids) that sqlc and
  goose handle without surprises.
- Good: ids sort by time in Go, in SQL and in TigerBeetle alike.
- Bad: most reads join both systems. Listing accounts means resolving addresses and metadata in
  Postgres, then calling `lookup_accounts` (up to 8189 ids per request) for the balances. Two
  round trips, two failure modes, two readiness probes (`apps/api/internal/app/status.go`).
- Bad: no balance filters or sorts in SQL. Formance's `balance` filter and ordering by balance
  are not offered; the API sorts by id or address and clients filter balances themselves.
- Bad: aggregated balances by address prefix (for example everything under `users:`) are
  computed by resolving the prefix to account ids in Postgres and summing chunks of
  `lookup_accounts` results. Each chunk is consistent, but the sum is not one snapshot when
  transfers land between chunks. The API documents this as an approximate aggregate.
- Bad: a read that races a write can observe metadata for a transaction whose transfers are not
  yet visible, or the reverse. Responses expose the idempotency state (pending or committed) so
  clients can tell.
- Bad: a developer new to the codebase must learn that "the account table has no balance
  column" is intentional, hence this document.

## More Information

- [ADR-0004](0004-formance-style-api-surface.md) covers the JSON side of decision 11 (amounts
  as strings, ids as hex).
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md) covers the (ledger, asset) to
  ledger number allocation that Postgres owns.
- [ADR-0006](0006-lazy-account-creation-and-address-rules.md) describes the INSERT ... ON
  CONFLICT DO NOTHING plus `create_accounts` sequence that relies on Postgres owning the address
  to id mapping.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md) details the idempotency record, the
  minted ids and the reconciler.
- Repo paths: `apps/api/internal/app/ports.go`, `apps/api/internal/domain/ledger/id.go`,
  `apps/api/internal/adapters/tigerbeetle/id.go`, `apps/api/internal/adapters/postgres/store.go`,
  `apps/api/migrations/00001_create_ledgers.sql`, `apps/api/queries/ledgers.sql`.
- TigerBeetle docs: data modeling and time-based ids
  (https://docs.tigerbeetle.com/coding/data-modeling/), reliable transaction submission
  (https://docs.tigerbeetle.com/coding/reliable-transaction-submission/), `lookup_accounts`
  (https://docs.tigerbeetle.com/reference/requests/lookup_accounts/), `query_accounts`
  (https://docs.tigerbeetle.com/reference/requests/query_accounts/).
