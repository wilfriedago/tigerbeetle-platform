# 22. Ledger events to Kafka through TigerBeetle CDC

Status: Accepted, amended by [ADR-0031](0031-authenticated-transports-and-strict-request-bodies.md)
Date: 2026-10-25

## Context and Problem Statement

The API gains a Kafka interface. Kafka could carry commands in (requests on a topic, replies on a
reply topic) or ledger events out, or both. Events must leave only after TigerBeetle has committed
them, and must not be lost if the process dies between that commit and their publication: the
dual-write gap.

ADR-0002 planned a Postgres outbox for publication, "written in the same Postgres transaction as
the commit mark so publication cannot be lost". It was never built. The commit mark is written
after TigerBeetle commits, so a crash in between leaves a committed transfer without an outbox
row until the reconciler settles it. And some ledger events never pass through the API at all: a
pending transfer that times out expires inside TigerBeetle, with no request to write an outbox
row for.

TigerBeetle has its own change data capture since 0.16.43: the `tigerbeetle amqp` subcommand
streams every committed transfer, with both account balances as of that event, to an AMQP 0.9.1
broker. Spike S3 (2026-10-25) ran it from the pinned `ghcr.io/tigerbeetle/tigerbeetle:0.17.9`
image against RabbitMQ 4.4.0:

- All five event types arrived, in ascending TigerBeetle timestamp order: `single_phase`,
  `two_phase_pending`, `two_phase_posted`, `two_phase_voided` and `two_phase_expired` (one second
  after a pending transfer with `timeout=1`, with no client involved). Each leg of a linked chain
  is its own event.
- Messages are persistent, `content_type: application/json`, with headers `event_type`,
  `ledger`, `transfer_code`, `debit_account_code` and `credit_account_code`.
- u128 and u64 values are JSON numbers up to 2^53 - 1 and JSON strings from 2^53 on
  (`9007199254740991` against `"9007199254740992"`); nanosecond timestamps are always strings.
  The documentation's example, which shows numbers, is the small case.
- `--host` accepts an IP address only: a compose service name fails with "invalid IPv4 or IPv6
  address".
- The job keeps its progress as headers (`release`, `timestamp`) on the durable queue
  `tigerbeetle.internal.progress.<cluster>`. Killed and restarted, it published the 20 transfers
  written while it was down within 0.7 s, without gaps. A second job for the same cluster is
  refused with `RESOURCE_LOCKED` on the exclusive queue `tigerbeetle.internal.locker.<cluster>`.
  A kill during a 2,000-event backlog produced no duplicate in one run, which proves nothing;
  TigerBeetle documents at-least-once delivery.

This record amends ADR-0002: the outbox is no longer the publication path for ledger events.

## Decision Drivers

- No dual-write gap: an event can be delayed, never lost.
- Every committed transfer, including expirations and writes from other TigerBeetle clients.
- No extra work on the request path.
- Amounts and IDs as strings on the wire (ADR-0004), decoded without a float.
- Pure Go clients, pinned; the default dev stack runs without brokers.

## Considered Options

1. Outbound events only: TigerBeetle CDC to RabbitMQ, a relay in the API, then Kafka (chosen).
2. A Postgres outbox written with the commit mark, relayed to Kafka.
3. A relay that polls TigerBeetle's `get_change_events` itself (the Go client's experimental
   `GetChangeEvents`) and keeps its checkpoint in Kafka.
4. Inbound commands over Kafka as well.

Option 2 has the gap described above and never sees expirations. Option 3 skips RabbitMQ but
builds on an undocumented API and must reimplement single-writer locking and progress. Option 4
rebuilds request/reply by hand (a reply topic plus correlation) while AMQP RPC and NATS RPC already
carry commands with replies (ADR-0021), and the Idempotency-Key already makes redelivery safe.

## Decision Outcome

Chosen option: 1. No inbound Kafka command transport for now.

