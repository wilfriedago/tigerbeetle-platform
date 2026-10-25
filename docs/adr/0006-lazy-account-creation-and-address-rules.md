# 6. Lazy account creation and address-pattern account rules

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

The API addresses accounts by hierarchical strings such as `users:1234:wallet` (ADR-0004), while TigerBeetle only knows 128-bit account IDs with a small set of immutable fields: `ledger`, `code`, `flags` and the `user_data` slots. API clients never create accounts explicitly: the first posting that names an address brings the account into existence. We therefore need to decide when a TigerBeetle account is created, which ID it gets, which flags and code it carries, and how to make that creation safe to repeat, because a request can crash between the Postgres write and the TigerBeetle write and the reconciler (ADR-0007) will run the sequence again.

Three facts about TigerBeetle constrain the design. Account fields and flags are immutable after creation. `create_accounts` returns `exists` only when every field except `timestamp` and the balance fields is identical to the stored account; any difference yields `exists_with_different_flags`, `_ledger`, `_code` or `_user_data_128/64/32`. And the account `id` is the idempotency key of `create_accounts`, so repeating a creation with the same id and the same fields is harmless.

This record covers decisions 4 (lazy creation), 5 (address-pattern rules) and 9 (account IDs).

## Decision Drivers

- Creation must be idempotent across retries and across the reconciler, with no special cases.
- Two concurrent first uses of the same address must converge on one TigerBeetle account.
- The limit flag on an account is a product rule (users cannot overdraw, `world` can) and must be applied uniformly, not chosen per request.
- TigerBeetle throughput: the docs recommend time-based IDs; random or hash-derived IDs defeat LSM optimisations.
- Postgres never stores a balance (ADR-0002); it may store everything else about an account.

## Considered Options

1. Explicit account creation endpoint, posting to an unknown address is an error.
2. Lazy creation on first use, with per-request flags supplied by the caller.
3. Lazy creation on first use, with flags and code resolved from address-pattern rules and frozen on the Postgres row at creation.
4. Account ID derived from a hash of `(ledger, address, asset)`.
5. Account ID minted with the TigerBeetle client's `ID()` helper and stored in Postgres.

## Decision Outcome

Chosen: option 3 for creation and rules, option 5 for IDs.

### The lazy-creation sequence

For every `(address, asset)` pair named by a transaction, inside the write path of ADR-0007:

1. Resolve the account rule for the address (see below). This yields the limit flag, the `history` flag and a non-zero `code`. The TigerBeetle ledger number comes from the `(ledger, asset)` row of ADR-0005.
2. Mint a candidate ID with `app.IDGenerator.NewID()`, implemented by `Client.NewID` in `apps/api/internal/adapters/tigerbeetle/client.go` with `tb.ID()`.
3. `INSERT INTO accounts (id, ledger, address, asset, tb_ledger, flags, code, created_at) VALUES (...) ON CONFLICT (ledger, address, asset) DO NOTHING`, then `SELECT` the rows for every pair in the batch. The re-select is required because `ON CONFLICT DO NOTHING` returns nothing for the loser of a race; the winner's row, with its id, flags and code, is what every caller uses from now on. A losing candidate ID is simply discarded.
4. Call `create_accounts` for every account in the batch, built only from the re-selected rows, and treat `exists` exactly as `created`. Submitting all rows and not only the ones this request inserted is what heals a crash between steps 3 and 4: the Postgres row exists, the TigerBeetle account does not, and the next request that touches the address creates it.
5. Submit the linked transfer chain (ADR-0007).

### Why `exists` is success and `exists_with_different_*` is an invariant violation

Step 4 always sends the fields stored on the Postgres row, so a repeat is byte-for-byte identical and TigerBeetle answers `exists`. The docs say many applications should handle `exists` exactly as `created`, and that is what we do. `exists_with_different_flags`, `_ledger`, `_code` or `_user_data_*` can only occur when the service sent different fields for the same id, which means the row was altered or the fields were recomputed from something other than the row. The use case does not map these to a client-facing error such as `ledger.ErrInvalid`: it fails the request as an internal error, logs the account id and the differing field, and the chain is not submitted. It is a bug to fix, never a condition to retry.

### Why flags and code are persisted at insert time

Flags and code are written to the `accounts` row in step 3 and read back from it in step 4, never re-evaluated from the live rules. Two reasons:

- TigerBeetle flags are immutable. If a rule changes after an account exists and we recomputed flags from the rules, the healing path in step 4 would send different flags and hit `exists_with_different_flags`. Freezing the values on the row keeps step 4 idempotent forever.
- The row then documents what the account really is in TigerBeetle, which is what operators and the console need to read. A rule change applies to accounts created after it; existing accounts keep their flags, and the only way to change one is to stop using it and route to a new address.

