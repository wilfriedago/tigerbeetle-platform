# 26. Organizations as tenants, owned by Keycloak Organizations

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

The platform serves several organizations from day 1. Today a ledger name is the primary key of
`ledgers` (`apps/api/migrations/00001_create_ledgers.sql`), eight tables point at it by name
(`ledger_assets`, `account_rules`, `event_types`, `addresses`, `accounts`, `write_requests`,
`transactions`, `holds`), and ADR-0004 describes ledgers as one global namespace. Two tenants
could not both have a ledger called `payments`, and a name taken by one would reveal itself to
the other.

Three questions follow: who owns organizations and their members, how a call names its
organization, and how the data model keeps one tenant's rows away from another's.

This record replaces only the "ledgers are one global namespace" part of ADR-0004 (ledger names
are now unique per organization) and adds the platform's organization tables to the list of
what Postgres holds in ADR-0002; everything else in both stands.

## Decision Drivers

- No cross-tenant read or write, by construction rather than by every query remembering a filter.
- Invitations and per-organization single sign-on without building them (ADR-0025).
- REST paths, protos and GraphQL arguments stay as they are.
- Removing someone from one organization never touches their other memberships.
- TigerBeetle stays the only source of truth for money (ADR-0002).

## Considered Options

1. Keycloak Organizations own organizations and membership; the platform keeps a copy of each
   organization for its foreign keys, and owns everything inside.
2. The platform owns organizations and membership in Postgres; Keycloak only signs people in.
3. One Keycloak realm per organization.

Option 2 was the first proposal: vendor-neutral, but invitations and per-organization SSO would
have to be built or bolted on later. Option 3 isolates best but makes the API trust one issuer
per tenant and does not scale to many tenants in Keycloak. The owner chose option 1.

## Decision Outcome

Chosen option: 1.

- Keycloak owns each organization: name, alias, email domains, identity providers, members and
  invitations. Every membership is unmanaged in Keycloak's terms, so removing a membership never
  deletes the account. The platform never deletes a Keycloak organization.
- The platform's `organizations` table keeps its own ID (what platform tables point at), the
  alias (unique and immutable in Keycloak, so the link between the two), the Keycloak ID once
  known, the name, and a status, `active` or `suspended`. Suspension is enforced by the platform:
  every call in the organization's name answers 403 `organization-suspended`.
- Only staff create organizations, for now (ADR-0027). Creating one creates it in Keycloak, then
  the copy, then invites the first owner; retrying with the same alias finishes the job.
- The tenant of a call comes from its credential: an API key or a machine client belongs to one
  organization (ADR-0028). A person names the organization with the `X-Organization` header (the
  alias); people's tokens are requested with the scope `organization:*`, so their `organization`
  claim lists every membership, and the API checks the header against it. A non-member gets 404.
  The header travels like the request ID: an HTTP header, `x-organization` gRPC metadata, an
  `organization` message header on the brokers.
- Ledgers gain an internal `id` as primary key and `organization_id`; `UNIQUE (organization_id,
  name)`. The eight tables replace `ledger text` with `ledger_id uuid`. Every use case resolves
  the name once, inside the caller's organization, and uses the ID afterwards. A ledger ID is
  unique across the platform, so the organization is checked at that one step.
- TigerBeetle is unchanged: ledger numbers are allocated per (ledger, asset) from one sequence,
  so they are unique across the cluster, and transfers cannot cross TigerBeetle ledgers.
- Pre-alpha data moves into an organization with the alias `default`.

### Consequences

- Good: tenant isolation is a property of the data model; the isolation tests target one
  resolution step and the tenant middleware.
- Good: invitations, member lists, and SSO by email domain come from Keycloak.
- Good: no contract path changes; machine callers need nothing extra.
- Bad: membership is read from the token, so a removed member keeps access until the token
  expires, at most five minutes. Grants (ADR-0027) are checked on every call and revoke at once.
- Bad: the API depends on Keycloak's admin API for organizations and invitations, with a service
  account that needs `manage-organizations` and `manage-users`.
- Bad: once any organization exists, Keycloak's login asks for the username or email first.
- Bad: the ledger migration rewrites the keys of eight tables.

## More Information

- `docs/plans/auth/plan.md`, section 4.3 (the Keycloak Organizations facts) and section 5.
- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md),
  [ADR-0004](0004-ledger-api-surface.md): the records this one amends.
- [ADR-0027](0027-authorization-through-platform-grants.md): roles inside an organization.
- Keycloak, managing organizations: https://www.keycloak.org/docs/latest/server_admin/#_managing_organizations
- TigerBeetle ledgers: https://docs.tigerbeetle.com/coding/data-modeling/#ledgers
