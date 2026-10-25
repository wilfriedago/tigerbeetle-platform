# Ledger events

The platform publishes every transfer TigerBeetle commits to the Kafka topic
`tbp.ledger-events.v1`, one record per transfer (ADR-0022). This page is the format a consumer
reads. It is development-only until authentication is decided, like the other broker transports
(ADR-0021): `just compose-up-brokers` turns it on.

## Where the events come from

```
TigerBeetle --(CDC job)--> RabbitMQ: exchange tbp.cdc --> queue tbp.ledger-events.relay
            --(relay in the API)--> Kafka: topic tbp.ledger-events.v1
```

- The CDC job is the `amqp` subcommand of the TigerBeetle server image, never newer than the
  cluster. It reads committed transfers in ascending TigerBeetle timestamp order, with both
  accounts and their balances as of the transfer, and publishes them to RabbitMQ. Its progress
  lives in RabbitMQ, so a restart resumes where it stopped.
- The relay runs in the API process (`KAFKA_EVENTS_ENABLED`). The relay queue has a single active
  consumer, so any number of replicas may run it while one at a time reads, in order. It adds what
  Postgres recorded before the transfer existed, then writes the record and acknowledges the
  RabbitMQ message only after Kafka acknowledged the record.
- Every committed transfer is published, including expirations, which happen inside TigerBeetle
  with no request behind them, and transfers written by other TigerBeetle clients.

## Delivery

- At least once. The CDC job and the relay may both send an event again after a crash. Nothing
  is lost while RabbitMQ keeps what it accepted: a crash delays events, it does not drop them.
- Deduplicate on the `tb-timestamp` header (or the `timestamp` member): TigerBeetle timestamps
  are unique per event.
- Order: events are relayed in TigerBeetle timestamp order. The record key keeps the legs of one
  transaction in one partition, in order; across keys, order follows partitions.

## Record

| Part    | Content                                                                                                                                                                                             |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Key     | The API transaction or hold ID (`user_data_128`, 32 hexadecimal characters); the transfer ID when `user_data_128` is zero.                                                                          |
| Headers | `event-type`: one of the types below. `tb-timestamp`: the TigerBeetle timestamp of the event, nanoseconds, decimal.                                                                                 |
| Value   | JSON, below. Every u128 and u64 is a string: IDs as 32 hexadecimal characters, as everywhere in the API; amounts, balances, `userData64` and timestamps in decimal. u32 and u16 values are numbers. |

Event types:

| `type`              | The transfer                                                    |
| ------------------- | --------------------------------------------------------------- |
| `single_phase`      | A posted transfer: one leg of a transaction or a revert.        |
| `two_phase_pending` | A pending transfer: a hold reserves its amount.                 |
| `two_phase_posted`  | The transfer that captures a hold, in full or in part.          |
| `two_phase_voided`  | The transfer that voids a hold.                                 |
| `two_phase_expired` | A hold whose timeout passed; TigerBeetle released it by itself. |

Value:

```json
{
  "type": "single_phase",
  "timestamp": "1792967077041736876",
  "ledger": "eu-payments",
  "asset": "EUR/2",
  "tbLedger": 1,
  "transactionId": "01a1752a6ca3d022fa801c5bde760c67",
  "transfer": {
    "id": "01a1752a6ca3d022fa801c5bde760c68",
    "amount": "4000",
    "pendingId": null,
    "userData128": "01a1752a6ca3d022fa801c5bde760c67",
    "userData64": "0",
    "userData32": 0,
    "timeout": 0,
    "code": 3,
    "flags": 0,
    "timestamp": "1792967077041736876"
  },
  "debitAccount": {
    "id": "01a1752a6b7d9909be66e7bc23fd4e98",
    "address": "users:1618:wallet",
    "debitsPending": "0",
    "debitsPosted": "4000",
    "creditsPending": "0",
    "creditsPosted": "90000",
    "userData128": "00000000000000000000000000000000",
    "userData64": "0",
    "userData32": 0,
    "code": 10,
    "flags": 10,
    "timestamp": "1792967076737848817"
  },
  "creditAccount": { "...": "the same members as debitAccount" }
}
```

- `ledger`, `asset` and the account `address` members come from Postgres, which records them
  before the transfer exists. They are `null` for a transfer the platform does not know, such as
  one another TigerBeetle client wrote.
- `transactionId` is `null` when `user_data_128` is zero, which the platform never writes.
- `tbLedger` and `code` are TigerBeetle's numbers: the ledger number of the (ledger, asset) pair
  (ADR-0005) and the event type code (ADR-0013).
- Balances are as of this event: a user balance is `creditsPosted` minus `debitsPosted`, and the
  available balance also subtracts `debitsPending` (`docs/architecture.md`, the read path).
- Transaction metadata and references are not in the event: Postgres writes them after the
  commit, so reading them here could race. A consumer that needs them calls
  `GET /v1/ledgers/{ledger}/transactions/{transactionId}`.

A field is only ever added to the value under `v1`; a change that breaks it would be a new topic,
`tbp.ledger-events.v2`.
