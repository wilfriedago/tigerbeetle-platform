# 21. Transports over one application layer: gRPC, AMQP RPC, NATS RPC

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

The API serves REST (ADR-0018) and GraphQL (ADR-0020), and gains gRPC and request/reply over
RabbitMQ (AMQP RPC) and NATS (NATS RPC); Kafka carries ledger events out (ADR-0022). Every
transport must call the same use cases (ADR-0017), map the same typed errors without leaking
driver text, carry the Idempotency-Key so that ADR-0007 holds everywhere, and start and stop from
one place. evrone/go-clean-template is the model for the broker RPC, but its code handles one
request at a time, delivers every request to every replica, collapses errors into three strings
and does not recognise a NATS timeout.

## Decision Drivers

- No business rule in a transport; controllers translate and call application interfaces.
- One error mapping, rendered per transport; no TigerBeetle or Postgres text on any wire.
- Amounts and 128-bit IDs are strings on every wire.
- Each broker request runs exactly once across replicas, with bounded concurrency and deadlines.
- Transports are switched by config; brokers stay off unless configured.
- Authentication is out of scope (`AGENTS.md`), so nothing new may be exposed by default.

## Considered Options

1. Per-transport controllers in each module, one shared RPC envelope and router for both
   brokers, standard gRPC error details (chosen).
2. One controller folder per broker, as evrone does.
3. A single generic gateway that maps any transport to REST internally.

Option 2 duplicates identical controllers: the envelope and semantics are the same, only
delivery differs. Option 3 sends every call through HTTP semantics and loses typed gRPC messages.

## Decision Outcome

Chosen option: 1.

Layout. Each module has `interfaces/grpc/v1` and `interfaces/rpc/v1`; the RPC folder serves both
brokers, and broker specifics live in `internal/platform/amqprpc` and `internal/platform/natsrpc`
over a shared `internal/platform/rpc` (envelope, router, worker pool). This deviates on purpose
from "one controller folder per transport".

Errors. `internal/platform/problem` turns any error into a neutral value (kind, reason, type,
title, REST status, detail, posting, results, retry delay); anything that is not a kernel error
is `INTERNAL` with no detail. Each transport renders it:

| Kernel error                        | REST | GraphQL `extensions.code` | gRPC                 | RPC `error.code` |
| ----------------------------------- | ---- | ------------------------- | -------------------- | ---------------- |
| `ErrInvalid`                        | 400  | `INVALID`                 | `InvalidArgument`    | `INVALID`        |
| `ErrNotFound`                       | 404  | `NOT_FOUND`               | `NotFound`           | `NOT_FOUND`      |
| `ErrAlreadyExists`                  | 409  | `ALREADY_EXISTS`          | `AlreadyExists`      | `ALREADY_EXISTS` |
| `ErrConflict`                       | 409  | `CONFLICT`                | `FailedPrecondition` | `CONFLICT`       |
| `ErrConflict`, `request-in-flight`  | 409  | `CONFLICT`                | `Aborted`            | `CONFLICT`       |
| `ErrRejected` (declines, key reuse) | 422  | `REJECTED`                | `FailedPrecondition` | `REJECTED`       |
| `ErrUnavailable`                    | 503  | `UNAVAILABLE`             | `Unavailable`        | `UNAVAILABLE`    |
| anything else                       | 500  | `INTERNAL`                | `Internal`           | `INTERNAL`       |

GraphQL extensions and the RPC `error` object carry the REST member names, so clients share one
parser: `code`, `reason`, `type`, `title`, `status`, `detail`, `requestId`, `retryAfter` (whole
seconds), `posting`, `results`. gRPC uses the standard `google.rpc` details, from
`google.golang.org/genproto/googleapis/rpc`, a module grpc-go already requires:

- `ErrorInfo`, following AIP-193. `reason` is the problem type's slug in UPPER_SNAKE_CASE
  (`/problems/exceeds-credits` gives `EXCEEDS_CREDITS`; `about:blank` gives the kind, such as
  `INTERNAL`), derived by one function, never a table. A unit test walks every problem type and
  checks the result against `[A-Z][A-Z0-9_]+[A-Z0-9]` and the 63-character limit. `domain` is the
  fixed service identifier `tigerbeetle-platform-api`, the `service` value of `/v1/_info`.
  `metadata["type"]` is the problem type URI unchanged, next to `title`, `status`, `detail`,
  `requestId`, `posting` and `results` (comma-separated).
- `RetryInfo` with the delay REST sends as `Retry-After`; `BadRequest` when an input error names
  its field.

Each transport's adapter tests return an error wrapping a fake driver message and assert that
the text appears in no byte of the response.

Idempotency. The protocol stays in bookkeeping's application layer (ADR-0024). REST and both
brokers carry the key as a header (`Idempotency-Key`, `idempotency-key` for AMQP); GraphQL and
gRPC carry it as an `idempotencyKey` / `idempotency_key` field of the input, because one GraphQL
request can hold several mutations and a header cannot say which one it belongs to. The body hash
covers the normalised command, so a write sent over REST and retried over GraphQL with the same
key is a replay. Every transport echoes the request ID.

