# Configuring Keycloak for the platform

How to set up the Keycloak realm the platform signs people in with, outside the local stack. The
local stack imports `deploy/compose/keycloak/realm.json`; this guide says what each part of that
file is for and what to change for a real environment. The decisions behind it are ADR-0026
(identity), ADR-0027 (organizations), ADR-0029 (keys and machine clients), ADR-0030 (console
sign-in) and ADR-0033 (the login theme).

## What the platform needs

- Keycloak 26.x with the Organizations feature (the local stack runs 26.8.1).
- One realm for the platform. Every person, organization and machine client of the platform lives
  in it, and nothing else should: the API's admin account can reach everything in the realm (see
  [The admin account](#the-admin-account-tbp-admin)).
- An SMTP server: invitations, account setup and password resets are emails.

## Start from the local realm file

`realm.json` is a working starting point. Import it once (Keycloak imports a realm only when it
does not exist yet), then change:

1. The secrets of `tbp-console` and `tbp-admin`: regenerate them in the admin console, under
   Clients, then Credentials, and give them to the API.
2. The URLs of `tbp-console`: redirect URI `https://<console>/api/auth/callback`, web origin
   `https://<console>`, post-logout redirect `https://<console>/*`, base URL `https://<console>/`.
3. Remove the `tbp-cli` client: it allows the password grant, for the e2e suite and the seed
   script only.
4. Remove the dev users (`staff`, `ada`, `bob`, `carol`, `dave`, `erin`) and the organizations
   `default`, `acme` and `globex`. Create the first staff person by hand (below).
5. The SMTP server (Realm settings, Email) and Keycloak's hostname.

Changes to `realm.json` never reach a realm that already exists. Apply them in the admin console,
as each section below says where.

## Realm settings

| Setting                            | Value      | Why                                                                 |
| ---------------------------------- | ---------- | ------------------------------------------------------------------- |
| Organizations                      | on         | Organizations are the platform's tenants (ADR-0027).                |
| User registration                  | off        | People join when an organization invites them.                      |
| Login with email                   | on         | The sign-in page asks for the email first.                          |
| Forgot password                    | on         | The reset email.                                                    |
| Brute force detection              | on         |                                                                     |
| Access token lifespan              | 5 minutes  | The API checks tokens on every call; short tokens bound a leak.     |
| SSO session idle                   | 30 minutes | The console's session ends after half an hour without use.          |
| Login theme                        | `tbp`      | The platform's pages (ADR-0033). The image must carry the theme.    |
| User profile, `lastName`, Required | off        | An invitation may give only a first name; nobody is asked for more. |

The login theme is the jar `pnpm --filter keycloak-theme build` writes
(`apps/keycloak-theme/dist_keycloak/tbp-keycloak-theme.jar`); copy it to `/opt/keycloak/providers/`
and run `kc.sh build`, as `deploy/compose/keycloak.Dockerfile` does. Keycloak's emails keep
Keycloak's look until the platform has an email theme.

The user profile setting is under Realm settings, User profile, `lastName`: turn "Required field"
off. In `realm.json` it is the `components` entry `declarative-user-profile`, whose configuration
Keycloak stores as one JSON string.

## Staff

Staff run the platform: they create and suspend organizations and hold no permission inside one
without a grant (ADR-0028).

- The realm role `tbp-staff` marks them. Give it to a person under Users, Role mapping.
- The console's tokens carry realm roles in the `roles` claim (the mapper `realm roles as roles`
  on `tbp-console`). The API reads it through `AUTH_STAFF_CLAIM` (`roles`) and `AUTH_STAFF_VALUE`
  (`tbp-staff`); change both if your realm names them differently.

## Clients

### `tbp-api`

The audience of every access token (`AUTH_AUDIENCE`). Bearer-only: it signs no one in and has no
secret to keep.

### `tbp-console`

The console's sign-in, run by the API (ADR-0030).

- Confidential, standard flow only, PKCE with S256.
- Mappers: an audience mapper that adds `tbp-api` to access tokens, and the `roles` mapper above.
- The built-in `organization` client scope: the console asks for `organization:*`, and the token's
  `organization` claim lists the person's organization aliases (`AUTH_ORGANIZATION_CLAIM`).
