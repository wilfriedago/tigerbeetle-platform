# Console

Status: implemented, 2026-10-25. Every page below exists in `apps/console`; the endpoints are
in `packages/openapi/openapi.yaml` and ADR-0013 records the ones no earlier ADR covered.
ADR-0014 records how the console is built (TanStack, feature slices, the design system).

This document describes the structure of the web console (`apps/console`): every route, what
each page shows, which actions it offers and which API it depends on. The console only talks to
the platform API (ADR-0003), so every page below is bounded by what the API can answer: the
ledger pages through GraphQL (`POST /graphql`, ADR-0023), the System page through the REST
client `packages/api-client`. The domain rules that shape the pages come from the decision
records linked in each section. The "API:" line of each page names the REST operation as it was
proposed; all of them exist, and the page reads the same use case through the matching GraphQL
field, often several of them in one request.

Where the code lives: the route file is `apps/console/src/routes/_console/` plus the URL (for
example `ledgers/$ledger/transactions/index.tsx`), and the page is in `src/features/<slice>/`.

## Layout

The shell follows the usual ledger-console pattern: an organization switcher, a ledger switcher, a
sidebar scoped to the selected ledger, a page title with a one-line description, and overview
cards. Every page needs a signed-in person (ADR-0030): without a session the browser goes to the
identity provider's sign-in and comes back to the page it asked for. Pages of an organization live
under `/$org/`, so each tab keeps its own organization (ADR-0027). The account menu shows the
person and their role, and signs out. Actions the person's role does not allow are hidden, such
as "New ledger" for a member; the API still checks every call (ADR-0028).

Top bar:

- Ledger switcher: search, plus "New ledger".
- Global search (Cmd+K): a 32-hex ID opens the matching transaction or hold, an address opens the
  account, a reference or an Idempotency-Key finds the write.
- Health indicator fed by polling `GET /v1/_info`.

Sidebar, scoped to the current ledger:

- Overview
- Activity: Transactions, Holds
- Money: Accounts, Balances, Assets
- Operations: Requests
- Settings: Ledger, Account rules, Event types
- Pinned at the bottom: System, Preferences

## Sitemap

```
/                                      redirect: last-used organization and ledger, else its ledgers,
                                       else /welcome for a person in no organization
/welcome                               Welcome (no organization yet)
/preferences                           Preferences (no organization)

Every path below is under /$org, the organization's alias, such as /acme/ledgers.

/ledgers                               Ledgers
/ledgers/$ledger                       Overview
/ledgers/$ledger/transactions          Transactions
/ledgers/$ledger/transactions/new      New transaction
/ledgers/$ledger/transactions/$txId    Transaction
/ledgers/$ledger/accounts              Accounts (list | tree)
/ledgers/$ledger/accounts/$address     Account
/ledgers/$ledger/balances              Balances (prefix aggregation)
/ledgers/$ledger/holds                 Holds
/ledgers/$ledger/holds/new             New hold
/ledgers/$ledger/holds/$holdId         Hold
/ledgers/$ledger/assets                Assets
/ledgers/$ledger/requests              Write requests (idempotency log)
/ledgers/$ledger/settings              Ledger settings
/ledgers/$ledger/settings/rules        Account rules
/ledgers/$ledger/settings/event-types  Event types
/system                                System health
/system/reconciler                     Reconciler
/preferences                           Console preferences

Later
/ledgers/$ledger/posting-rules[/$rule[/versions/$v]]   posting-rule DSL
/ledgers/$ledger/live                                  live feed (TigerBeetle CDC)
/login, /org/members, /org/api-keys, /audit            once auth is decided
```

The ledger is a path segment, not a query parameter, so console URLs mirror the API paths of
ADR-0004 (`/v1/ledgers/{ledger}/...`) and every view is a shareable link.

## Ledger pages

### Ledgers

`/ledgers`. API: planned (`POST /v1/ledgers/{ledger}`, ADR-0004); the list endpoint is new.

- Table: name, created at, asset badges, metadata preview, with cursor paging.
- "New ledger" dialog: name, checked against the same rule as the `CHECK` constraint in
  `apps/api/migrations/00001_create_ledgers.sql` and shown inline; metadata key/value pairs.
- Empty state: a create button plus the equivalent `curl` command.
- No delete action: TigerBeetle accounts and transfers are permanent.

### Overview

`/ledgers/$ledger`. API: planned; the counts are new.

- Header: name, creation date, metadata chips.
- Cards:
  - Money issued per asset: minus the balance of `world`, one account lookup per asset. Money
    enters and leaves through `world` (ADR-0006), so this is the cheapest "how much money is in
    this ledger" figure.
  - Accounts: count from Postgres.
  - Open holds: count and reserved amount per asset.
  - Declined writes in the last 24 hours, with the top reasons such as `exceeds_credits`.
- The 10 most recent transactions.
- Quick actions: New transaction, New hold, Look up address.

### Transactions

`/ledgers/$ledger/transactions`. API: planned (ADR-0004); the filters are new.

