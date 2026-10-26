# 28. Authorization through platform grants, enforced in the application layer

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

Once callers are known (ADR-0026) and belong to organizations (ADR-0027), the platform must
decide what each may do. Who may move money on which ledger is a business rule of this platform,
not of the identity provider: roles change when a ledger is created or a member is added, and
Keycloak organizations have no roles of their own (Keycloak issue 50606). Putting a role per
ledger in tokens would grow every token with every ledger and keep a revoked role valid until
the token expires.

The API has five transports over one application layer (ADR-0021). A check written in a
transport would have to be written five times, and a transport that forgot it would be a hole.

This record adds the `access` module to the modules of ADR-0017, with the same dependency rules;
everything else in ADR-0017 stands.

## Decision Drivers

- One check per use case, the same answer on every transport.
- No use case reachable without a check, enforced by a test rather than by review.
- Revoking a role takes effect on the next call.
- Ledger names do not leak across organizations.
- Room for finer rules (address prefixes, amount limits) without touching use cases.

## Considered Options

1. Roles and permissions defined in the kernel, grants in Postgres, checked by wrappers around
   each module's use cases.
2. Roles in token claims (Keycloak organization groups, ADR-0027).
3. A policy engine now: cedar-go, Casbin, OPA, OpenFGA or SpiceDB.

Option 2 has the token problems above. Option 3 pays off with attribute conditions or
relationships; for fixed roles per organization and ledger it adds a policy language or a
service for what one indexed query does. cedar-go is the first candidate when attribute
conditions appear.

## Decision Outcome

Chosen option: 1.

- Permissions: `ledger.read`, `money.write`, `metadata.write`, `ledger.configure`,
  `ledger.members`, `org.ledgers`, `org.members`, `org.credentials`, `org.operate`,
  `platform.organizations`. Roles: on a ledger, `viewer`, `operator`, `admin`; in an organization,
  `member`, `admin`, `owner`; and staff. The table is in
  `apps/api/internal/kernel/ledger/permission.go`, so every module agrees on it without importing
  another.
- Staff hold a Keycloak realm role and only `platform.organizations`. They read a tenant's
  ledgers only if that organization grants them a role, visible in its member list.
- The `access` module owns `grants` (organization, issuer, subject, scope: the organization or one
  ledger ID, role, granted by and at) and implements `Authorizer`: `Require(ctx, permission,
  ledgerName)` and the list of ledgers a caller may see.
- Each module declares `Authorizer` in its `application/ports.go` and wraps its own use cases in
  `application/authorize.go`: one type per use-case interface whose methods call `Require`, then
  the real use case. The constructor returns only wrapped values. A test walks every method of
  every exported use-case interface by reflection and fails if one has no permission.
- `LedgerQueries.List` filters by grants in SQL, so cursors stay correct.
- Answers: no credential or a bad one is 401 `UNAUTHENTICATED`; a caller with no grant on a
  ledger, or not a member of the named organization, gets 404, as if it did not exist; a caller
  who can see the ledger but lacks the permission gets 403 `FORBIDDEN`. The kernel gains
  `ErrUnauthenticated` and `ErrForbidden`; `internal/platform/problem` renders them per transport.
- The reconciler and the CDC relay run as `ledger.System`, set only by `internal/bootstrap`.

### Consequences

- Good: every transport, present and future, gets the same decision without code of its own.
- Good: the reflection test makes "forgot the check" a failing build.
- Good: one indexed query per check, no cache, immediate revocation.
- Good: the `Authorizer` port is where a policy engine would plug in later.
- Bad: one more query per call. A cache can come later, at the price of delayed revocation.
- Bad: two places hold access facts (membership in Keycloak, roles in Postgres); removing a
  member must clear both, which the member use case does.

## More Information

- `docs/plans/auth/plan.md`, sections 6.4 to 6.6.
- [ADR-0017](0017-bounded-context-modules-and-dependency-rules.md): the module rules this record
  extends.
- [ADR-0021](0021-transports-over-one-application-layer.md): the per-transport error mapping.
- cedar-go: https://github.com/cedar-policy/cedar-go
