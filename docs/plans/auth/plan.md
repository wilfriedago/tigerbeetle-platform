# Plan: authentication, authorization and multi-tenancy

Date: 2026-10-26. Written against `feat/console` at 5667331.
Status: approved by the owner on 2026-10-26, with the answers of section 12; the assumptions O1
to O5 and the recommendation of O6 stand. ADR-0026 to ADR-0032 record the decisions (phase 0).

How to read it. Section 1 is the design in one page. Section 2 lists the facts in the code that
the design depends on. Sections 3 and 4 answer "build or buy" and "which provider". Section 5 is
tenancy, sections 6 to 8 the rest of the design. Sections 9 and 10 are the delivery: ADRs, then
phases. Section 11 lists the risks, section 12 the owner's answers and the open questions. Short
"Go note" paragraphs explain choices that are not obvious to someone new to Go.

Words used throughout:

- authentication (authn): proving who is calling. Answers 401 when it fails.
- authorization (authz): deciding what that caller may do. Answers 403, or 404 (section 6.6).
- identity provider (IdP): the service that owns users, passwords, MFA and login pages, and issues
  tokens. Keycloak is the IdP here.
- organization: a tenant. It owns ledgers, members, API keys and machine clients. Keycloak calls
  it an organization too.
- principal: the caller as the API sees it after authentication: an issuer and a subject.
- staff: the people who run the platform itself, as opposed to members of an organization.
- grant: "this principal holds this role in this organization, or on this ledger", stored by the
  platform.

## 1. The design in one page

1. Do not build identity. Users, passwords, MFA, passkeys, login pages, password reset and
   federation belong to Keycloak, the reference IdP (section 4).
2. Multi-tenant from day 1. Keycloak Organizations own the organizations and their membership,
   including invitations and single sign-on per organization. The platform keeps a small copy of
   each organization so its tables can point at it, and owns everything inside: ledgers, roles,
   API keys.
3. Do build authorization. Who may do what in an organization and on a ledger is a business rule
   of this platform. It lives in a new `access` module and is enforced in each module's
   application layer, so every transport gets the same answer without repeating the check.
4. The tenant comes from the credential: an API key or a machine client belongs to one
   organization. A person names the organization in an `X-Organization` header (the console sends
   it from its URL), and the API checks the person is a member. REST paths, protos and GraphQL
   arguments do not change.
5. Ledger names become unique per organization instead of globally. Ledgers gain an internal ID,
   and every table that points at a ledger uses it (section 5.5).
6. The console signs in through a backend-for-frontend (BFF) inside the API process: the browser
   holds an HttpOnly session cookie, never a token.
7. Machine access two ways: API keys issued by the platform per organization, and OAuth 2.0 client
   credentials from Keycloak. Both work on every transport.
8. Staff create organizations (no public sign-up yet) and manage them, but cannot read a tenant's
   ledgers unless that organization grants them a role.
9. Authentication is always on. There is no switch to turn it off; tests use an in-process issuer.
10. Two existing behaviours become security bugs once callers are told apart, and are fixed
    (section 6.9): idempotency keys shared by every caller of a ledger, and request bodies read as
    JSON whatever their type.

## 2. What exists today

Facts re-checked in the code; the design depends on each one.

| #   | Fact                                                                                                                                                                                                                                                              | Where                                                                                                    |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| F1  | Six entry points: HTTP (REST and `POST /graphql` on one ServeMux behind one middleware chain), gRPC (interceptor chain), AMQP RPC and NATS RPC (one dispatcher), and two internal workers (reconciler, CDC relay). Kafka is outbound only.                        | `internal/bootstrap/servers.go`; `platform/restapi/middleware.go`; `platform/grpcserver/server.go:38-44` |
| F2  | The broker RPC already carries per-message headers: request ID, idempotency key, deadline. Token and organization headers fit the same pattern.                                                                                                                   | `platform/amqprpc/client.go:93-103`; `platform/natsrpc/client.go:44-50`                                  |
| F3  | Every use case is an interface in `application/usecases.go`, and almost every method takes the ledger name. The exceptions are `LedgerQueries.List`, `LedgerCommands.Create` and the reconciler status.                                                           | `modules/*/application/usecases.go`                                                                      |
| F4  | Six kernel sentinel errors and seven problem kinds; none means 401 or 403.                                                                                                                                                                                        | `kernel/ledger/errors.go`; `platform/problem/problem.go:21-27`                                           |
| F5  | Idempotency records are keyed by `(ledger, idempotency_key)`. A caller who reuses another caller's key in the same ledger gets that caller's stored answer.                                                                                                       | `migrations/00006_create_write_requests.sql`                                                             |
| F6  | `jsonBodies` rewrites any non-JSON `Content-Type` to `application/json` in front of the whole mux, `/graphql` included. With cookie sessions, a cross-site `text/plain` form post would be read as JSON.                                                          | `platform/restapi/middleware.go`; gqlgen `transport/http_post.go:41`                                     |
| F7  | The console is a SPA. In development Vite proxies `/api` to the API, so the browser sees one origin; production assumes a reverse proxy doing the same.                                                                                                           | `apps/console/vite.config.ts:25-28`                                                                      |
| F8  | gRPC, AMQP RPC, NATS RPC and Kafka events are off by default and development-only "until authentication is decided".                                                                                                                                              | `AGENTS.md`; ADR-0021                                                                                    |
| F9  | The ledger name is the primary key of `ledgers`, and eight tables point at it by name (`ledger_assets`, `account_rules`, `event_types`, `addresses`, `accounts`, `write_requests`, `transactions`, `holds`), through about 38 query conditions in the sqlc files. | `migrations/00001` to `00008`; `modules/*/infrastructure/postgres/queries`                               |
| F10 | TigerBeetle ledger numbers are allocated per (ledger, asset) from one identity sequence, unique across the cluster. Transfers cannot cross TigerBeetle ledgers, so tenants sharing the cluster cannot move money between each other.                              | `migrations/00002_create_ledger_assets.sql:10-14`                                                        |
| F11 | The console docs already list "login, members and roles, API keys, audit log" and an organization switcher as the work that follows this decision.                                                                                                                | `docs/console.md:21-23,320`                                                                              |

