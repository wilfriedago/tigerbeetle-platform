# 10. Holds as two-phase transfers

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

The transactions of ADR-0004 move money immediately and have no notion of a hold: an
authorisation that reserves funds now and settles later for the same or a smaller amount, or is
cancelled. Payment flows need exactly that (card
authorisations, escrow, pre-paid bookings), and emulating it with two ordinary transactions
through an intermediate account is fragile: the reservation and its settlement are separate
writes, the "spendable" balance has to be recomputed by the caller, and nothing stops a double
settlement.

TigerBeetle has this concept natively. A transfer created with the `pending` flag reserves the
amount on both accounts without moving it, and is later resolved by exactly one follow-up
transfer: `post_pending_transfer` to settle or `void_pending_transfer` to release. This ADR
records that the API exposes two-phase transfers as a holds resource next to the transactions of
ADR-0004, and defines how each hold operation maps onto TigerBeetle.

## Decision Drivers

- Reuse TigerBeetle guarantees instead of re-implementing them in Postgres (ADR-0002: Postgres
  never stores a balance, so it cannot own "reserved" amounts either).
- A hold must resolve exactly once, whatever the caller retries.
- Reserved funds must reduce what an account can spend, without a second balance to maintain.
- Keep the API consistent with transactions: string amounts, Idempotency-Key, linked chains
  (ADR-0007), typed errors mapped to problem+json.
- Stay honest about what TigerBeetle does not support (multiple captures per hold).

## Considered Options

1. Native two-phase transfers: `pending`, `post_pending_transfer`, `void_pending_transfer`.
   Chosen.
2. Emulation with a per-account holding address (`users:1234:holds`) and two regular
   transactions. Rejected: no single-resolution guarantee, the spendable balance is a client-side
   subtraction, and the holding address pollutes account listings and aggregates.
3. No holds at all, keep the surface to immediate transactions. Rejected: the first consumer of
   this platform is a payments flow that cannot work without authorisations.

## Decision Outcome

Holds are a first-class resource under a ledger, with four operations. Each one is one API write
with its own Idempotency-Key and its own minted IDs (ADR-0007), and each one maps onto one
TigerBeetle transfer:

| API operation     | TigerBeetle transfer                                                            | Amount sent                                                 |
| ----------------- | ------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| Create hold       | flag `pending`, source = debit account, destination = credit account (ADR-0007) | the hold amount                                             |
| Capture (full)    | flag `post_pending_transfer`, `pending_id` = the hold's transfer id             | `AMOUNT_MAX` (2^128-1), which posts the full pending amount |
| Capture (partial) | flag `post_pending_transfer`, same `pending_id`                                 | the smaller amount; the remainder is released               |
| Void              | flag `void_pending_transfer`, same `pending_id`                                 | 0, which releases everything                                |

Rules the service enforces on top:

- One hold resolves exactly once. A partial capture posts the given amount and closes the hold;
  the uncaptured remainder goes back to the source. Capturing a hold in several instalments is
  not supported in this version. The known extension is a linked chain of
  [post_pending_transfer for the instalment, new pending transfer for the remainder], which
  creates a successor hold; it is deferred until a consumer needs it.
- Amount `0` is rejected at the API with `ledger.ErrInvalid` (400) for create and capture. Since
  TigerBeetle 0.16.0 a post with amount 0 voids the remaining amount instead of posting 0, so
  letting it through would silently turn a capture into a void. Void takes no amount at all; the
  adapter sends 0.
- Timeout is an optional `timeout_seconds` integer. It is passed through as the transfer's
  `timeout`, a u32 number of seconds relative to the cluster clock, never a wall-clock deadline
  chosen by the client (TigerBeetle assigns timestamps, see ADR-0004). An expired pending transfer
  is released by the cluster itself; the API reports it as status `expired`, derived at read time
  from `lookup_transfers` on the hold's transfer id and the absence of a post or void, and a
  capture or void against it fails with `pending_transfer_expired`, mapped to 409.
