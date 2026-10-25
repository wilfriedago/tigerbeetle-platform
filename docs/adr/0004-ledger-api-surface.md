# 4. Ledger API surface

Status: Accepted, amended by [ADR-0013](0013-ledger-api-implementation.md)
Date: 2026-10-24

## Context and Problem Statement

TigerBeetle exposes a small, fixed-shape API: accounts and transfers addressed by 128-bit IDs,
with no string keys, no metadata and no prefix queries. That narrowness is what makes it fast and
safe, but application teams do not want to think in those terms. They want named ledgers,
readable account addresses, multi-leg transactions with metadata, and list endpoints with
pagination, and today each team builds that layer around TigerBeetle on its own. We need to
decide what the public API of this platform looks like, how amounts are represented in JSON, and
which common ledger features we knowingly leave out because TigerBeetle cannot or should not
provide them.

The OpenAPI document at `packages/openapi/openapi.yaml` is the contract (ADR-0008). Today it only
describes `GET /healthz`, `GET /readyz` and `GET /v1/_info`; this ADR fixes the shape the ledger
endpoints will take as they are added.

## Decision Drivers

- Readability: the resources should use plain double-entry vocabulary (ledgers, accounts,
  transactions, postings), so an engineer can use the API before learning TigerBeetle's data
  model.
- Faithful to TigerBeetle: every resource should map onto something TigerBeetle does natively,
  so the TigerBeetle documentation still explains what the API does.
- Honesty about the backend: TigerBeetle is the only source of truth for money (ADR-0002) and
  has no string keys or prefix queries, so the surface must not promise reads it cannot serve
  consistently.
- Exact money: amounts must survive JSON parsers in every language without rounding.
- Generated code: the surface must be expressible in OpenAPI 3.1 so the Go server interfaces and
  the TypeScript client stay generated, never hand-written.

## Considered Options

1. Expose TigerBeetle more or less directly: accounts and transfers by 128-bit ID, numeric
   ledgers and codes, no metadata. Simple to build, but every consumer would rebuild the naming
   and metadata layer themselves, which is the problem this project exists to solve.
2. A ledger surface built on TigerBeetle's own primitives (linked transfers, two-phase
   transfers, balance limits, debit and credit volumes), with explicit limits where TigerBeetle
   semantics rule a feature out. Readable resources, and the limits are documented rather than
   papered over.
3. A general-purpose ledger surface offering every feature such APIs commonly have, including
   client-supplied timestamps, forced reverts and balance filters. This would require emulating
   features in Postgres that contradict ADR-0002, or quietly returning wrong answers.

## Decision Outcome

Chosen option: 2, a ledger surface built on TigerBeetle's primitives. The platform adds names,
metadata and pagination around TigerBeetle; it does not hide or work around TigerBeetle's
semantics. Every feature we leave out is listed below with the TigerBeetle reason.

### Resources

- Ledgers are namespaces. `POST /v1/ledgers/{ledger}` creates one; the name rule is the
  `CHECK` constraint in `apps/api/migrations/00001_create_ledgers.sql`, mirrored by
  `ValidateLedgerName` in `apps/api/internal/domain/ledger/ledger.go`. Ledgers carry metadata.
- Accounts are addressed by hierarchical strings such as `users:1234:wallet`, validated by
  `ParseAddress` in `apps/api/internal/domain/ledger/address.go`. Segments are separated by
  colons and prefixes match whole segments only, so `users:1` does not cover `users:12:wallet`.
  `world` is the conventional counterpart for money entering or leaving the ledger. Accounts
  carry metadata and expose per-asset balances and volumes. Internally one address maps to one
  TigerBeetle account per asset (ADR-0005, ADR-0006), but the API hides that.
- Transactions hold one or more postings, each with `source`, `destination`, `asset` and
  `amount`, plus a `reference` and metadata. One transaction is one batch of linked TigerBeetle
  transfers (ADR-0007), so a transaction either fully commits or fully fails. The number of
  postings per transaction is capped well below TigerBeetle's request limit because a linked
  chain cannot span requests.
- Revert: `POST /v1/ledgers/{ledger}/transactions/{id}/revert` creates a new transaction with
  the postings reversed and links the two through metadata. The reverting transaction is a
  normal linked chain in the opposite direction and can be declined, for example with
  `exceeds_credits` when the destination has already spent the funds.
- Aggregated balances: `GET /v1/ledgers/{ledger}/aggregate/balances?address=users:` sums the
  balances of every account under a prefix, per asset.
- Volumes: total debits and total credits per account and asset, which TigerBeetle tracks
  natively on every account.
- Cursor pagination on every list endpoint, with an opaque `cursor` query parameter and a
  `next` cursor in the response. Offsets are not offered because the underlying tables grow
  append-only and offset scans would get slower over time.