gRPC. Protos in `packages/proto/tigerbeetle/platform/v1`, one package, services mirroring the
REST tags; money and IDs are `string` fields; paging uses `page_size`, `page_token` and
`next_page_token` (AIP-158) with the REST cursor. buf v1.75.0 with protoc-gen-go v1.36.12 and
protoc-gen-go-grpc v1.6.2 as local plugins, pinned with checksums in `scripts/install-tools.sh`,
writes `internal/platform/grpcapi/platformv1`; CI runs `buf lint` (`STANDARD`) and
`buf breaking` against the base branch. `internal/platform/grpcserver` adds request ID, slog
access log, panic recovery and error-mapping interceptors, `grpc.health.v1` and server
reflection, and on shutdown calls `GracefulStop`, then `Stop` after `SHUTDOWN_TIMEOUT`.
Generated protobuf code registers itself in `init()`; the "no `init()`" rule covers hand-written
code. grpc-go's logger is a package-level setting, routed to slog once behind a `sync.Once`, the
same exception ADR-0018 makes for Huma.

Broker RPC. Methods are named `v1.<operationId>`. A request is
`{"params": {...}, "body": {...}}` with REST's parameter and body names; a reply is
`{"ok": true, "data": ...}` or `{"ok": false, "error": {...}}`. Each server runs a worker pool
(`RPC_CONCURRENCY`, default 16); a request's deadline is the earlier of the client's `x-deadline`
header and `RPC_TIMEOUT_MAX` (30 s), and a request already past it is answered
`UNAVAILABLE deadline-exceeded` without running.

- AMQP (amqp091-go v1.16.0): one durable queue `tbp.rpc.v1` bound to the direct exchange
  `tbp.rpc`, consumed by every replica (competing consumers), so each request runs once.
  Prefetch equals the pool size. Replies go to `reply_to` (normally RabbitMQ's direct reply-to)
  with the request's `correlation_id`; the ack follows the reply; a malformed request is answered
  and acked, never requeued forever. amqp091-go does not reconnect, so `internal/platform/amqp`
  redials with capped backoff and redeclares the topology.
- NATS (nats.go v1.55.0): a `micro` service in queue group `tbp-api`, endpoints
  `tbp.rpc.v1.<operationId>`, answered from the worker pool; errors also set micro's
  `Nats-Service-Error` headers. Clients map `nats.ErrTimeout` and `nats.ErrNoResponders`
  separately. Core NATS does not redeliver; a client retries a write with the same key.

Lifecycle. Every server and background worker implements
`Runner { Name() string; Run(ctx) error }`. `internal/bootstrap` starts the enabled ones in an
`errgroup`; when one fails, the others are cancelled and drain, and resources close in reverse
order. Constructors return errors instead of exiting.

Switches and exposure. `REST_ENABLED` and `GRAPHQL_ENABLED` default to true; `GRPC_ENABLED`,
`AMQP_RPC_ENABLED` and `NATS_RPC_ENABLED` default to false, and enabling a broker without its URL
is a startup error. gRPC, AMQP RPC, NATS RPC and Kafka are development-only until authentication
is decided: they stay off by default, and only the compose `brokers` profile turns them on.
GraphQL has the status REST has today.

Readiness. `/readyz` probes Postgres, TigerBeetle and every enabled broker, and reports each. It
answers 503 only when Postgres or TigerBeetle is down: readiness gates HTTP traffic from the load
balancer, broker consumers pull their work, and all replicas share one broker, so failing
readiness on a broker outage would stop REST everywhere without helping. A broker that is down
makes the status `degraded`, with a 200; `grpc.health.v1` stays SERVING. The `Dependencies`
object gains optional broker members, absent when a broker is off, and `Readiness.status` gains
`degraded`, a response enum addition that oasdiff reports as a warning, accepted on purpose.

### Consequences

- Good: a new operation is one use case and a thin controller per transport, with the same
  errors and idempotency everywhere.
- Good: generic gRPC tooling reads the error details; broker requests run once, bounded.
- Bad: five transports to test and keep in step; adapter tests and an end-to-end suite against
  the compose stack (`just test-e2e`) carry the load.
- Bad: two package-level settings exceptions (Huma, grpc-go's logger), each behind one
  `sync.Once`.
- Bad: brokers mean more moving parts locally, mitigated by the compose profile and switches.

## More Information

- [ADR-0007](0007-linked-transfers-ids-idempotency.md), [ADR-0017](0017-bounded-context-modules-and-dependency-rules.md),
  [ADR-0018](0018-rest-spec-generated-from-go-with-huma.md), [ADR-0020](0020-graphql-schema-generated-from-go-registrations.md).
- `docs/plans/ddd-transports/plan.md`, sections 9 to 11 and 13.
- AIP-193, errors: https://google.aip.dev/193; AIP-158, pagination: https://google.aip.dev/158
- RabbitMQ direct reply-to: https://www.rabbitmq.com/docs/direct-reply-to
- NATS `micro` package at the pinned tag: https://github.com/nats-io/nats.go/tree/v1.55.0/micro