- The CDC job runs from the server's own image, so it is never newer than the cluster, as
  TigerBeetle requires, and `scripts/check-versions.sh` still sees one TigerBeetle tag. It
  publishes to the durable fanout exchange `tbp.cdc`, bound to the durable queue
  `tbp.ledger-events.relay` with `x-single-active-consumer`. A RabbitMQ definitions file declares
  both, so the job never publishes to a missing exchange.
- The relay is a bootstrap runner in the API process (ADR-0021), behind `KAFKA_EVENTS_ENABLED`
  (default false) with `KAFKA_BROKERS`, `KAFKA_EVENTS_TOPIC`, `CDC_AMQP_URL` and `CDC_QUEUE`. Every
  replica may run it; the queue lets one consumer at a time read, which keeps the order. It lives
  in bookkeeping: `interfaces/cdc` consumes, `application/relay_ledger_event.go` enriches and
  publishes through a `LedgerEventPublisher` port, `infrastructure/kafka` implements the port.
- Decoding uses `json.Decoder.UseNumber` and accepts a number or a string for every u128 and u64,
  parsed into exact integers.
- Enrichment adds only facts Postgres holds before the transfer exists: the ledger name and asset
  by TigerBeetle ledger number (ADR-0005), and the addresses by account ID (ADR-0006). Transaction
  metadata and references are written after the commit and could race, so consumers that need
  them call the API with the transaction ID.
- Kafka (franz-go v1.22.2, idempotent producer, `acks=all`), topic `tbp.ledger-events.v1`. The
  record key is the API transaction ID (`user_data_128` as 32 hexadecimal characters), or the
  transfer ID when `user_data_128` is zero, so the legs of one transaction share a partition and
  keep their order. The value is JSON with every u128 and u64 as a string; headers carry
  `event-type` and `tb-timestamp`. TigerBeetle timestamps are unique, so consumers deduplicate
  on `tb-timestamp`.
- The RabbitMQ delivery is acknowledged only after Kafka acknowledges the record. With Kafka or
  Postgres unavailable, the delivery is requeued after a capped backoff, and readiness shows the
  dependency down: Kafka as `degraded`, Postgres as 503 (ADR-0021).
- Compose: the `brokers` profile adds RabbitMQ, the CDC job and `apache/kafka:4.3.2`, pinned by
  digest. The network gets a fixed subnet, and `tigerbeetle` and `rabbitmq` fixed addresses,
  because the job accepts IP addresses only. The job needs `seccomp=unconfined` for io_uring, like
  the server. It is development-only until authentication is decided (ADR-0021).
- Events born in Postgres (ledger created, metadata replaced, rules and event types changed) are
  not published: nothing consumes them yet. ADR-0002's outbox remains the planned path for them.

### Consequences

- Good: the source is TigerBeetle's committed state, read by timestamp; a crash delays events,
  it cannot lose them while RabbitMQ keeps what it accepted.
- Good: expirations and writes from any TigerBeetle client are published; the request path does
  no extra work.
- Bad: Kafka events need RabbitMQ too, and the CDC job is one more process; there is no TLS on
  the job's AMQP link and only PLAIN authentication.
- Bad: progress lives in RabbitMQ. Losing its state replays from the beginning unless the job is
  started with `--timestamp-last`; consumers must deduplicate anyway.
- Bad: fixed IP addresses in compose, because the job resolves no names (the API resolves its
  own before calling the Go client, `docs/handoff.md` fact 3).
- Bad: events carry TigerBeetle fields, ledger names and addresses only, not transaction
  metadata.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): the outbox this record
  replaces for ledger events.
- [ADR-0021](0021-transports-over-one-application-layer.md): switches, readiness, exposure.
- `docs/plans/ddd-transports/plan.md`, section 12; `docs/plans/ddd-transports/research.md`,
  section (e).
- TigerBeetle CDC: https://docs.tigerbeetle.com/operating/cdc/
- TigerBeetle 0.17.9: https://github.com/tigerbeetle/tigerbeetle/releases/tag/0.17.9
- franz-go: https://github.com/twmb/franz-go