## 3. Build or buy

### 3.1 What "build it ourselves" means

| Piece                                  | What it takes                                            | Risk if done wrong                      |
| -------------------------------------- | -------------------------------------------------------- | --------------------------------------- |
| User store and password hashing        | argon2id parameters, rehash on login, breach checks      | Credential theft                        |
| Login, logout, sessions                | Pages, cookies, CSRF, session fixation, revocation       | Account takeover                        |
| Password reset and email verification  | An email service, signed single-use links, expiry        | Account takeover through reset          |
| MFA and passkeys                       | TOTP with recovery codes, WebAuthn ceremonies            | Bypassable second factor                |
| Brute-force and enumeration protection | Rate limits, lockout, constant-time answers              | Credential stuffing                     |
| Invitations                            | Emails, single-use links, accepting into an organization | Joining the wrong tenant                |
| Single sign-on per organization        | OIDC and SAML federation, routing by email domain        | Cannot serve companies that require SSO |
| Admin UI                               | Users, resets, sessions, MFA resets                      | Operators editing the database by hand  |

Every row is generic and an IdP has had years of security review on each. The platform builds
only what is about its own domain: roles, ledger access and API keys (section 6.8).

### 3.2 Who owns what

| Concern                                        | Owner                                  | Why                                                                                                                                                       |
| ---------------------------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| People, credentials, MFA, login pages          | Keycloak                               | Section 3.1                                                                                                                                               |
| Organizations and membership, invitations, SSO | Keycloak Organizations                 | Owner's choice (section 12). Invitations and per-organization SSO come with it.                                                                           |
| Staff                                          | Keycloak realm role                    | Staff can always be restored through Keycloak, even if every row in Postgres is wrong                                                                     |
| Roles in an organization and on a ledger       | Platform (`access`)                    | They change with the domain: a new ledger, a new grant. Keycloak organizations have no roles, and per-ledger roles in tokens would grow with every ledger |
| API keys                                       | Platform (`access`)                    | Self-service for organization admins, bound to one organization                                                                                           |
| Machine clients (OAuth)                        | Keycloak, created through the platform | Organization admins create them in the console; the platform records which organization each one belongs to                                               |

The API verifies tokens with standard OpenID Connect only. The Keycloak-specific code (creating
organizations, inviting members, creating machine clients) sits behind one port in
`access/infrastructure/keycloak`, so it is the only place that names the vendor.

## 4. Which identity provider

Criteria, in order: OpenID certified; JWT access tokens with a configurable audience; client
credentials for machines; a login UI that ships with it; configuration as code, so the local
stack starts with users and clients already seeded; Postgres as its store; a licence and
footprint that suit a self-hosted Apache-2.0 project; maturity.

### 4.1 Comparison

Checked on 2026-10-26 against each project's releases and documentation.

| IdP                  | Latest, licence                                                                                                                                                     | Stack, footprint                                                                               | Machine clients, tokens                                                                                                                                                     | Dev seeding, login UI                                                                                                                                                                   | Fit                                                                                                      |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Keycloak (chosen)    | 26.8.1, Apache-2.0. A minor release about every quarter                                                                                                             | Java, Postgres. The heaviest option: the sizing guide budgets about 1.25 GB per production pod | JWT access tokens; audience through a mapper; client credentials through service accounts; token exchange (RFC 8693) on by default since 26.2; DPoP and passkeys since 26.4 | One container. `--import-realm` loads a realm JSON file committed in the repo and skips a realm that already exists. Login UI built in. Terraform provider. Organizations GA since 26.0 | Best standards coverage, simplest reproducible seed, licence matches the project, organizations built in |
| ZITADEL              | v4.20.0, AGPL-3.0 since v3 (May 2025); unmodified use as a separate service carries no obligation. A release about every week                                       | Go, Postgres only. 512 MB is enough for test                                                   | Private_key_jwt, client credentials or personal access tokens; JWT or opaque per app; token exchange in beta; no DPoP                                                       | Two containers in v4: the server and a Next.js login that needs a bootstrap token, with reports of that token file not being written                                                    | Lightest real IdP with native organizations; more moving parts                                           |
| authentik            | 2026.8.5, MIT core with an enterprise directory                                                                                                                     | Python and Go, Postgres. Needs 2 CPU and 2 GB                                                  | Client credentials need a service account with an app password, not a plain ID and secret. Multi-tenancy is enterprise-only and alpha                                       | Blueprints, Terraform provider, login UI                                                                                                                                                | Weak on machine clients and tenancy                                                                      |
| Ory Kratos and Hydra | v26.2.0, Apache-2.0, but patches between the twice-yearly releases, organizations and SAML are enterprise-only                                                      | Go, small images                                                                               | Opaque tokens by default; no DPoP                                                                                                                                           | No login UI: we would write the login and consent app                                                                                                                                   | Rejected: we would build what section 3 says not to                                                      |
| Logto                | v1.45.0, MPL-2.0                                                                                                                                                    | TypeScript, Postgres                                                                           | M2M apps; JWT only when a resource is requested                                                                                                                             | Login UI; no official Terraform provider                                                                                                                                                | Plausible, less proven                                                                                   |
| Smaller ones         | Dex (federation broker), Authelia (OIDC in open beta), Pocket ID (passkeys only), Rauthy (pre-1.0), Casdoor (a 9.1 SAML signature bypass and an MFA bypass in 2026) | -                                                                                              | -                                                                                                                                                                           | -                                                                                                                                                                                       | Not as the reference IdP                                                                                 |