- Columns:
  - short ID with a copy button;
  - committed time, the TigerBeetle timestamp;
  - business time, shown only when set and labelled separately (ADR-0004);
  - reference;
  - postings summary, such as `world -> users:1234:wallet 12.50 EUR`, plus "+N more";
  - event type;
  - reverted or reverts badge.
- Filters, kept in the URL: time range, address, asset, reference, event type, metadata,
  reverted. Which of these Postgres can serve still has to be designed.
- Not offered: sorting or filtering by amount or balance (Postgres holds no balances,
  ADR-0002), and page numbers or total counts (paging is cursor-based, ADR-0004).
- Declined writes are not transactions in TigerBeetle. They appear under Requests.

### New transaction

`/ledgers/$ledger/transactions/new`. API: planned (ADR-0004, ADR-0007).

A three-step form with a progress stepper (Postings, Details, Review). A finished step can be
reopened from the stepper; nothing is sent before the Review step commits. A summary column
stays on the right: the postings so far and their effect on balances.

1. Postings. One block per posting:
   - source and destination are account comboboxes, not free text. Typing filters existing
     accounts by address prefix on whole segments; each option shows the available balance
     in the posting's asset; `world` is always offered; an address that does not exist is
     offered as "Use new address" with the rule that will create it and its limit (ADR-0006);
   - the amount is typed in major units with the asset beside it and shown as the integer
     that will be sent (`12.50 EUR` sent as `"1250"`), using bigint arithmetic only
     (ADR-0005: the scale is a label, amounts are integer strings);
   - a note gives what the source can spend at that point of the chain, counting earlier
     postings;
   - a counter shows the cap of 64 postings per transaction (ADR-0007).
2. Details: reference, event type (from the registry), optional business time, metadata.
3. Review and commit:
   - the postings drawn as one linked chain, the details with an Edit link per section;
   - preflight checks done in the console (one asset per posting, accounts that will be
     created, the posting cap, available balances); the preview is advisory, TigerBeetle
     enforces the limits;
   - "View JSON" and "Copy as curl" with the exact request body and `Idempotency-Key`;
   - the read-only Idempotency-Key field and the Commit button.

- Submission states (ADR-0007):
  - 201: open the new transaction.
  - 409 `request-in-flight`: retry after `Retry-After`.
  - Declined: show which posting failed and why, and explain that the key stays declined and
    that "Retry" uses a new key.

### Transaction

`/ledgers/$ledger/transactions/$txId`. API: planned (ADR-0004).

- Header: ID, committed time, business time, reference, event type (name and code).
- Postings table with the transfer ID of each leg.
- Net effect per account and asset.
- Revert:
  - shows the link either way ("reverted by" or "reverts");
  - the Revert action's dialog explains that a revert is a new transaction that can be
    declined, and that there is no force option (ADR-0004).
- Metadata: view, and edit if the API allows it.
- Technical tab:
  - the TigerBeetle transfers: ledger number, `code`, `linked` flag, `user_data_128`;
  - the Idempotency-Key and request ID;
  - raw JSON.

### Accounts

`/ledgers/$ledger/accounts`. API: planned (ADR-0004, ADR-0006).

- Two views:
  - List: address, asset badges, balance and reserved amount per asset, overdraft limit
    ("cannot go negative" or none), created date, metadata. Balances come from one batched
    `lookup_accounts` call for the visible page.
  - Tree, the chart of accounts by segment (`users` > `1234` > `wallet`): each node shows its
    number of child accounts and a balance on demand.
- Address-prefix filter with whole-segment semantics and the hint "`users:1` does not match
  `users:12`" (`Address.HasPrefix` in `apps/api/internal/domain/ledger/address.go`).
- No balance sort, with a tooltip explaining why.
- No "Create account" button: accounts are created by their first transaction (ADR-0006).
  Instead, a "Fund an account" shortcut opens a transaction from `world`.

### Account

`/ledgers/$ledger/accounts/$address`. API: planned (ADR-0004, ADR-0006, ADR-0010).

- Header: each address segment links to Balances at that prefix.
- One card per asset:
  - balance (credits minus debits);
  - reserved (pending debits, ADR-0010);
  - available (balance minus reserved);
  - incoming pending credits;
  - total debits and total credits (volumes).
- Frozen properties: flags, `code`, the rule that matched at creation, and the TigerBeetle
  account ID and ledger number per asset, with a note that they cannot change (ADR-0006).
- This account's transactions, and its holds (open first).
- Balance history chart only when the account has the `history` flag; otherwise a note linking
  to Account rules, since the flag cannot be added later.
- Actions: New transaction from or to this address, New hold, Edit metadata.

### Balances

`/ledgers/$ledger/balances`. API: planned (`GET /v1/ledgers/{ledger}/aggregate/balances`,
ADR-0004).

- Input: an address prefix, autocompleted by segment.
- Per-asset totals: balance, reserved, and the number of accounts summed.
- Freshness note: an "as of" time and the warning "summed in chunks, not one snapshot".
- Over the prefix cap: an error asking to narrow the prefix.

### Holds

