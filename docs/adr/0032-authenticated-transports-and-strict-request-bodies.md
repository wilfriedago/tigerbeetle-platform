# 32. Authenticated transports and strict request bodies

Status: Accepted
Date: 2026-10-26

## Context and Problem Statement

ADR-0021 kept gRPC, AMQP RPC, NATS RPC and the Kafka events off by default and development-only
until authentication was decided. ADR-0018 kept today's lenient request bodies: the `jsonBodies`
middleware in `apps/api/internal/platform/restapi/middleware.go` reads any body as JSON whatever
its `Content-Type`, and a stricter answer waited for a new API minor version. ADR-0022 published
ledger events for every ledger on one topic.

With cookie sessions (ADR-0030) the lenient bodies become a CSRF risk: a form on another site can
post `text/plain`, which the middleware rewrites to JSON in front of `/graphql` too. Tenants
(ADR-0027) make one event topic for every ledger a cross-tenant stream.

This record replaces only the lenient bodies and the absence of a security scheme in ADR-0018,
the development-only exposure in ADR-0021, and the event payload of ADR-0022 (which gains
fields); everything else in the three stands.

## Decision Drivers

- Every transport authenticates the same way and resolves the same tenant (ADR-0027).
- No body read as JSON unless it says so.
- The REST contract states the security it requires.
- Events never reach a tenant they do not belong to.

## Considered Options

1. Credentials on every transport where the request ID already travels; strict bodies; events
   internal to the platform.
2. Keep the brokers development-only, authenticate HTTP and gRPC only.

Option 2 leaves two transports that can never be turned on outside development.

## Decision Outcome

Chosen option: 1.

- Credentials: `Authorization: Bearer` on HTTP; `authorization` and `x-organization` gRPC
  metadata, checked by interceptors in `internal/platform/grpcserver`; `authorization` and
  `organization` message headers on AMQP and NATS, checked by `rpc.Dispatcher` before routing.
  A message that waits in a queue past its token's expiry is unauthenticated.
- The spec gains a `bearerAuth` scheme required on every operation except liveness and
  readiness, and the `X-Organization` header on every tenant operation. Security requirements
  are breaking by oasdiff's rules; the change is accepted and `SpecVersion` becomes 0.2.0.
- `jsonBodies` goes. A body with a non-JSON `Content-Type` gets 415, Huma's default.
- gRPC stays off by default but stops being development-only once phase 7 of the plan adds TLS
  options. AMQP RPC and NATS RPC are for the platform's own services, authenticated like any
  caller; tenants use REST, GraphQL and gRPC.
- Ledger events gain the organization's ID and alias (the `v1` format only gains fields). The
  topic stays internal; delivering events to tenants is later work.
- `/readyz` reports Keycloak as an optional dependency, like a broker.

### Consequences

- Good: one authentication model on six entry points.
- Good: the CSRF hole that cookies would open through lenient bodies is closed.
- Good: generated clients see the security scheme and the header.
- Bad: `curl -d` without `-H 'Content-Type: application/json'` now answers 415; "Copy as curl"
  already sends the header.
- Bad: a breaking spec version for every integrator, acceptable pre-alpha.

## More Information

- `docs/plans/auth/plan.md`, sections 5.6, 6.2, 6.9 and 7.1.
- [ADR-0018](0018-rest-spec-generated-from-go-with-huma.md),
  [ADR-0021](0021-transports-over-one-application-layer.md),
  [ADR-0022](0022-ledger-events-to-kafka-through-tigerbeetle-cdc.md): the records this one amends.
- RFC 6750: https://www.rfc-editor.org/rfc/rfc6750