### 4.2 Keycloak, and what it costs

Keycloak is the owner's choice and the recommendation: one container seeded from a committed
realm file, Apache-2.0 like the project, the widest standards coverage, and organizations with
invitations and per-organization identity providers built in. The cost is memory: it is the
heaviest container in the stack.

### 4.3 Keycloak Organizations: the facts the design relies on

Checked on 2026-10-26 in the 26.8 admin guide, release notes, Admin REST API reference and the
26.8.1 source. Unverified points are marked.

| Topic                | Fact                                                                                                                                                                                                                                                                                                                            | Consequence here                                                                                                                 |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Seeding              | `organizationsEnabled: true` and an `organizations[]` array in the realm JSON create organizations, their identity-provider links and members (by username) at import.                                                                                                                                                          | The dev realm file creates `default`, `acme` and `globex` with their people.                                                     |
| Alias                | Unique, URL-friendly, defaults to the name, and cannot be changed afterwards. The name can change.                                                                                                                                                                                                                              | The alias is the stable link between Keycloak and the platform, and the value of `X-Organization`.                               |
| Requesting the claim | Scope `organization` selects one organization, and a person in several is asked to pick one at login. `organization:<alias>` asks for named ones. `organization:*` returns all of them. Mixing forms is an error.                                                                                                               | The BFF and people's tokens ask for `organization:*`: no prompt at login, and the header picks the organization per call.        |
| Claim shape          | By default a list of aliases, `["acme","globex"]`, in the access token, the ID token and introspection. Mapper options turn it into a map keyed by alias with the ID, attributes or domains. A person in no organization gets no claim at all (issue 50410).                                                                    | The API reads the default list; a missing claim means no organizations (staff are often in none).                                |
| Members              | Managed members are deleted with their organization or membership; unmanaged members are realm users that survive. Adding an existing user makes an unmanaged member.                                                                                                                                                           | Every membership is unmanaged, so removing someone from one organization never deletes their account or their other memberships. |
| Invitations          | `POST .../members/invite-user` (by email) and `.../invite-existing-user` (by user ID). SMTP is required; a failed send is a 500. Since 26.5 invitations are stored and can be listed, resent and deleted. A person with no account gets a registration link; whether realm self-registration must be on for that is unverified. | Mailpit in the local stack; the members page lists pending invitations from Keycloak; question O6.                               |
| Roles                | No native organization roles (issue 50606). Organization groups (26.6) can carry roles (26.7), which appear per organization inside the claim.                                                                                                                                                                                  | Roles stay in the platform (3.2). Per-ledger roles as groups would grow every token with every ledger.                           |
| Service accounts     | Nothing in the code prevents a service account from being a member, and its token should carry the claim with `organization:<alias>` (read in the source, not run).                                                                                                                                                             | Not relied on: the platform maps each machine client to its organization itself (`machine_clients`).                             |
| Admin permissions    | Creating organizations needs `manage-organizations` (26.7); adding a member also needs manage on that user, so `manage-users`; listing members needs view or query on users. Fine-grained admin permissions v2 can narrow both.                                                                                                 | The API's admin service account gets exactly these, narrowed with v2 permissions where possible (section 11).                    |
| Single sign-on       | An identity provider can be linked to several organizations (26.8), with redirect by email domain configured on the domain. Once an organization exists, the login page asks for the username or email first.                                                                                                                   | Per-organization SSO is configuration, not code. The login page changes for everyone (identity-first).                           |
| Known issues         | One extra query per user when listing users with organizations on (46681); `organization:<alias>` ignored when the preview parameterized-scopes feature is on (54090, opened 2026-10-19). No documented cap on organizations per realm (unverified).                                                                            | Leave that preview feature off; keep an eye on member listing performance.                                                       |

## 5. Tenancy

### 5.1 Organizations

- Keycloak owns each organization: its name, alias, email domains, identity providers and members.
- The platform keeps `organizations`: its own ID (the primary key the platform's tables point
  at), the alias (unique and immutable in Keycloak, so the link between the two), the Keycloak
  organization ID once known, the name, a status `active` or `suspended`, and when and by whom it
  was created. It is a copy of what Keycloak holds plus the status the platform enforces; the
  platform never stores membership in it.
- Every membership is unmanaged in Keycloak's terms (4.3): removing a person from one organization
  never deletes their account or their other memberships. The platform never deletes a Keycloak
  organization; staff suspend instead.