`/ledgers/$ledger/holds`, `/holds/new`, `/holds/$holdId`. API: planned (ADR-0010).

- List:
  - status tabs: pending, captured, voided, expired;
  - columns: source, destination, amount, captured amount, created, expiry countdown;
  - Capture and Void actions on each row.
- New hold: like a one-posting transaction, plus a timeout in seconds with a readable preview
  ("expires about 14:32, cluster clock"). The preview shows the source's available balance after
  the hold.
- Detail:
  - timeline: created, then captured, voided or expired;
  - Capture dialog, full or partial: a partial capture closes the hold and releases the
    remainder, and a hold is captured only once;
  - errors for "already resolved" and "expired" (409);
  - Technical tab: the pending transfer and the transfer that resolved it.

### Assets

`/ledgers/$ledger/assets`. API: planned (ADR-0005). Read-only.

- Table: `CODE/SCALE`, TigerBeetle ledger number, first used, account count, money issued.
- Note: a scale cannot change; a new scale means a new asset (ADR-0005).

### Requests

`/ledgers/$ledger/requests`. API: new (reads the idempotency records of ADR-0007).

- Table:
  - key and operation (transaction, hold, capture, void, revert);
  - status: pending, committed or failed;
  - created and resolved times;
  - link to what was created;
  - problem type for failures;
  - request ID.
- Highlight pending records older than the reconciler's grace period.
- Row drawer: the stored response or problem body, the body hash and the minted IDs.
- This is the only place where declined and in-flight writes are visible.

### Ledger settings

`/ledgers/$ledger/settings`. API: planned (ADR-0004).

- Name (read-only), creation date, metadata editor.
- No delete action.

### Account rules

`/ledgers/$ledger/settings/rules`. API: planned; the endpoints come with the transactions use
case (ADR-0006).

- Table:
  - pattern;
  - limit: none, debits must not exceed credits, or credits must not exceed debits;
  - history flag;
  - code;
  - the built-in `*` and `world` rules shown as defaults.
- Banner: a change applies only to accounts created afterwards; existing accounts keep their
  flags.
- "Which rule wins?" tester: type an address and the matching rule is highlighted (the rule
  with the most matching segments wins).

### Event types

`/ledgers/$ledger/settings/event-types`. API: new.

- Table: name, code (u16, never zero), usage count.
- Add only, no edit or delete, because a code never changes meaning (ADR-0007).
- Open question: ADR-0007 does not say whether this registry is per ledger or global. If it is
  global, the page moves under `/system`.

## Platform pages

### System health

`/system`. API: exists (`GET /v1/_info`). Today's console page becomes this one.

- Postgres and TigerBeetle status with latency, refreshed automatically.
- Service name and version.
- TigerBeetle client and server versions, once the API reports them.

### Reconciler

`/system/reconciler`. API: new (an admin endpoint).

- Last run and pending records per ledger.
- The "some legs found, some missing" cases from the reconciler algorithm in ADR-0007, with
  their transfer IDs.

### Preferences

`/preferences`. No API; stored in the browser only.

- Theme and density.
- Amounts in major units or raw integers.
- Times in UTC or local time.

## Later

- Posting rules: a list of rules with immutable versions, an editor, a compile preview that
  shows the linked chain a rule produces, and a dry run against a sample event
  (`docs/architecture.md`, Future work).
- Live feed: a stream of transfers and balance changes from TigerBeetle CDC, filterable by
  address and asset, with pause.
- Once authentication is decided: login, members and roles, API keys, audit log.

## Rules shared by every page

- URL as state: filters, tabs and the cursor live in search params (TanStack Router
  `validateSearch`), so every view is a shareable link.
- Amounts: shared `Amount` and `AmountInput` components that never use JavaScript `number`,
  format with the asset scale and show the raw integer on hover (ADR-0004, ADR-0005).
- IDs: monospace, shortened in the middle, with a copy button. Transaction and account IDs are
  32 lowercase hex characters (ADR-0004).
- Time: the TigerBeetle timestamp is the authoritative time; business time is always labelled
  separately.
- Errors: one panel for `application/problem+json` answers, with title, detail, the problem's
  reason linked to its entry in `docs/problems.md` (ADR-0025) and a copyable request ID. Code
  compares problems by reason (`ApiError.reason`, `problemReason`), never by parsing `type`.
- Writes: one `useIdempotentMutation` hook. It creates the Idempotency-Key when a form opens,
  reuses it on retry, renews it when the body changes, and surfaces the "a declined key stays
  declined" rule (ADR-0007).
- Paging: Next and Previous over a stack of cursors, with no totals and no page numbers.

## Endpoints that were designed for the console

The pages needed endpoints no earlier ADR covered: the ledger list, transaction list filters,
the write-request log, the reconciler status, rule resolution, the event-type registry and the
overview counts. ADR-0013 records them; they are all in `packages/openapi/openapi.yaml`.

## What is left

- Transactions: the metadata and "reverted" filters (the API filters by address, asset,
  reference, event type and committed time today) and an export.
- Account rules: editing a rule's pattern means deleting it and creating another.
- Posting rules and the live feed (see Later).
