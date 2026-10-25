# 7. Transactions as linked batches; ID scheme and idempotency

Status: Accepted, amended by [ADR-0013](0013-ledger-api-implementation.md)
Date: 2026-10-24

## Context and Problem Statement

An API transaction carries several postings (ADR-0004), and callers expect all of them
to apply or none. TigerBeetle only knows single transfers, batched into one request, with an
optional `linked` flag that ties consecutive transfers into an atomic chain. The service therefore
has to map one API transaction onto one linked chain, choose how every transfer and account gets
its 128-bit ID, and make writes safe to retry when the process crashes between "written to
Postgres" and "accepted by TigerBeetle".

Two TigerBeetle properties shape the answer. First, the `id` of `create_transfers` is its
idempotency key: the same id resubmitted returns `exists`. Second, since 0.16.4 an id that failed
with a transient error (`exceeds_credits`, `exceeds_debits`, account not found, pending transfer
not found, closed account) is burned, and resubmitting it returns `id_already_failed` forever.
An id is therefore spent on first contact with the cluster, whether it succeeded or not.

## Decision Drivers

- Atomicity: a multi-posting transaction must never half-apply.
- Exactly-once semantics for clients that retry over flaky networks.
- Crash safety: a reconciler must be able to finish or settle any write it finds half done.
- Throughput: the TigerBeetle docs recommend time-based IDs; random or hash-derived IDs
  significantly lower throughput because they defeat LSM optimisations.
- Traceability: every TigerBeetle transfer must point back to its API transaction.

## Considered Options

1. Transfer IDs derived by hashing (ledger, Idempotency-Key, leg index). Retries recompute the
   same ids without a Postgres lookup, but hash IDs hurt throughput, and the amendment of
   2026-10-24 rejects this for the reasons below.
2. IDs minted with the client's `ID()` helper and persisted on the idempotency record before the
   TigerBeetle call. Chosen.
3. One unlinked transfer per request with compensation on failure. Rejected: not atomic.

## Decision Outcome

Option 2, minted and persisted IDs, with one linked chain per transaction.

### Transaction to chain mapping

- One API transaction is one batch of linked transfers, submitted in one `create_transfers` call.
  Each posting becomes one transfer: the posting's source is the debit account and its
  destination the credit account (decision 6), so a user balance is credits minus debits.
- `linked` is set on every transfer except the last. A single-posting transaction sets no
  `linked` flag at all, because a one-event chain with `linked` set fails with
  `linked_event_chain_open`.
- `user_data_128` holds the transaction ID on every leg, so `query_transfers` filtered by
  `user_data_128` returns all legs of a transaction straight from TigerBeetle.
- `code` holds the business event type (a u16, never zero). The mapping table from event names to
  codes lives in Postgres and is append-only, so a code never changes meaning.
- The ledger number of each transfer is the (ledger, asset) ledger from ADR-0005, which is what
  makes cross-asset postings fail inside TigerBeetle rather than in our code.
- A linked chain cannot span requests and the default request limit is 8189 events. The service
  caps postings per transaction at 64 (`ledger.ErrInvalid` beyond that). Holds (ADR-0010) and
  lazy account creation (ADR-0006) each fit easily under that cap.
- A revert is a new transaction, hence a new chain in the opposite direction, with its own IDs and
  its own Idempotency-Key. It can fail with `exceeds_credits` like any other chain; there is no
  "force" revert because flags and balance limits are immutable after creation.

### ID scheme

- Transaction IDs, transfer IDs and account IDs (ADR-0006) all come from the TigerBeetle client's
  `ID()` helper: 48-bit millisecond timestamp plus random bits, monotonic within a process. The
  `app.IDGenerator` port in `apps/api/internal/app/ports.go` exposes it; the use case, not the
  adapter, decides when to mint.
- IDs are stored as `uuid` columns in big-endian byte order (ADR-0002 note on decision 11), so
  `ORDER BY id` is time order. `apps/api/internal/adapters/tigerbeetle/id.go` converts to the
  client's little-endian `Uint128`.
- IDs are never a hash of the Idempotency-Key or of the address. Besides throughput, a hash
  scheme would make a burned id unrecoverable: after `id_already_failed` nothing could ever be
  written under that key again, not even a corrected request.

### Idempotency-Key semantics

Every write endpoint requires an `Idempotency-Key` header. Records live in Postgres keyed by
(ledger, key) with a SHA-256 of the canonical request body.

