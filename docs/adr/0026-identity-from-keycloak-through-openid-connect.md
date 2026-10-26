# 26. Identity from Keycloak through standard OpenID Connect

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

Until now the API had no authentication: `AGENTS.md` kept it out of scope, and ADR-0021 kept
gRPC, AMQP RPC, NATS RPC and the Kafka events development-only for that reason. The platform is
now multi-tenant (ADR-0027) and moves money for several organizations, so every call must say
who makes it.

Identity is the generic part of the problem: user accounts, password hashing, login and logout
pages, password reset and email verification, MFA and passkeys, brute-force protection,
invitations, and single sign-on with a customer's own provider. None of it is about ledgers, all
of it is security-critical, and identity providers have had years of review on each piece. The
question is which provider, and how tightly the API binds to it.

The full comparison, with sources checked on 2026-10-26, is in
`docs/plans/auth/plan.md` sections 3 and 4.

## Decision Drivers

- Do not build identity; build only what is about this platform (roles, ledger access, API keys).
- OpenID certified flows: authorization code with PKCE, client credentials, JWT access tokens
  with a configurable audience.
- A login UI that ships with the provider.
- Configuration as code: the local stack starts with users, clients and organizations seeded
  from a committed file.
- Organizations, invitations and per-organization single sign-on (ADR-0027).
- A licence and footprint that suit a self-hosted Apache-2.0 project.
- The API must not depend on the vendor for verifying tokens, so the choice stays reversible.

## Considered Options

1. Build identity in the platform.
2. Keycloak (26.8.1, Apache-2.0, Java).
3. ZITADEL (v4.20.0, AGPL-3.0 since v3, Go).
4. authentik (2026.8.5, MIT core).
5. Ory Kratos and Hydra (v26.2.0, Apache-2.0).
6. Logto (v1.45.0, MPL-2.0).

Option 1 is months of security-critical work that the platform gets nothing from. ZITADEL is the
lightest real provider, but v4 runs two containers (the server and a Next.js login that needs a
bootstrap token, with reports of that token file not being written), issues weekly releases, has
no DPoP and keeps token exchange in beta. authentik needs 2 CPU and 2 GB and makes client
credentials awkward (no plain client ID and secret); its multi-tenancy is enterprise-only.
Ory ships no login UI and keeps patches between its twice-yearly releases, organizations and
SAML in its enterprise licence. Logto is plausible but less proven.

## Decision Outcome

Chosen option: 2, Keycloak, behind standard OpenID Connect.

- The API is an OAuth 2.0 resource server. `internal/platform/oidc` verifies access tokens with
  `github.com/coreos/go-oidc/v3`: discovery from `AUTH_ISSUER_URL`, the JWKS cached and refetched
  on an unknown key ID, then the signature (an allowlist of asymmetric algorithms), issuer,
  audience (`AUTH_AUDIENCE`), expiry and not-before, with one minute of clock skew. The audience
  check is what stops an ID token, whose audience is the console client, from passing as an
  access token.
- Claims are mapped to a `ledger.Principal` (issuer and subject identify it, never the email).
  Where a provider puts roles is configured (`AUTH_STAFF_CLAIM`, `AUTH_STAFF_VALUE`); the dev
  realm copies realm roles into a flat `roles` claim with a protocol mapper.
- Nothing in token verification names Keycloak. The Keycloak-specific code (organizations,
  invitations, machine clients through the admin API) lives in one adapter,
  `internal/modules/access/infrastructure/keycloak`.
- `internal/platform/oidc/oidctest` is an in-process issuer for tests: an RSA key, an `httptest`
  server for discovery and JWKS, and a helper that mints tokens with any claims. Unit and
  integration tests do not need Keycloak.
- Authentication is always on. There is no switch to turn it off, in any environment.
- The local stack runs `quay.io/keycloak/keycloak` pinned by tag and digest, on the existing
  Postgres server with its own database, seeded with `--import-realm` from
  `deploy/compose/keycloak/realm.json`, plus Mailpit for the emails Keycloak sends.

### Consequences

- Good: login pages, MFA, passkeys, password reset, invitations and SSO come from a mature,
  Apache-2.0 project with the widest standards coverage among the candidates (token exchange,
  DPoP, passkeys), so later needs require no migration.
- Good: the realm file is reviewable in a pull request and identical on every machine.
- Good: tests stay fast and hermetic with `oidctest`.
- Good: switching providers changes the compose service, the realm file and the adapter, not
  the token checks or any use case.
- Bad: Keycloak is the heaviest container in the stack; the sizing guide budgets about 1.25 GB
  per production pod.
- Bad: Keycloak publishes security fixes often; its image pin needs regular bumps.
- Bad: the API cannot start without reaching the issuer for discovery. Afterwards keys are
  cached, so a Keycloak outage blocks sign-in and refreshes, not calls with valid tokens.

## More Information

- `docs/plans/auth/plan.md`, sections 3, 4 and 6.3: the comparison and the verification design.
- [ADR-0027](0027-organizations-as-tenants.md): organizations, which Keycloak owns.
- [ADR-0028](0028-authorization-through-platform-grants.md): what the platform decides once
  the caller is known.
- [ADR-0030](0030-console-sessions-through-a-backend-for-frontend.md): how the console signs in.
- go-oidc: https://github.com/coreos/go-oidc
- Keycloak server administration guide: https://www.keycloak.org/docs/latest/server_admin/
- Keycloak import at startup: https://www.keycloak.org/server/importExport
- ZITADEL licence change: https://zitadel.com/blog/apache-to-agpl