- Creating an organization is one staff action in the platform: the API creates it in Keycloak
  through the admin API, records the copy, and invites the first owner by email (section 5.3). If
  a step fails, retrying with the same alias finishes the job instead of creating a second one.
- Suspending an organization is a platform status: every call in its name answers 403
  `organization-suspended`, without touching Keycloak.
- No public sign-up yet (owner's answer). Self-service creation can be added later as one more
  path to the same staff action.

### 5.2 Which organization a call is for

| Credential                          | Organization                                                                                                                                                                                                                                   |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| API key                             | The organization the key belongs to                                                                                                                                                                                                            |
| Machine client (client credentials) | The organization the platform recorded when the client was created, looked up by the token's client ID                                                                                                                                         |
| Person (console session or bearer)  | The `X-Organization` header (the alias). The API checks it against the token's `organization` claim, requested with the scope `organization:*` so it lists every membership (4.3); a non-member gets 404, as if the organization did not exist |

An API key or machine client may send `X-Organization` too; if it names another organization the
call gets 403. The header travels like the request ID: an HTTP header for REST and GraphQL,
`x-organization` gRPC metadata, an `organization` message header on the brokers.

Calls that are not about one organization take no header: health, the staff organization
operations, the caller's own profile (`viewer`, which lists their organizations), and the sign-in
routes.

Removing a person from an organization in Keycloak takes effect when their access token expires,
at most five minutes later. That is the trade-off for checking membership from the token instead
of calling Keycloak on every request.

### 5.3 Members and invitations

- An organization admin invites a person by email from the console. The API asks Keycloak to send
  the invitation and records the roles the person will get (a pending grant keyed by the email).
  How a person without an account gets one is question O6.
- When the person accepts and signs in, the token shows the membership; the API binds the
  pending grants to the person's subject, but only when the token's email is verified.
- Removing a member removes them from the Keycloak organization and deletes their grants in that
  organization.
- The members page lists Keycloak's members of the organization with the platform's roles next to
  each one, and the pending invitations, which Keycloak stores and can resend or delete (26.5).
- Keycloak needs SMTP to send invitations. The local stack gains Mailpit, which catches every
  email and shows it in a web page.

### 5.4 Staff

Staff hold a Keycloak realm role (`tbp-staff`, read from a configured claim). They may create,
rename, suspend and inspect organizations and invite their first owner. They hold no permission
inside an organization: to read a tenant's ledgers, that organization grants them a role like
anyone else, and the grant shows in its member list.

### 5.5 Ledgers inside an organization

- Deviation, decided in phase 1 (ADR-0027): a ledger is (organization, name). `ledgers` and the
  eight tables of F9 gain `organization_id`, and their primary, unique and foreign keys lead with
  it. The repositories read the tenant from the context and every query filters by it; use cases
  keep their ledger-name signatures. The first version of this section resolved the name to a
  platform-wide ledger ID instead, which would have changed about 350 signatures in the
  application and domain layers.
- `apps/api/migrations/tenancy_test.go` fails when a query touches a tenant table without
  `organization_id`, unless a `-- tenant-free:` comment says why it may cross organizations.
- TigerBeetle is unchanged: its ledger numbers are already unique across the cluster (F10).
- One migration does it. Pre-alpha data moves into an organization with the alias `default`,
  which the dev seed also creates in Keycloak.

Go note: the tenant travels in the `context.Context`, so repository signatures did not change;
`platformpg.TenantOf(ctx)` reads it at the top of each repository method and fails when it is
missing.

### 5.6 Events and the brokers

- Ledger events on Kafka gain the organization's ID and alias (the format only gains fields under
  `v1`). The topic stays internal: only the platform's own consumers read it. Delivering events
  to tenants (webhooks or a topic per organization) is later work.
- AMQP RPC and NATS RPC stay for the platform's own services, authenticated like any caller, not
  for tenants (assumed, question O4). Tenants use REST, GraphQL and gRPC.

## 6. Authentication and authorization

### 6.1 The principal and the tenant

The kernel gains `ledger.Principal` (issuer, subject, kind: person, machine client, API key or
system; a display name for logs, never used in decisions; staff or not) and `ledger.Tenant` (the
organization ID and alias). Both travel in the `context.Context` of the request. A principal is
identified by issuer and subject together, never by email.

`ledger.System` is the principal of the reconciler and the CDC relay; it acts across tenants and
can only be set by `internal/bootstrap`.

Go note: a context value is the standard way to carry request-scoped data such as the caller
across layers without adding a parameter to every function. The key is an unexported type, so no
other package can overwrite it.

### 6.2 Authentication per entry point

| Entry point              | Credential                                                                              | Where it is checked                                                                 |
| ------------------------ | --------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| REST and GraphQL         | `Authorization: Bearer <token or API key>`, or the console session cookie (section 6.7) | A middleware in `platform/restapi`, after the request ID, so a 401 still carries it |
| gRPC                     | `authorization: Bearer <token or API key>` metadata                                     | Unary and stream interceptors in `platform/grpcserver`                              |
| AMQP RPC and NATS RPC    | An `authorization` message header                                                       | `rpc.Dispatcher`, before routing, so both brokers behave the same                   |
| Reconciler and CDC relay | None: they run as `ledger.System`                                                       | `internal/bootstrap`                                                                |
| Kafka ledger events      | Kafka's own client authentication and ACLs                                              | The broker; the topic is internal (5.6)                                             |