- The event type (`hold.created`, `hold.captured`, `hold.voided`) is stored in Postgres, never
  in `code` (`user_data_64` stays reserved for the optional client business time, ADR-0004). A
  post or void transfer's `code` must be 0 or equal to the pending transfer's code, so `code`
  keeps carrying the business event of the original hold (ADR-0007) and the resolving transfer
  repeats it.
- Both legs of a hold share the hold ID in `user_data_128`, so `query_transfers` by
  `user_data_128` returns the hold and its resolution together.

Why this reduces spendable balance for free: the default account rule (ADR-0006) sets
`debits_must_not_exceed_credits`, and that check includes pending debits. A hold on a user wallet
therefore lowers what the wallet can spend by exactly the held amount, and a later transfer that
would overdraw fails with `exceeds_credits` inside TigerBeetle. The API exposes this as a
`reserved` figure next to `balance` on account reads, computed from `debits_pending` returned by
`lookup_accounts`; Postgres is not involved.

Idempotency follows ADR-0007 unchanged: the resolving transfer id is minted when the pending
record is inserted and reused on retry; a second capture or void of the same hold with a new key
is answered by TigerBeetle with `pending_transfer_already_posted` or
`pending_transfer_already_voided`, which the adapter maps to `ledger.ErrAlreadyExists` (409).

### Consequences

- Good: exactly-once resolution is guaranteed by the cluster, not by application locks.
- Good: reserved funds are visible in the same balance check that protects every other transfer,
  so no separate "available balance" bookkeeping exists.
- Good: expiry is handled by the cluster clock, which survives API restarts and crashes.
- Good: holds reuse the write path, reconciler and ID scheme of ADR-0007 with no new tables for
  money, only metadata rows.
- Bad: one capture per hold. Instalment captures need the linked post plus new pending extension.
- Bad: holds are a second write resource next to transactions, with their own lifecycle
  (pending, captured, voided, expired) that the console, the client and the OpenAPI spec
  (ADR-0008) must model separately.
- Bad: a hold's `timeout` cannot be extended after creation (transfers are immutable); extending
  means voiding and creating a new hold under a new Idempotency-Key.
- Bad: `expired` is a derived status; a reader sees it only after the cluster has released the
  transfer, and the service has no push notification of that moment.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): Postgres never stores
  a balance, so reserved amounts live in TigerBeetle too.
- [ADR-0004](0004-ledger-api-surface.md): the transaction surface this ADR sits next to,
  and the features it leaves out (no client timestamps).
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md): one ledger number per
  (ledger, asset), so a hold and its resolution always share a ledger.
- [ADR-0006](0006-lazy-account-creation-and-address-rules.md): one TigerBeetle account per (address,
  asset), created lazily before the hold's transfer, and the `debits_must_not_exceed_credits`
  default.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md): minted IDs, `user_data_128`, `code`,
  Idempotency-Key and the reconciler.
- [ADR-0008](0008-openapi-first-generated-server-and-client.md): the holds resource lives in
  `packages/openapi/openapi.yaml` before any Go is written.
- Repo paths: `apps/api/internal/adapters/tigerbeetle/client.go` (client, `NewID`, error
  mapping), `apps/api/internal/adapters/tigerbeetle/id.go` (byte-order conversion),
  `apps/api/internal/domain/ledger/errors.go` (typed errors), `apps/api/internal/app/ports.go`.
- TigerBeetle docs: https://docs.tigerbeetle.com/coding/two-phase-transfers/ (pending, post,
  void, partial post, AMOUNT_MAX, timeout and expiry),
  https://docs.tigerbeetle.com/reference/transfer/ (flags, `pending_id`, `timeout`, `code`
  rules), https://docs.tigerbeetle.com/reference/requests/create_transfers/ (result codes),
  https://docs.tigerbeetle.com/reference/account/ (`debits_must_not_exceed_credits` includes
  pending debits, `debits_pending`).
