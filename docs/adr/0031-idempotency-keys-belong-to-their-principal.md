# 31. Idempotency keys belong to the principal that used them

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

ADR-0007 keys idempotency records by `(ledger, idempotency_key)`, and ADR-0024 carries the key
on every transport. A retry with the same key and body replays the stored answer. That was safe
while every caller was the same anonymous caller.

With authentication (ADR-0026) several principals write to the same ledger: people in the
console, API keys, machine clients. A principal that reuses another principal's key would get
that principal's stored answer, with its transaction or hold, which is a read it may not be
allowed, and could believe its own write succeeded.

This record replaces only the replay rule of ADR-0007 and ADR-0024 (any caller with the key
replays); everything else in both stands.

## Decision Drivers

- A replay never returns another principal's answer.
- `/v1/ledgers/{ledger}/requests/{key}` and the console's search by key keep working.
- The reconciler keeps settling interrupted writes exactly as before.

## Considered Options

1. Keep one key per ledger; record the actor; replay only for the same actor.
2. One key namespace per principal: the key becomes `(ledger, principal, idempotency_key)`.

Option 2 lets two callers use the same key independently, but a key no longer names one write in
a ledger, so the request detail by key and the search by key need the principal too.

## Decision Outcome

Chosen option: 1.

- `write_requests` gains `actor_issuer` and `actor_subject`, set when the record is inserted.
- The same principal, same key, same body: replay, as today. Same principal, other body: 422
  `idempotency-key-reused`, as today.
- Another principal, same key, whatever the body: 422 `idempotency-key-reused`, without the
  stored answer or any part of it.
- The reconciler, running as `ledger.System`, settles records whatever their actor; it never
  replays to a caller.
- Records written before this change have no actor; they replay only to the system and answer
  422 to everyone else.

### Consequences

- Good: no cross-principal replay; the key still names one write in a ledger.
- Good: the actor is recorded on every write, the first piece of an audit trail.
- Bad: two callers in one ledger cannot pick the same key; keys are random UUIDs in practice, so a
  collision means a bug or a probe, and the answer reveals only that the key is taken.

## More Information

- `docs/plans/auth/plan.md`, section 6.9.
- [ADR-0007](0007-linked-transfers-ids-idempotency.md),
  [ADR-0024](0024-transport-neutral-write-protocol.md): the records this one amends.
- `apps/api/migrations/00006_create_write_requests.sql`: the table this record extends.
