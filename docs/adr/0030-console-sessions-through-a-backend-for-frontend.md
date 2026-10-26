# 30. Console sessions through a backend-for-frontend

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

The console is a single-page application that talks only to the platform API, through `/api` on
its own origin (ADR-0014; a Vite proxy in development, a reverse proxy in production). It must
now sign people in with Keycloak (ADR-0026), work inside one organization at a time (ADR-0027),
and hide what the person may not do (ADR-0028).

A browser application can hold OAuth tokens itself or leave them to a server. RFC 10017 (OAuth
2.0 for Browser-Based Applications, BCP 212, August 2026) ranks a browser-only client the most
exposed of its three patterns and says the backend-for-frontend is "strongly recommended for
business applications, sensitive applications". A console that moves money is both.

This record adds sign-in, the organization in the console's URLs and permission-aware pages to
ADR-0014 and ADR-0023; everything else in both stands.

## Decision Drivers

- No token readable by JavaScript.
- No new service: the console already talks to one origin.
- One path to a principal in the API, whether the caller is the console or an integrator.
- CSRF defence that does not rely on one mechanism.

## Considered Options

1. A backend-for-frontend inside the API process.
2. The console as a public OAuth client (PKCE, tokens in memory).
3. A separate backend-for-frontend service.

Option 2 lets any injected script use the tokens. Option 3 adds a container and a hop for the
same result.

## Decision Outcome

Chosen option: 1.

- Routes outside the REST spec, because they are browser redirects: `GET /auth/login`,
  `GET /auth/callback`, `POST /auth/logout`, `GET /auth/session`. The API runs the authorization
  code flow with PKCE as a confidential client (`golang.org/x/oauth2`), asking for the scope
  `organization:*`.
- `sessions` (in the `access` module) holds a hash of the session ID, the principal and the
  tokens encrypted with AES-GCM under `AUTH_SESSION_KEY`. Logging out deletes the row and ends
  the Keycloak session.
- The cookie is `HttpOnly`, `Secure`, `SameSite=Lax`, with the `__Host-` prefix in production.
  The API refreshes the access token before it expires, once per session even under concurrent
  requests. After the cookie is resolved, the request takes the same token check as a bearer
  request.
- CSRF in depth: SameSite keeps the cookie off cross-site posts; a cookie-authenticated request
  that changes state must also carry `Sec-Fetch-Site: same-origin` or an `Origin` equal to
  `AUTH_CONSOLE_ORIGIN`; and request bodies are JSON only (ADR-0032).
- Console: every page moves under `/$org/`; the organization switcher lists the person's
  organizations from the GraphQL `viewer`; every request sends `X-Organization` from the URL; a
  401 sends the browser to `/api/auth/login`; `useCan(permission, ledger)` hides what the person
  may not do (the server still checks); a 403 page; sign-out in the user menu; staff pages under
  `/staff`.

### Consequences

- Good: tokens never reach the browser; a stolen cookie stops working at sign-out.
- Good: integrators and the console share one verification path in the API.
- Good: URLs name the organization, so links are unambiguous for people in several.
- Bad: the API gains session storage and an OAuth client.
- Bad: plain-HTTP development needs `AUTH_COOKIE_SECURE=false`.

## More Information

- `docs/plans/auth/plan.md`, section 6.7.
- [ADR-0014](0014-console-architecture.md), [ADR-0023](0023-console-on-graphql.md): the records
  this one amends.
- RFC 10017: https://www.rfc-editor.org/rfc/rfc10017.html
- Fetch metadata: https://www.w3.org/TR/fetch-metadata/
