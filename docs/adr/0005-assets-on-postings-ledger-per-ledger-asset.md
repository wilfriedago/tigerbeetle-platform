# 5. Assets on postings; one TigerBeetle ledger number per (ledger, asset)

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

The API is Formance-style (ADR-0004): a ledger is a namespace, an account is a hierarchical
address such as `users:1234:wallet`, and each posting of a transaction names an asset. The same
address can hold several currencies, and two ledgers can both contain `users:1234:wallet` without
sharing anything.

TigerBeetle has no notion of currency or namespace. It has a `ledger` field, a 32-bit unsigned
integer that "partitions the sets of accounts that can transact with each other", and a transfer
is rejected unless both accounts and the transfer carry the same ledger number. Amounts are plain
integers; the asset scale (how many decimal places the integer represents) is a convention the
application attaches to a ledger number, and the docs state that changing it requires a new
ledger number.

We need to decide where the asset is declared in the API, how a (ledger, asset) pair becomes a
TigerBeetle ledger number, who allocates those numbers, and how the service finds them on the
hot path.

## Decision Drivers

- Money correctness: a transfer must never mix currencies or cross namespaces, and the check
  should be enforced by TigerBeetle, not only by service code that could have a bug.
- Scale safety: an amount integer is meaningless without its scale, so the scale must be
  impossible to change silently after the first transfer.
- Hot path cost: resolving the ledger number must not add a Postgres round trip to every
  transfer once the mapping is known.
- Formance familiarity: the `CODE/SCALE` asset syntax already used by Formance Ledger keeps
  the API recognisable.

## Considered Options

1. Asset on the posting, written `CODE/SCALE`, one TigerBeetle ledger number per (ledger, asset).
2. Asset on the account (an account has one fixed currency), one ledger number per currency
   shared across all API ledgers.
3. Asset on the posting, one ledger number per API ledger, currency separation in service code only.

## Decision Outcome

Chosen option: option 1.

### Asset syntax and canonical form

An asset is `CODE/SCALE`, for example `XOF/0` or `EUR/2`. The grammar lives in
`apps/api/internal/domain/ledger/asset.go`:
- `CODE` is uppercase ASCII, a letter followed by 1 to 15 letters or digits.
- `SCALE` is 0 to `MaxAssetScale` (18) decimal places. `EUR/2` means the amount integer is in
  cents; `XOF/0` has no fractional unit.
- A missing scale means zero. `ParseAsset("EUR")` is `EUR/0`, not `EUR/2`.
- The canonical form rendered by `Asset.String()` always includes the scale, so `EUR` and
  `EUR/0` compare equal and responses never show a bare code.

JSON amounts are strings in the asset's smallest unit (ADR-0004), so a posting is
`{"source": "world", "destination": "users:1234:wallet", "asset": "EUR/2", "amount": "1050"}`
for 10.50 EUR. The service never multiplies or divides amounts; the scale is a label.

### Scale is part of the asset identity

`EUR/2` and `EUR/3` are two different assets and therefore two different TigerBeetle ledger
numbers. This is how we honour the TigerBeetle rule that a ledger's asset scale cannot change:
"you would need to use a different ledger number and duplicate all the accounts on that ledger
over to the new one". There is no API to rescale an asset; a caller who needs a new scale uses a
new asset name and moves funds with ordinary transactions.

### Ledger number allocation in Postgres

Postgres is the allocator of ledger numbers, because TigerBeetle stores the number only as a
field on accounts and transfers and has no registry of its own. A planned migration under
`apps/api/migrations/` adds a table of the shape:

```sql
CREATE TABLE ledger_assets (
    ledger_name text NOT NULL REFERENCES ledgers (name),
    asset text NOT NULL,
    tb_ledger integer GENERATED ALWAYS AS IDENTITY,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ledger_name, asset),
    CONSTRAINT ledger_assets_tb_ledger_positive CHECK (tb_ledger > 0),
    CONSTRAINT ledger_assets_tb_ledger_unique UNIQUE (tb_ledger)
);
```

- `asset` is stored in canonical form, so `EUR` and `EUR/0` cannot create two rows.
- The identity column starts at 1 and only ever grows. Zero is never allocated because
  TigerBeetle rejects it (`ledger_must_not_be_zero` on accounts and transfers). A Postgres
  `integer` tops out at 2^31 - 1, inside the u32 range TigerBeetle accepts.