`code` is a `u16` that TigerBeetle requires to be non-zero, and `ledger` is a non-zero `u32`; both are checked when the row is built, before any write, so a misconfigured rule fails as `ledger.ErrInvalid` on the first use rather than as a TigerBeetle result code.

### Address-pattern rules and specificity

A rule is `(ledger, pattern, limit flag, history, code)`. A pattern is either `*`, the catch-all, or an address prefix made of whole segments, matched with `Address.HasPrefix` in `apps/api/internal/domain/ledger/address.go`, so `users:1` matches `users:1:wallet` but not `users:12:wallet`. The most specific matching rule wins, where specificity is the number of segments in the prefix and `*` counts as zero. Two distinct prefixes with the same segment count cannot both match one address, so there are no ties. An exact address is just a prefix with all segments spelled out, which is how `world` is matched.

Two built-in rules apply when a ledger defines nothing else:

- `*`: `debits_must_not_exceed_credits`. With a posting's source as the debit account and its destination as the credit account (ADR-0007), this keeps credits minus debits from going below zero. TigerBeetle includes pending debits in that check, so a hold (ADR-0010) reduces the spendable balance.
- `world`: no limit flag. Money enters and leaves the ledger through `world`, so it goes arbitrarily negative. `ledger.World` is the constant for this address.

A rule sets at most one of `debits_must_not_exceed_credits` and `credits_must_not_exceed_debits`; TigerBeetle rejects both together. Rules are stored in Postgres per ledger; the endpoints to manage them come with the transactions use case.

### The `history` flag

`get_account_balances` returns point-in-time balances only for accounts created with the `history` flag, and the flag cannot be added later. It is therefore part of the rule, decided per pattern at creation time, and defaults to false. Point-in-time balance queries are out of scope of this API unless a pattern opts in; opting in later only affects accounts created after the change, which is a direct consequence of the freezing rule above.

### Account IDs

Every account ID is minted by `tb.ID()`: a 48-bit millisecond timestamp plus random bits, monotonic within a process, as the TigerBeetle docs recommend. We never derive the ID from a hash of the address, even though that would remove the Postgres lookup, because hash-derived IDs have significantly lower throughput in TigerBeetle. The ID is stored in a `uuid` column in big-endian order so that byte order equals numeric order and rows sort by creation time; the client's little-endian form is converted in `apps/api/internal/adapters/tigerbeetle/id.go` (ADR-0002 for the byte-order rule). Resolving an address to its ID is always a Postgres read of `(ledger, address, asset)`.

### Consequences

- Good: creation is idempotent at every step, so retries and the reconciler run the same code with no special cases.
- Good: concurrent first uses converge through the unique constraint, and a lost race costs one unused ID.
- Good: the Postgres row is the complete, auditable description of the account, and rules can evolve without touching existing accounts.
- Good: time-ordered IDs keep `create_accounts` and `lookup_accounts` on TigerBeetle's fast path.
- Bad: every write resolves addresses through Postgres before reaching TigerBeetle; this is the price of string addresses on a store with no string keys.
- Bad: a wrong default rule is permanent for the accounts it created; fixing it means new addresses, not an update.
- Bad: rules are evaluated only on first use, so an address that should have been `world`-like but was first used under `*` stays limited.
- Bad: `history` must be decided up front; a later need for historical balances on existing accounts cannot be met.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): TigerBeetle holds balances, Postgres holds context, 128-bit IDs as big-endian uuid columns.
- [ADR-0004](0004-ledger-api-surface.md): hierarchical addresses and the ledger namespace.
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md): one TigerBeetle ledger number per `(ledger, asset)`.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md): the write path and reconciler this sequence runs inside, and the posting direction (source is the debit account, destination the credit account).
- [ADR-0010](0010-holds-as-two-phase-transfers.md): holds and their effect on spendable balance.
- Repo paths: `apps/api/internal/domain/ledger/address.go`, `apps/api/internal/domain/ledger/id.go`, `apps/api/internal/app/ports.go` (`IDGenerator`), `apps/api/internal/adapters/tigerbeetle/client.go`, `apps/api/internal/adapters/tigerbeetle/id.go`, `apps/api/migrations/` (the `accounts` migration lands with the transactions use case).
- TigerBeetle docs relied on: https://docs.tigerbeetle.com/reference/account/ (fields, flags, immutability, `history`), https://docs.tigerbeetle.com/reference/requests/create_accounts/ (`exists` and `exists_with_different_*`), https://docs.tigerbeetle.com/reference/requests/get_account_balances/, https://docs.tigerbeetle.com/coding/data-modeling/ (time-based IDs).