A bearer value starting with `tbp_` is an API key (section 6.8); anything else must be a JWT from
the configured issuer. After authentication the tenant is resolved (5.2) and both go into the
context.

### 6.3 Token verification

`internal/platform/oidc` verifies access tokens with `github.com/coreos/go-oidc/v3` (v3.21.0):
discovery from the issuer URL; the JWKS fetched, cached and refetched when an unknown key ID
appears; then the signature (an allowlist of asymmetric algorithms), issuer, audience, expiry and
not-before, with one minute of clock skew. go-oidc verifies any signed JWT, not only ID tokens; the
audience check is what stops an ID token, whose audience is the console client, from passing as
an access token. The BFF uses `golang.org/x/oauth2` (v0.37.0, Go 1.26 or later; the API is on
1.27.3) for the authorization code flow and refresh.

`internal/platform/oidc/oidctest` is an in-process issuer for tests: an RSA key, an `httptest`
server for discovery and JWKS, and a helper that mints tokens with any claims, organization
claims included. Unit and integration tests never need Keycloak; only the compose stack and one
smoke check use it.

Go note: a JWT is a signed JSON document. Verifying it means checking the signature with the IdP's
published public key, then the claims, so no call to Keycloak is needed per request.

### 6.4 Roles and permissions

Permissions are what use cases check. Roles are named sets of permissions. Both are defined in
the kernel, so every module and the access module agree on them without importing each other.