| Situation                                | Response                                             |
| ---------------------------------------- | ---------------------------------------------------- |
| Header missing                           | 400 problem, `missing-idempotency-key`               |
| Key seen, body hash differs              | 422 problem, `idempotency-key-reused`                |
| Key seen, record still `pending`         | 409 problem, `request-in-flight`, with `Retry-After` |
| Key seen, record `committed` or `failed` | the stored status code and body, replayed            |
| Key new                                  | the write runs                                       |

A key maps to exactly one outcome. A transaction declined by TigerBeetle (for example
`exceeds_credits`) is stored as `failed` with its problem body and stays declined under that key,
because its transfer ids are burned. A client that wants to try again, for instance after funding
the account, must send a new key. The API reference will say so explicitly.

### Write path and record states

A record is `pending`, then either `committed` or `failed`; it never moves back.

1. Validate the request in the use case (addresses, assets, amounts, posting count).
2. Mint the transaction ID and one transfer ID per leg. `INSERT` the idempotency record with
   status `pending`, the body hash, the transaction ID and the leg IDs in one statement. A unique
   violation on (ledger, key) means another request got there first: re-select the record and
   apply the table above.
3. Lazily create accounts (ADR-0006), then build the chain from the stored leg IDs and call
   `create_transfers`.
4. On all `created`: `UPDATE` the record to `committed` and store the response body. On any
   other result: `UPDATE` to `failed` and store the problem body. Return the stored body.

The record is written before TigerBeetle is called so that a crash after step 3 leaves evidence
of which ids were in flight. Nothing in Postgres holds a balance; the record holds context and
ids only (ADR-0002).

### Reconciler algorithm

A background loop, and the same code path on request retry, settles `pending` records older than
a grace period (initially 30 seconds, well above the request timeout):

1. `lookup_transfers` with the stored leg IDs.
2. If every leg is found, the chain was accepted before the crash: mark `committed`.
3. If none is found, the chain never reached the cluster: resubmit it with the same ids. Because
   the ids are unchanged this is safe even if the first attempt did land after all, since
   TigerBeetle answers `exists` on the first leg and `linked_event_failed` on the rest, and the
   service treats the pattern `[exists, linked_event_failed, ...]` as already committed.
4. If some legs are found and others not, a linked chain cannot have produced that state: log at
   error level, leave the record `pending` and alert. It signals a bug in the chain mapping.
5. If the resubmission returns `id_already_failed` on a leg, the first attempt was declined:
   look up nothing further, mark `failed` with a generic problem body, because the original
   reason is no longer available from the cluster.

The reconciler never mints new ids and never guesses from the Idempotency-Key, which is why the
ids must be persisted before the first submission.

### Consequences

- Good: multi-posting transactions are atomic inside TigerBeetle, with no compensation logic.
- Good: retries and crash recovery reuse stored ids, so a transaction can never be applied twice.
- Good: time-ordered ids keep TigerBeetle and Postgres indexes sequential.
- Good: `user_data_128` and `code` make every transfer self-describing for audits and for the
  posting-rule DSL later on.
- Bad: one extra Postgres round trip (the pending insert) before every TigerBeetle call.
- Bad: a declined transaction can never be retried under its key; clients must learn to mint a new
  key for every retry after a decline.
- Bad: the `failed` branch of the reconciler loses the original decline reason, and the 64
  posting cap forces larger operations to be split into several transactions that are not
  atomic together.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md) for which store holds what.
- [ADR-0004](0004-ledger-api-surface.md) for the transaction and revert resources.
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md) for the ledger number per leg.
- [ADR-0006](0006-lazy-account-creation-and-address-rules.md) for step 3, account IDs and flags.
- [ADR-0010](0010-holds-as-two-phase-transfers.md) for the hold operations built on this path.
- Code: `apps/api/internal/app/ports.go` (`IDGenerator`, `LedgerStore`, `MetadataStore`),
  `apps/api/internal/domain/ledger/id.go` and `errors.go`,
  `apps/api/internal/adapters/tigerbeetle/id.go`, `apps/api/migrations/`, `apps/api/queries/`.
- TigerBeetle docs relied on: https://docs.tigerbeetle.com/coding/data-modeling/ (ids,
  user_data, code), https://docs.tigerbeetle.com/coding/linked-events/,
  https://docs.tigerbeetle.com/reference/requests/create_transfers/ (`exists`,
  `linked_event_failed`, `linked_event_chain_open`, `id_already_failed`),
  https://docs.tigerbeetle.com/coding/reliable-transaction-submission/.