- Numbers are never reused and rows are never deleted or updated. A sequence value burned by a
  rolled-back insert leaves a gap, which is harmless: uniqueness is all TigerBeetle needs.
- Allocation is lazy and uses the same pattern as account creation (ADR-0006):
  `INSERT ... ON CONFLICT DO NOTHING` followed by a `SELECT`, so two concurrent first uses of
  `(ledger, asset)` agree on one number.

### What TigerBeetle enforces

Because every (ledger, asset) pair has its own number and every TigerBeetle account is created
with the number of its pair (ADR-0006), the cluster itself rejects any transfer that would mix
currencies or namespaces:
- `accounts_must_have_the_same_ledger` when the debit and credit accounts carry different
  numbers, which is what a `EUR/2` to `XOF/0` posting, or a posting between two API ledgers,
  would produce.
- `transfer_must_have_the_same_ledger_as_accounts` when the accounts agree but the transfer's
  own `ledger` field does not; this catches a service bug that resolved the wrong number.
- `pending_transfer_has_different_ledger` for a post or void that names a hold on another
  ledger, which protects the hold flows of ADR-0010.

The adapters map these results to `ledger.ErrInvalid`, which the HTTP adapter answers with a
400 problem (`apps/api/internal/adapters/httpapi/problem.go`). The service also rejects a
mixed-asset posting before submission, but the cluster check is the one we rely on.

### In-process cache of the mapping

The mapping `(ledger name, canonical asset) -> tb_ledger` is immutable once written, so the
service keeps it in a process-local map behind the `MetadataStore` port. A miss reads Postgres
(allocating if needed) and fills the map; a hit costs nothing. No invalidation is required:
rows never change, and several API replicas caching independently cannot disagree.

### Multi-currency exchange later

When exchange arrives, it will be an explicit linked chain: leg 1 on the `EUR/2` number debits
`users:1234:wallet` and credits a liquidity account such as `fx:eur`; leg 2 on the `XOF/0`
number debits `fx:xof` and credits `users:1234:wallet`. Each transfer stays inside its own
ledger number, and the `linked` flag on leg 1 makes both succeed or both fail, which is the
approach the TigerBeetle currency exchange recipe describes.

### Consequences

- Good: cross-currency and cross-namespace transfers are impossible at the storage layer, even
  if the service has a bug.
- Good: the asset scale is fixed by construction; nothing can reinterpret historical amounts.
- Good: the hot path needs no Postgres round trip for the ledger number after first use.
- Good: a ledger number is a stable filter for `query_accounts` and `query_transfers`, which
  accept `ledger` as an exact-match criterion.
- Bad: `EUR` parses as `EUR/0`. A caller who omits the scale on a cents-based asset gets a
  separate asset and a separate ledger number, and the funds do not mix. The API documentation
  must say so plainly and the console should always show the canonical form.
- Bad: multi-currency exchange is two linked transfers through liquidity accounts (see above).
- Bad: aggregated balances by address prefix must be grouped by asset, one sum per number.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): who owns money and context.
- [ADR-0004](0004-formance-style-api-surface.md): ledgers as namespaces, string amounts.
- [ADR-0006](0006-lazy-account-creation-and-address-rules.md): one TigerBeetle account per
  (address, asset), created lazily with the number allocated here.
- [ADR-0010](0010-holds-as-two-phase-transfers.md): holds that post or void within one ledger.
- `apps/api/internal/domain/ledger/asset.go`, `apps/api/internal/domain/ledger/ledger.go`,
  `apps/api/migrations/`, `apps/api/queries/`.
- https://docs.tigerbeetle.com/reference/account/ (ledger field: u32, must not be zero,
  partitions accounts; account fields are immutable).
- https://docs.tigerbeetle.com/reference/requests/create_transfers/ (result codes
  `ledger_must_not_be_zero`, `accounts_must_have_the_same_ledger`,
  `transfer_must_have_the_same_ledger_as_accounts`, `pending_transfer_has_different_ledger`).
- https://docs.tigerbeetle.com/coding/data-modeling/ (ledgers, asset scale) and
  https://docs.tigerbeetle.com/coding/recipes/currency-exchange/ (linked transfers).