- The API's settings: `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`, `AUTH_REDIRECT_URL` (the redirect
  URI above), `AUTH_CONSOLE_ORIGIN`, and `AUTH_SESSION_KEY` (32 random bytes in base64,
  `openssl rand -base64 32`).
- Account setup emails send people back to this client's base URL.

### `tbp-admin`

The service account the API calls Keycloak's admin API with (`AUTH_ADMIN_CLIENT_ID`,
`AUTH_ADMIN_CLIENT_SECRET`). Without it, organizations, members, invitations and machine clients
answer 503; the rest of the API runs. It needs service accounts on and these roles of the client
`realm-management`:

| Role                                              | What the API does with it                                                       |
| ------------------------------------------------- | ------------------------------------------------------------------------------- |
| `manage-organizations`, `view-organizations`      | Creates and renames organizations, adds and removes members, sends invitations. |
| `manage-users`, `view-users`, `query-users`       | Creates the account of a new invitee and sends the account setup email.         |
| `manage-clients`, `view-clients`, `query-clients` | Creates machine clients, reads and regenerates their secrets, deletes them.     |

### Machine clients

The API creates one confidential client per machine client an organization admin adds (ADR-0029):
client ID `<organization alias>-<name>`, service accounts on, client credentials only, an
audience mapper for `tbp-api`, and access tokens of 5 minutes. Do not edit them in the admin
console: the platform's record and Keycloak's must stay alike. Delete them from the console, which
removes both.

## The admin account (`tbp-admin`)

The roles above are realm-wide. `tbp-admin` can therefore read and change every user and every
client of the realm, not only those the platform created, including `tbp-console` and itself. Its
secret is as sensitive as the realm's admin password. Today the platform contains it this way:

- A realm for the platform alone, so "every user and client of the realm" is exactly the
  platform's.
- The secret in a secret manager, given only to the API, and rotated: regenerate it under
  Clients, `tbp-admin`, Credentials, then deploy it. The API reads it at start-up.
- Admin events on (Realm settings, Events, Admin events settings, "Save events" and "Include
  representation"), so every change `tbp-admin` makes is recorded.
- Keycloak's admin endpoints reachable only from the API's network where your setup allows it.

Narrowing `tbp-admin` itself needs Keycloak's fine-grained admin permissions (version 2, Keycloak
26.2 and later), and has not been tried yet. The likely shape: replace `manage-clients` with
`create-client` and let the API grant `tbp-admin` the right to manage each client it creates, and
limit user management to the people of the platform's organizations. Until that is done and
tested, leave the roles as listed; without one of them, the calls that need it fail.

## The API's settings, in one place

| Setting                                            | Value                                                                           |
| -------------------------------------------------- | ------------------------------------------------------------------------------- |
| `AUTH_ISSUER_URL`                                  | `https://<keycloak>/realms/<realm>`, as tokens name the issuer                  |
| `AUTH_DISCOVERY_URL`                               | The same realm at the address the API reaches it, when that differs; else empty |
| `AUTH_AUDIENCE`                                    | `tbp-api`                                                                       |
| `AUTH_STAFF_CLAIM`, `AUTH_STAFF_VALUE`             | `roles`, `tbp-staff`                                                            |
| `AUTH_ORGANIZATION_CLAIM`                          | `organization`                                                                  |
| `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`             | `tbp-console` and its secret                                                    |
| `AUTH_REDIRECT_URL`, `AUTH_CONSOLE_ORIGIN`         | `https://<console>/api/auth/callback`, `https://<console>`                      |
| `AUTH_SESSION_KEY`                                 | 32 random bytes in base64; a comma-separated list during a rotation             |
| `AUTH_ADMIN_CLIENT_ID`, `AUTH_ADMIN_CLIENT_SECRET` | `tbp-admin` and its secret                                                      |
| `AUTH_API_KEY_PEPPER`                              | 32 random bytes in base64; changing it invalidates every API key                |

`/readyz` reports Keycloak as an optional dependency: when it does not answer, tokens already
issued still verify, but nobody can sign in.