| Permission               | What it allows                                                                                                              |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------- |
| `ledger.read`            | Every query on a ledger: the ledger, settings, accounts, balances, history, transactions, holds, requests, overview, search |
| `money.write`            | Create and revert transactions; create, capture and void holds                                                              |
| `metadata.write`         | Replace the metadata of an account or a transaction                                                                         |
| `ledger.configure`       | Replace the ledger's metadata; put and delete rules; create event types                                                     |
| `ledger.members`         | Grant and revoke roles on the ledger                                                                                        |
| `org.ledgers`            | Create ledgers in the organization (the creator becomes the ledger's admin)                                                 |
| `org.members`            | Invite and remove members; grant organization roles                                                                         |
| `org.credentials`        | Create and revoke API keys and machine clients                                                                              |
| `org.operate`            | Reconciler status and service details for the organization                                                                  |
| `platform.organizations` | Create, rename, suspend and inspect organizations (staff only)                                                              |

| Role       | Scope        | Permissions                                                                                                                      |
| ---------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| `viewer`   | One ledger   | `ledger.read`                                                                                                                    |
| `operator` | One ledger   | viewer + `money.write`, `metadata.write`                                                                                         |
| `admin`    | One ledger   | operator + `ledger.configure`, `ledger.members`                                                                                  |
| `member`   | Organization | Sees the organization; ledger access only through ledger roles                                                                   |
| `admin`    | Organization | Every ledger permission on every ledger of the organization, plus `org.ledgers`, `org.members`, `org.credentials`, `org.operate` |
| `owner`    | Organization | admin, and the only role that can grant or remove `owner`; an organization always keeps at least one                             |
| staff      | Platform     | `platform.organizations` only (5.4)                                                                                              |

API keys and machine clients get grants exactly like people. An integrator that may only post
transactions to one ledger gets `operator` on it.

Finer scopes, such as "may only move money from `merchants:123:*`", are not in this plan. The
`Authorizer` port of section 6.5 is where a policy engine would plug in, without touching the use
cases:

| Engine                      | Form                                 | Verdict for now                                                                        |
| --------------------------- | ------------------------------------ | -------------------------------------------------------------------------------------- |
| Grants in Postgres (chosen) | Our code, one indexed query          | Fixed roles per organization and ledger are exactly what it does well                  |
| cedar-go v1.8.0             | Embedded library, two dependencies   | The first candidate when attribute conditions appear (address prefixes, amount limits) |
| Casbin v3                   | Embedded library                     | RBAC with domains fits, but adds a policy file format for what a table already does    |
| OPA                         | Embedded or sidecar, 55 dependencies | Worth it only for one policy language across infrastructure and the app                |
| OpenFGA, SpiceDB            | Separate services                    | Pay off with relationships and inheritance; a service too many for these roles         |

### 6.5 Enforcement

- The `access` module owns `organizations` (5.1), `grants` (organization, issuer, subject, scope:
  the organization or one ledger ID, role, granted by and at), `pending_grants` (5.3), `api_keys`
  (6.8), `machine_clients` (5.2) and `sessions` (6.7). It implements `Authorizer`:
  `Require(ctx, permission, ledgerName) error`, and `Ledgers(ctx)` for list filtering. One indexed
  query per check; no cache in the first version, because a revoked grant must stop working at
  once.
- Each module declares the `Authorizer` port in its `application/ports.go`, as it already does
  for what it needs from another module, and wraps its own use cases in
  `application/authorize.go`: one small type per use-case interface whose methods call `Require`
  and then the real use case. The module's constructor returns only the wrapped values, so no
  transport can reach an unchecked use case.
- A test per module walks every method of every exported use-case interface, by reflection, and
  fails if one is missing from the permission table. A new use case without a permission cannot
  be merged.
- `LedgerQueries.List` filters by the caller's grants in SQL; it does not filter a page after
  reading it, which would break the cursor.

Go note: wrapping an interface in another type with the same methods is the decorator pattern. Go
has no annotations, so this is the idiomatic replacement for `@PreAuthorize` in Spring: the check
is plain code, visible in one file per module.

### 6.6 Errors on every transport

The kernel gains `ErrUnauthenticated` and `ErrForbidden`, and `platform/problem` the kinds
`UNAUTHENTICATED` and `FORBIDDEN`, rendered by the existing per-transport mapping (ADR-0021).

| Kind              | REST                                                                                          | GraphQL                                                                           | gRPC                                                    | AMQP and NATS RPC                    |
| ----------------- | --------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------- | ------------------------------------ |
| `UNAUTHENTICATED` | 401, `/problems/unauthenticated`, `WWW-Authenticate: Bearer error="invalid_token"` (RFC 6750) | 401 from the HTTP middleware: no request reaches the executor without a principal | `Unauthenticated`, `ErrorInfo` reason `UNAUTHENTICATED` | `ok: false`, `code: UNAUTHENTICATED` |
| `FORBIDDEN`       | 403, `/problems/forbidden` (and `/problems/organization-suspended`)                           | An error with `extensions.code: FORBIDDEN`, `status: 403`                         | `PermissionDenied`, `ErrorInfo` reason `FORBIDDEN`      | `ok: false`, `code: FORBIDDEN`       |

Not found or forbidden: a caller with no grant at all on a ledger, or who is not a member of the
named organization, gets 404, the same answer as a ledger or organization that does not exist, so
names are not disclosed across tenants. A caller who can see the ledger but lacks the permission
gets 403.

### 6.7 Console sign-in: a backend-for-frontend in the API process

| Option                                          | How it works                                                                                                                                        | Verdict                                                                                                                                                                                                                                                                                   |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SPA as a public client (PKCE, tokens in memory) | The console runs the OAuth flow itself and sends bearer tokens.                                                                                     | Rejected: any script injected into the page can use the tokens. RFC 10017 (OAuth 2.0 for Browser-Based Applications, BCP 212, August 2026) ranks it the most exposed of its three patterns, and says the BFF is "strongly recommended for business applications, sensitive applications". |
| BFF in the API process (chosen)                 | The API runs the authorization code flow with PKCE as a confidential client, keeps the tokens server-side and gives the browser an HttpOnly cookie. | Tokens never reach JavaScript. No new service, and the console already talks to one origin (F7).                                                                                                                                                                                          |
| BFF as a separate service                       | A small gateway in front of the API.                                                                                                                | Rejected for now: another container and another hop for the same result.                                                                                                                                                                                                                  |

- Routes outside the REST spec, because they are browser redirects, not API operations:
  `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout`, `GET /auth/session`.
- `sessions` holds a hash of the session ID, the principal, and the tokens encrypted with AES-GCM
  under `AUTH_SESSION_KEY`. Logging out deletes the row and ends the Keycloak session, so a stolen
  cookie stops working at once.
- Cookie: `HttpOnly`, `Secure`, `SameSite=Lax`, the `__Host-` prefix in production. The API
  refreshes the access token when it is about to expire, once per session even under concurrent
  requests. After an invitation is accepted, the next refresh carries the new membership.
- CSRF, in depth: SameSite already keeps the cookie off cross-site posts. A cookie-authenticated
  request that changes state must also carry `Sec-Fetch-Site: same-origin` or an `Origin` equal to
  `AUTH_CONSOLE_ORIGIN`, and F6 is fixed (6.9).
- After the cookie is resolved, the request goes through the same token check as a bearer request:
  one path to a principal.

### 6.8 API keys

- Format `tbp_<key id>_<secret>`: the prefix lets secret scanners recognise a leaked key, the key ID
  finds the row, the secret is 32 random bytes. Shown once, at creation.
- Stored as an HMAC-SHA-256 of the secret under `AUTH_API_KEY_PEPPER`, compared in constant time. A
  slow hash such as argon2 is not needed for 256 bits of randomness, and the pepper means a leaked
  database alone cannot verify guesses.
- Each key belongs to one organization, has a name, the grants it is given, an optional expiry,
  `last_used_at` (written at most once a minute) and `revoked_at`.
- Organization admins create and revoke keys in the console; the key's principal is
  (`tbp:api-key`, key ID), so it appears in grants, write records and logs like any caller.

Machine clients are the OAuth alternative for integrators who prefer short-lived tokens: an
organization admin creates one in the console, the API creates a confidential client with a
service account in Keycloak, and records the client ID against the organization in
`machine_clients`. Its tokens then resolve to that organization (5.2).

### 6.9 Two fixes that auth makes necessary

1. Idempotency keys (F5). Write records gain the actor (issuer and subject). A key replays only for
   the principal that created it. Another principal reusing the key gets 422
   `idempotency-key-reused`, without the stored answer. The key stays unique per ledger, so
   `/requests/{key}` and the console's search by key keep working. Amends ADR-0007.
2. JSON coercion (F6). `jsonBodies` goes. A body with a non-JSON `Content-Type` gets 415, Huma's
   default. This is the "new API minor version" ADR-0018 deferred strictness to; "Copy as curl"
   already sends the header.

### 6.10 Audit

Write records keep the actor (6.9); grants keep who granted them; every request log line carries
the principal's issuer and subject and the organization, never an email. A full audit log (who
changed which rule, who invited whom) is later work.

## 7. Contracts, layout and configuration

### 7.1 Contracts

