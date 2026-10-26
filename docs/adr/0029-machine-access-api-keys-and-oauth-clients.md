# 28. Machine access: API keys and OAuth machine clients

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

Integrators call the API from their own services, on REST, gRPC and, for the platform's own
services, the broker RPC. They need credentials that belong to one organization, that an
organization admin can create and revoke without asking staff, and that work the same on every
transport. Some integrators expect a static API key; others prefer short-lived OAuth tokens.

## Decision Drivers

- Self-service for organization admins, from the console.
- A credential belongs to exactly one organization, so its calls need no `X-Organization` header
  (ADR-0026).
- The same grants as people (ADR-0027): a key that may only post to one ledger gets `operator`
  on it.
- A leaked database must not let anyone use the keys in it.
- A leaked key must be recognisable by secret scanners.

## Considered Options

1. Both: API keys issued by the platform, and OAuth machine clients created in Keycloak through
   the platform.
2. OAuth client credentials only.
3. API keys only.

The owner chose option 1: API keys for integrators who want one secret, OAuth clients for those
whose security policy requires short-lived tokens.

## Decision Outcome

Chosen option: 1.

API keys:

- Format `tbp_<key id>_<secret>`. The prefix is recognisable by secret scanners, the key ID finds
  the row, the secret is 32 random bytes. The full key is shown once, at creation.
- Stored as an HMAC-SHA-256 of the secret under `AUTH_API_KEY_PEPPER`, compared in constant time.
  A slow hash is not needed for 256 bits of randomness, and the pepper means a leaked database
  alone cannot verify guesses.
- Each key has an organization, a name, its grants, an optional expiry, `last_used_at` (written at
  most once a minute) and `revoked_at`. Its principal is (`tbp:api-key`, key ID), so it appears in
  grants, write records and logs like any caller.
- Sent as `Authorization: Bearer tbp_...` on HTTP, the same in gRPC metadata and in the broker
  `authorization` header. A bearer value starting with `tbp_` is an API key; anything else must be
  a JWT from the configured issuer.

Machine clients:

- An organization admin creates one in the console; the API creates a confidential client with a
  service account in Keycloak and records its client ID against the organization in
  `machine_clients`. The platform's own mapping, not Keycloak's organization claim, decides the
  organization of its tokens.
- The client secret is shown once; revoking deletes the Keycloak client and the row.

### Consequences

- Good: integrators choose; both kinds are managed by organization admins without staff.
- Good: both resolve to a principal with grants, so authorization has one path.
- Good: API keys verify without a call to Keycloak.
- Bad: API keys do not expire unless given an expiry; the console shows `last_used_at` so unused
  keys can be revoked.
- Bad: two credential kinds to document on every transport.

## More Information

- `docs/plans/auth/plan.md`, section 6.8.
- [ADR-0026](0026-organizations-as-tenants.md), [ADR-0027](0027-authorization-through-platform-grants.md).
- Keycloak service accounts: https://www.keycloak.org/docs/latest/server_admin/#_service_accounts
- RFC 6750, bearer token usage: https://www.rfc-editor.org/rfc/rfc6750
