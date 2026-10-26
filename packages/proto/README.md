# packages/proto

The gRPC contract of the platform API (ADR-0021): one proto package, `tigerbeetle.platform.v1`,
with one service per REST tag. The services call the same use cases as REST and GraphQL, so a
write sent with an `idempotency_key` replays on any transport.

- `just lint-proto` runs `buf lint` (STANDARD rules).
- `just generate` runs `buf generate`, which writes the Go code into
  `apps/api/internal/platform/grpcapi/platformv1`. Never edit that code.
- CI runs `buf breaking` against the base branch.

Conventions: amounts, IDs and TigerBeetle timestamps are decimal or hex strings, never numbers,
as on every other transport. Lists page with `page_size`, `page_token` and `next_page_token`
(AIP-158), carrying the same opaque cursor as REST. A money write carries its Idempotency-Key in
`idempotency_key` (AIP-155 style). Errors carry `google.rpc.ErrorInfo`, `RetryInfo` and
`BadRequest` details (plan section 9.2). Liveness and readiness are the standard
`grpc.health.v1.Health` service.

Authentication (ADR-0025, ADR-0031): every call except `grpc.health.v1.Health` and server
reflection sends `authorization: Bearer <token>` metadata, and a person names the organization
with `x-organization` metadata (ADR-0026). Without a valid token a call fails with
`UNAUTHENTICATED`; a role that lacks the permission gives `PERMISSION_DENIED` (ADR-0027). For
example: `grpcurl -H "authorization: Bearer $TOKEN" -H "x-organization: acme" ...`.