- REST: a `bearerAuth` security scheme required on every operation except liveness and readiness;
  the `X-Organization` header documented on every tenant operation; new tags `organizations`
  (staff), `members`, `api-keys`, `machine-clients`. Security requirements are breaking by
  oasdiff's rules, accepted and recorded; `SpecVersion` becomes 0.2.0. `packages/api-client` gains
  `getAccessToken` and `organization` options in its hand-written fetch layer.
- GraphQL: `viewer` (the person, staff or not, their organizations and roles), then members, API
  keys and machine clients. Ledger fields are unchanged; the organization comes from the header.
- gRPC: no change to existing messages; `authorization` and `x-organization` metadata documented
  in `packages/proto`; new services for members and credentials only if integrators need them.
- Broker RPC: `authorization` and `organization` headers documented with the envelope.

### 7.2 Layout

```
apps/api/internal/
  kernel/ledger/
    principal.go          Principal, Tenant, System, context accessors
    permission.go         Permission, Role, the role table
    errors.go             + ErrUnauthenticated, ErrForbidden
  modules/access/
    domain/               Organization, Grant, APIKey, MachineClient, Session; the ports
    application/          Authorizer, organization, member, credential and session use cases
    infrastructure/
      postgres/           organizations, grants, pending_grants, api_keys, machine_clients, sessions
      keycloak/           admin API client: organizations, invitations, members, clients
    interfaces/
      rest/v1/  graphql/  staff, members, credentials
      web/                /auth/login, /callback, /logout, /session (the BFF)
  modules/ledgers/application/authorize.go
  modules/bookkeeping/application/authorize.go
  platform/
    oidc/  oidc/oidctest/ token verification, claim mapping, test issuer
    restapi/              + authenticate and tenant middleware, - jsonBodies
    grpcserver/           + authenticate interceptors
    rpc/                  + token and organization check in the dispatcher
    problem/              + UNAUTHENTICATED, FORBIDDEN
apps/api/migrations/      + organizations, ledger IDs, access tables, write_requests actor
apps/console/src/
  routes/_console/$org/   every page moves under the organization
  lib/auth/               session query, route guard, useCan, organization header
  features/organization/  switcher, members, API keys, machine clients
  features/staff/         organizations (staff only)
deploy/compose/           + keycloak, its realm file, mailpit
```

The depguard rules gain the access module like any other, with a fixture each (AGENTS.md).

### 7.3 Configuration

| Variable                                           | Meaning                                                                                        |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `AUTH_ISSUER_URL`                                  | The Keycloak realm issuer; discovery is read from it. Required.                                |
| `AUTH_AUDIENCE`                                    | The audience access tokens must carry, `tbp-api`. Required.                                    |
| `AUTH_STAFF_CLAIM`, `AUTH_STAFF_VALUE`             | Where the staff role is read from, such as `roles` and `tbp-staff`.                            |
| `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`             | The console's confidential client, for the BFF.                                                |
| `AUTH_REDIRECT_URL`, `AUTH_CONSOLE_ORIGIN`         | The BFF callback, and the origin the CSRF check accepts.                                       |
| `AUTH_SESSION_KEY`                                 | 32 random bytes, base64, encrypting stored tokens; rotation keeps the previous key.            |
| `AUTH_API_KEY_PEPPER`                              | 32 random bytes, base64, for API key hashes.                                                   |
| `AUTH_ADMIN_CLIENT_ID`, `AUTH_ADMIN_CLIENT_SECRET` | The service account the API uses for Keycloak's admin API, with only the permissions it needs. |
| `AUTH_COOKIE_SECURE`                               | True by default; false only for plain-HTTP development.                                        |