- Holds expose TigerBeetle's two-phase pending transfers, with capture, partial capture and void,
  as their own resource. They are described in ADR-0010.

### Amounts and identifiers in JSON

JSON amounts are strings holding an integer in the asset's smallest unit, matching the regex
`^(0|[1-9][0-9]*)$`. `EUR/2` with amount `"1250"` means 12.50 EUR. Strings rather than numbers
because TigerBeetle amounts are unsigned 128-bit integers and JSON numbers are commonly parsed as
IEEE 754 doubles, which silently lose precision above 2^53. The regex forbids signs, leading
zeros, decimals and exponents so that two equal amounts always have one spelling. The upper bound
is `MaxAssetScale` in `apps/api/internal/domain/ledger/asset.go` for the scale and 2^128 - 1 for
the integer; both checks belong to the domain package (the amount type lands with the
transactions use case), and the HTTP adapter maps the resulting `ledger.ErrInvalid` to a `400`
problem via `apps/api/internal/adapters/httpapi/problem.go`.

Transaction and account IDs are 128-bit time-ordered IDs (ADR-0006 for accounts, ADR-0007 for
transactions), rendered as 32 lowercase hexadecimal characters by `ledger.ID.String` in
`apps/api/internal/domain/ledger/id.go`, most significant byte first, so the string sorts like
the number (ADR-0002).

### Deliberate limits

Each item is a common ledger feature that TigerBeetle rules out or restricts, or that would
contradict ADR-0002.

- No client-supplied transaction timestamps. TigerBeetle assigns the timestamp when the transfer
  is committed and that is the only time the ledger knows. A client may send an optional
  business time, which is stored in `user_data_64` and returned as a separate field.
- No `force` on revert. TigerBeetle transfers and account flags are immutable, so a revert is a
  new transaction that can be declined; there is no way to force a balance below its limit.
- No revert `atEffectiveDate`, for the same reason: the revert is timestamped by TigerBeetle.
- No filters or sorts by balance on list endpoints. Postgres never stores a balance (ADR-0002),
  so such a query would need a full scan of TigerBeetle. Clients filter by address, metadata and
  time instead.
- Prefix aggregation is chunked and capped. Postgres resolves the prefix to account IDs, the
  service calls `lookup_accounts` in chunks of at most 8189 IDs and sums the results. The sum
  is not a single consistent snapshot across chunks, and a prefix covering more accounts than a
  documented cap is rejected rather than answered slowly.
- No unkeyed writes. Every write requires an `Idempotency-Key` header (ADR-0007 covers the
  semantics).

### Consequences

- Good: engineers can use the API in plain double-entry terms, and each resource still maps onto
  one TigerBeetle concept they can look up when they need the detail.
- Good: string amounts and documented limits mean the API never returns a number that is
  rounded, nor an answer that TigerBeetle did not actually compute.
- Good: the surface stays generatable; adding an endpoint means editing the spec, running
  `pnpm generate` and implementing the generated interface in
  `apps/api/internal/adapters/httpapi`.
- Bad: engineers used to other ledger APIs will look for the features listed above, and every
  missing one is a question we will be asked. This ADR is the answer we point to.
- Bad: aggregated balances over large prefixes are more expensive and less consistent than in a
  SQL-backed ledger; consumers that need exact totals should keep a dedicated aggregate account.
- Bad: string amounts are slightly less convenient in loosely typed clients, which must use a
  big-integer type rather than `Number`.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): TigerBeetle holds money,
  Postgres holds context, ids are big-endian in Postgres and hex in JSON.
- [ADR-0005](0005-assets-on-postings-ledger-per-ledger-asset.md): assets on postings and one
  ledger number per (ledger, asset).
- [ADR-0006](0006-lazy-account-creation-and-address-rules.md): one TigerBeetle account per
  (address, asset), its time-ordered ID and its flags.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md): a transaction is one batch of linked
  transfers, posting direction, transaction IDs and the Idempotency-Key semantics.
- [ADR-0008](0008-openapi-first-generated-server-and-client.md): the OpenAPI spec is the
  contract.
- [ADR-0010](0010-holds-as-two-phase-transfers.md): holds as two-phase transfers.
- Spec and generated code: `packages/openapi/openapi.yaml`, `packages/api-client`,
  `apps/api/internal/adapters/httpapi/gen`.
- Domain rules: `apps/api/internal/domain/ledger/{address,asset,ledger,errors}.go`.
- TigerBeetle references used: https://docs.tigerbeetle.com/reference/requests/lookup_accounts/,
  https://docs.tigerbeetle.com/reference/requests/query_transfers/,
  https://docs.tigerbeetle.com/reference/transfer/ (immutability, linked chains, timestamps),
  https://docs.tigerbeetle.com/reference/account/ (debits and credits volumes).
