# 33. Keycloak's pages through a Keycloakify theme

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

People sign in, set their password, accept invitations and enter a second factor on pages Keycloak
renders, not the console (ADR-0026, ADR-0030). Keycloak's own theme looks like a different
product, and `.design/auth.pen` draws these pages in the console's design system (the "Keycloak:"
frames). The owner suggested Keycloakify for them.

## Decision Drivers

- The pages look like the console: same tokens, type and components.
- They are written in React and TypeScript, as the console is, not in FreeMarker.
- Keycloak stays unmodified: a theme is a provider jar, nothing else changes.
- No machine needs Java to work on the repository.

## Considered Options

1. A Keycloakify 11 login theme in `apps/keycloak-theme`.
2. FreeMarker templates and CSS overriding Keycloak's own theme.
3. A custom sign-in UI in the console with Keycloak's direct grant.

Option 2 keeps the design system in two languages and two styling systems. Option 3 puts
passwords in the console and gives up Keycloak's flows (identity-first, single sign-on, required
actions, second factors), which ADR-0026 relies on.

## Decision Outcome

Chosen option: 1.

- `apps/keycloak-theme` is a Vite+ React app with Keycloakify 11.16.1, theme name `tbp`, a login
  theme only: Keycloak's account console stays its own, and the console's profile links to it.
- `packages/design-tokens` holds the Ledger Instrument tokens; the console and the theme both
  import it, so a token changes in one place.
- The pages `.design/auth.pen` draws are the theme's own (sign-in in one or two steps, password,
  two-factor code and setup, reset, new password, verify email, the account setup and
  invitation pages, errors and expired pages). Every other Keycloak page is Keycloakify's
  default page inside the theme's frame, with Keycloak's class keys mapped to the design's
  classes; Keycloak's CSS is not loaded.
- `keycloakify build` packages the jar with Maven. It runs in the first stage of
  `deploy/compose/keycloak.Dockerfile` (Maven and Node images, pinned by digest); the second stage
  is Keycloak 26.8.1 with the jar in `/opt/keycloak/providers`. `just build` builds the theme's
  bundle only.
- The realm file sets `loginTheme: tbp`. `vp dev` previews any page without Keycloak from a mock
  context: `http://localhost:5174/?page=login-username.ftl`.

### Consequences

- Good: the pages people meet first match the console, and changing them is ordinary React work.
- Good: Keycloak upgrades keep working; Keycloakify targets "all other versions" from 26 on.
- Bad: Keycloak's markup and messages change between versions; Keycloakify absorbs most of it,
  and the theme follows Keycloakify releases.
- Bad: Keycloak's emails (invitation, account setup, reset) keep Keycloak's look until an email
  theme follows; the design draws them too.
- Bad: building the Keycloak image downloads Maven dependencies.

## More Information

- Keycloakify: https://www.keycloakify.dev
- [ADR-0026](0026-identity-from-keycloak-through-openid-connect.md): Keycloak as the identity
  provider.