There is no switch that turns authentication off (owner's answer). Every one of the first three
rows is required at startup.

## 8. Local stack and tests

- Compose gains Keycloak, on the existing Postgres server with its own database, and Mailpit. The
  realm file in `deploy/compose` seeds: organizations enabled; the console client, the API
  audience, the admin service account; a `roles` mapper; the organizations `default`, `acme` and
  `globex`; a staff user; an owner in each organization; one person in both `acme` and `globex`;
  and an operator and a viewer in `acme`. A `just` recipe grants the platform roles, another
  prints an API key and a machine token for the curl, grpcurl and broker examples.
- Unit tests: the role table; the authorization wrappers of every use case against every role
  (a table built from the role table, so it grows with it); claim mapping; tenant resolution for
  each credential kind; the session cookie and CSRF rules; API key parsing and hashing.
- Isolation tests, the most important ones: `acme` and `globex` each have a ledger named
  `payments`; every query and command of every transport, called as `acme`, must never see or
  touch `globex`'s, and the answer for the other tenant's ledger is 404.
- Transport tests: no credential, a bad token, an expired token and a revoked API key give the
  unauthenticated answer on REST, GraphQL, gRPC, AMQP RPC and NATS RPC; a missing permission the
  forbidden one.
- Integration tests: real Postgres with `oidctest`; the migration of pre-alpha data into
  `default`; the idempotency actor rule end to end. The Keycloak adapter is tested against a
  Keycloak container in the `integration` tag (there is no testcontainers-go module; a generic
  container with the realm file).
- One browser check against the compose stack: sign in, switch organization, accept an
  invitation from Mailpit, a 403, sign out.

## 9. ADRs

| ADR  | Title                                                                               | Relation                                                                             |
| ---- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| 0026 | Identity from Keycloak through standard OpenID Connect                              | New. Sections 3 and 4; the vendor confined to one adapter.                           |
| 0027 | Organizations as tenants, owned by Keycloak Organizations; ledgers per organization | New. Section 5. Amends 0002 (the platform's tables) and 0004 (ledger names).         |
| 0028 | Authorization: platform grants enforced in the application layer                    | New. Section 6.4 to 6.6. Amends 0017 (the access module and its rules).              |
| 0029 | Machine access: API keys and OAuth machine clients                                  | New. Section 6.8.                                                                    |
| 0030 | Console sessions through a backend-for-frontend                                     | Amends 0014 and 0023 (sign-in, organization in the URL, permission-aware pages).     |
| 0031 | Idempotency keys belong to the principal that used them                             | Amends 0007 and 0024.                                                                |
| 0032 | Authenticated transports and stricter request bodies                                | Amends 0018 (security scheme, header, 415), 0021 (exposure) and 0022 (event fields). |

## 10. Phases

Each phase is a series of small Conventional Commits that keep `just lint`, `just test` and
`just build` green and `just generate` diff-free, and ends with a handoff note. The branch is not
deployable between phases 1 and 3, which is acceptable pre-alpha.

0. Decisions: the ADRs of section 9 and this plan, committed.
1. Tenancy in the data model: `organizations`, ledger IDs, names unique per organization, the
   eight tables on `ledger_id`, pre-alpha data in `default`. Until phase 2 a fixed `default`
   tenant is put in every context, so behaviour is unchanged; the isolation tests start here.
2. Authentication: the principal and tenant in the kernel, `platform/oidc` and `oidctest`, the
   authenticators on HTTP, gRPC and the dispatcher, tenant resolution for people (header and
   organization claim), Keycloak and Mailpit in compose with the realm file. From here every call
   needs a credential.
3. Authorization: the access module's grants, the role table, the wrappers and their coverage
   tests, 404 or 403, list filtering, the idempotency actor, 415, the security scheme and header
   in the spec, `SpecVersion` 0.2.0, the client options.
4. Console sign-in: the BFF routes, sessions and CSRF checks, every route under `$org`, the
   organization switcher, `viewer`, `useCan`, the 403 page, sign-out.
5. Organizations and members: first a spike on inviting a person with no account (O6), then the
   Keycloak adapter, staff creating and suspending organizations, invitations and pending grants,
   the members page, organization roles.
6. Machine access: API keys and machine clients, their console pages, the integrator guide for
   every transport.
7. Exposure: TLS options for gRPC and the broker connections, broker credentials in compose, the
   end of "development-only" for gRPC; AGENTS.md loses "authentication" from its out-of-scope
   line.

## 11. Risks

- Cross-tenant leaks are the worst failure. The design concentrates the organization check in
  ledger resolution and the tenant middleware; the isolation tests of section 8 run every
  operation of every transport across two tenants with the same ledger name.
- A use case left unwrapped would be reachable without a check. The reflection test of 6.5 is the
  guard, and it runs in `just test`.
- Membership comes from the token, so a removed member keeps access until the token expires (at
  most five minutes). Grants are checked in Postgres on every call, so revoking a role is
  immediate.
- The platform depends on Keycloak's admin API for organizations, invitations and machine
  clients. Those calls are rare and retryable; a Keycloak outage blocks sign-in and those actions,
  not calls with valid tokens or API keys. `/readyz` reports Keycloak as optional.
- The API's Keycloak service account needs `manage-users` to add members, which also lets it edit
  any user of the realm, staff included. A compromised API process could therefore take over
  Keycloak accounts. Mitigations: fine-grained admin permissions v2 narrowed to what the adapter
  does, the secret only in the API's environment, and every admin call logged with the principal
  that caused it. Staff accounts can additionally live in a separate realm later if this is not
  enough.
- Once any organization exists, Keycloak's login page asks for the username or email first, then
  the password. It is a visible change for everyone, documented in the console guide.
- Cookie sessions bring CSRF; section 6.7 layers SameSite, a fetch-metadata or origin check and
  strict JSON.
- The ledger migration rewrites keys on eight tables. It runs on pre-alpha data only and has its
  own integration test.

## 12. Answers and open questions

Answered by the owner on 2026-10-26:

- Multi-tenant from day 1 (section 5).
- Keycloak as the IdP.
- Machine access through both API keys and OAuth client credentials.
- Authentication always on, no switch.
- The tenant comes from the credential, or the `X-Organization` header for people.
- Only staff create organizations for now.
- Keycloak Organizations own organizations and membership.
- Staff manage organizations but cannot read tenants' ledgers without a grant.

Assumed from the first proposal, accepted with the approval:

- O1 The console signs in through the BFF in the API (6.7).
- O2 Staff come from a Keycloak realm role (5.4).
- O3 An idempotency key replays only for the principal that used it (6.9).
- O4 Broker RPC and the Kafka topic are for the platform's own services, not tenants (5.6).
- O5 The role set of 6.4.

Open, raised by section 4.3:

- O6 Inviting a person who has no account yet. Keycloak's invitation then sends a registration
  link, which may need self-registration switched on for the whole realm (unverified). Either
  allow that (simplest; an account with no organization sees nothing), or have the API create the
  account itself and send the existing-user invitation, so there is no public registration page
  (recommended, to keep "no public sign-up" literal). Phase 5 starts with a spike that settles
  whether the first option even needs self-registration.
