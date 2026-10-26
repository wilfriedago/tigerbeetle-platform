# Calling the API from your own systems

How a service, a script or a batch job authenticates to the platform and calls it, on every
transport. The decisions behind it are ADR-0026 (identity), ADR-0027 (organizations), ADR-0028
(roles) and ADR-0029 (keys and machine clients).

## Pick a credential

An organization's owners and admins create both kinds in the console, under **API keys** and
**Machine clients**. Each belongs to one organization and acts only with the ledger roles given
to it: `viewer` reads, `operator` also moves money and edits metadata, `admin` also changes the
ledger's rules, event types and access.

|                 | API key                                 | Machine client                                         |
| --------------- | --------------------------------------- | ------------------------------------------------------ |
| What you hold   | `tbp_<id>_<secret>`, sent on every call | A client ID and secret, traded at Keycloak for a token |
| Lifetime        | Until it expires or is revoked          | The secret until regenerated; each token 5 minutes     |
| Sent to the API | The key itself                          | Only short-lived tokens                                |
| Choose it when  | The caller cannot do OAuth 2.0          | The caller supports OAuth 2.0 client credentials       |

Both secrets are shown once, at creation. The platform keeps only a keyed hash of an API key;
Keycloak keeps the client secret. Store them in your secret manager.

A credential never needs `X-Organization`: it acts for its own organization. Naming another one
answers 403 `forbidden`.

## Get a token (machine clients)

```sh
curl "$TOKEN_URL" \
  -d grant_type=client_credentials \
  -u "$CLIENT_ID:$CLIENT_SECRET"
```

`TOKEN_URL` is shown when the client is created, `https://<keycloak>/realms/tbp/protocol/openid-connect/token`.
The answer's `access_token` lasts `expires_in` seconds (300); fetch a new one shortly before it
expires rather than on every call. A deleted client gets no new token, and the API refuses its
tokens from the next call.

## Call the API

The bearer value below is either an API key or an access token; the API tells them apart by the
`tbp_` prefix.

### REST

```sh
curl https://api.example/v1/ledgers/payments/accounts \
  -H "Authorization: Bearer $TBP_API_KEY"
```

The contract is `packages/openapi/openapi.yaml`. Every write that moves money also sends
`Idempotency-Key`; retry a write with the same key, and only you can replay your own key
(ADR-0031).

### GraphQL

```sh
curl https://api.example/graphql \
  -H "Authorization: Bearer $TBP_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query":"{ ledger(name: \"payments\") { name accountCount } }"}'
```

The schema is `packages/graphql/schema.graphql`. A refused call answers HTTP 200 with an error
whose `extensions.code` is `UNAUTHENTICATED`, `FORBIDDEN` or `NOT_FOUND`.

### gRPC

```sh
grpcurl -H "authorization: Bearer $TBP_API_KEY" \
  -d '{"ledger":"payments"}' api.example:9090 tigerbeetle.platform.v1.LedgerService/GetLedger
```

The contract is `packages/proto`. Credentials travel in the `authorization` metadata; a refused
call fails with `UNAUTHENTICATED`, `PERMISSION_DENIED` or `NOT_FOUND`.

### AMQP and NATS RPC

These serve the platform's own services, not tenants (plan section 5.6), and are off unless
enabled. They take the same credential in the `authorization` message header, the idempotency key
in `idempotency-key`, and answer a refused call with `{"ok": false, "error": {...}}`.

## When a call is refused

| Answer                       | Meaning                                                                                                      | What to do                               |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------- |
| 401 `unauthenticated`        | No credential, or one that is not valid: a wrong, expired or revoked key, an expired token, a deleted client | Fetch a new token, or check the key      |
| 403 `forbidden`              | The credential lacks the role, or names another organization                                                 | Ask an admin for the role on that ledger |
| 403 `organization-suspended` | The organization is suspended; `detail` says why                                                             | Contact the platform's staff             |
| 404 `not-found`              | The ledger does not exist, or the credential holds no role on it                                             | Check the ledger name and the roles      |

Problem types are documented in `docs/problems.md`.

## Rotating

- API key: create a second key with the same roles, deploy it, then revoke the first. Both work
  in between.
- Machine client: regenerate the secret from the client's page and deploy it; the old secret
  stops at once, tokens already issued last until they expire. For no gap, create a second
  client with the same roles, switch, then delete the first.

## What the platform records

A key is the principal `tbp:api-key/<id>`; a machine client is its service account in Keycloak.
Either one is the actor of every write it makes (the write requests in the console), and keeps
its grants and writes after it is revoked or deleted. A key's last use and a client's last token
are recorded at most once a minute.
