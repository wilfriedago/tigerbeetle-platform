# Research: DDD modules and multi-transport API

Date: 2026-10-25. Repo HEAD when written: 2cfa3a3. Research only: nothing was installed, cloned or run
against the project. Another worker writes the plan from this file.

How facts were gathered: GitHub REST API (`gh api`), `go list -m`, `npm view`, Docker Hub API, the
published npm tarball files through jsDelivr, and vendor docs. "Latest stable" means the newest
non-prerelease on the registry on 2026-10-25. Docker sizes are the compressed amd64 size reported by
the Docker Hub API; digests are manifest-list digests. No startup time was measured locally; the
numbers quoted in (d) are from the cited sources and are labelled as such.

Source bases used for file paths (append the quoted path to build the URL):

| Short | Base URL                                                                                    |
| ----- | ------------------------------------------------------------------------------------------- |
| GB    | https://github.com/meghrazchi/go-boilerplate/blob/836d02be2e8bef4cf21a481614cc3c81678ae922/ |
| EV    | https://github.com/evrone/go-clean-template/blob/e72dda1c4aeafe55490c4118427ddca13b70187e/  |
| HU    | https://github.com/danielgtaylor/huma/blob/v2.39.1/                                         |
| GQ    | https://github.com/99designs/gqlgen/blob/v0.17.98/                                          |
| NG    | https://github.com/nats-io/nats.go/blob/v1.55.0/                                            |
| TB    | https://github.com/tigerbeetle/tigerbeetle/blob/0.17.9/                                     |
| TBGO  | https://github.com/tigerbeetle/tigerbeetle-go/blob/v0.17.10/                                |
| HA    | https://github.com/hey-api/hey-api/blob/d6ba11fd9a/                                         |

## 0. Findings the plan must not miss

1. go-boilerplate has one real module (`user`) and one interfaces-only module (`health`). No module
   imports another, so it gives no guidance on cross-module dependencies. It has no licence file,
   so copy ideas only.
2. evrone/go-clean-template is a good map of "one folder per transport, versioned by folder, one
   server package each, one wiring file". Its AMQP and NATS RPC code is not safe to copy: requests
   are handled serially, every replica receives every request (fanout exchange, plain NATS
   subscribe), handler errors collapse into three status strings, and a NATS timeout is never
   mapped. Details in (b).
3. TypeScript compiler API, verified by scanning the published files: `@hey-api/openapi-ts` 0.99.0
   (latest) and `openapi-typescript` 7.13.0 import `typescript` at runtime and cannot run on
   TypeScript 7.0.2; `orval` 8.x core/fetch/query and all `@graphql-codegen/*` packages do not.
4. TigerBeetle CDC exists since 0.16.43 (2025-05-30), the pinned 0.17.9 server supports it, and it
   publishes only to AMQP 0.9.1 (RabbitMQ). There is no direct path to Kafka. Through a bridge,
   ledger events reach Kafka at-least-once with no dual-write gap; the Postgres outbox is still
   needed for events that are born in Postgres. Details in (e).
5. Huma replaces the code-generation choice recorded in ADR-0008. That ADR lists "code-first ...
   a framework such as huma" as a rejected option, so a superseding ADR is needed.
6. Huma sets two package-level globals that the "no globals" rule will notice:
   `huma.NewError` (error model) and `huma.DefaultArrayNullable` (defaults to true, so every slice is
   `type: ["array","null"]` in the spec). Both must be assigned once in `main` wiring.
7. pnpm's documented default `minimumReleaseAge` is 1440 minutes
   (https://github.com/pnpm/pnpm/blob/main/pnpm/docs/settings/dependency-resolution.md; the repo
   already uses `minimumReleaseAgeExclude` in `pnpm-workspace.yaml`). `orval@8.45.0` was published
   2026-10-25 12:33 UTC (`npm view orval time`), so it is blocked for 24 hours; pin `8.44.0` or exclude it.
8. TigerBeetle's CDC job takes `--host` as `ip:port` only ("Multiple addresses ... and DNS names are not
   supported", https://docs.tigerbeetle.com/operating/cdc/), so a compose service name such as
   `rabbitmq` cannot be used as is.

## (a) meghrazchi/go-boilerplate

Repo: https://github.com/meghrazchi/go-boilerplate. Default branch `main`, read at 836d02be (pushed
2026-06-09). GitHub reports no licence (`license: null`) and the tree has no LICENSE file. Go 1.25,
chi v5.3.0, gorm v1.30.0, zap v1.27.0 (GB `go.mod`).

### Tree (files that matter)

```
cmd/api/main.go
internal/app/bootstrap.go            wiring: repos -> services -> handlers -> router
internal/app/server.go               http.Server + signal-based graceful shutdown
internal/config/config.go
internal/routes/routes.go            global middleware, docs, module route registration
internal/platform/{database,httperror,logger,middleware,response,validator}/
internal/modules/health/interfaces/http/{handler.go,routes.go}
internal/modules/user/application/{service.go,service_test.go}
internal/modules/user/application/commands/{create_user.go,update_user.go}
internal/modules/user/application/queries/{get_user.go,list_users.go}
internal/modules/user/domain/{entity.go,errors.go,repository.go,value_objects.go}
internal/modules/user/infrastructure/persistence/{gorm_model.go,mapper.go,repository.go}
internal/modules/user/interfaces/http/{handler.go,routes.go,handler_test.go}
internal/modules/user/interfaces/http/dto/{request.go,response.go}
migrations/20260101000001_create_users_table.sql
tests/integration/{testcontainer.go,user_repository_test.go}
```

Source: tree listing at https://github.com/meghrazchi/go-boilerplate/tree/836d02be2e8bef4cf21a481614cc3c81678ae922

### Commands and queries in the application layer

- A command or query is a bare struct in its own file: GB `internal/modules/user/application/commands/create_user.go`
  (`CreateUserCommand{Name, Email}`) and GB `internal/modules/user/application/queries/get_user.go`
  (`GetUserQuery{ID uuid.UUID}`).
- All use cases are methods of one struct, GB `internal/modules/user/application/service.go`:
  `Service.CreateUser`, `GetUser`, `ListUsers`, `UpdateUser`, `DeleteUser`. Each takes a command or query
  value. There is no bus, no handler-per-command type, no read model: the split is a naming and
  folder convention only. Query normalisation (page, limit, sort) lives in `Service`.
- The repository interface is declared in the domain: GB `internal/modules/user/domain/repository.go`;
  GORM implements it in GB `internal/modules/user/infrastructure/persistence/repository.go`.
- Documented dependency direction (README "DDD layer dependencies"):
  `interfaces/http -> application -> domain` and `infrastructure -> domain`
  (https://github.com/meghrazchi/go-boilerplate/blob/836d02be2e8bef4cf21a481614cc3c81678ae922/README.md).
  Nothing enforces it: GB `.golangci.yml` has no depguard rule.

### How modules depend on each other

They do not. `grep` of the module tree finds no import from one module into another. Only
GB `internal/app/bootstrap.go` and GB `internal/routes/routes.go` import module packages (six import
lines in `internal/app`, `internal/routes` and `cmd`). The README "Adding a new module" list has eight
steps; steps 5 and 6 edit `bootstrap.go` and the router, so the central wiring file grows with every
module. The repo is silent on module-to-module calls, events or shared kernels.

### The health module (interfaces layer only)

- GB `internal/modules/health/interfaces/http/handler.go`: `Handler{db *sql.DB}`; `Health` always
  answers 200; `Ready` runs `PingContext` with a 2 second timeout and answers 503 on failure.
- GB `internal/modules/health/interfaces/http/routes.go`: `RegisterRoutes(r chi.Router, h *Handler)`
  registers `/health` and `/ready`.
- There is no domain, application or infrastructure folder: the handler holds `*sql.DB` directly and
  probes only Postgres. This is the pattern to keep for a health module that probes several
  dependencies, but with a small port interface instead of `*sql.DB` so the module stays testable.

### Bootstrap and wiring kept apart from use cases

- GB `cmd/api/main.go`: config, logger, DB connect, then `app.Bootstrap(...)` and `app.Serve(...)`.
- GB `internal/app/bootstrap.go`: hand-written constructor calls
  (`userpersistence.NewGormRepository(db)` -> `userapp.NewService(repo)` -> `userhttp.NewHandler(...)`),
  returns `routes.NewRouter(routes.Dependencies{...})`. No DI framework.
- GB `internal/app/server.go`: `http.Server` with timeouts, `signal.Notify(SIGINT, SIGTERM)`,
  `server.Shutdown(ctx)` with a configured timeout.
- Application code never imports `config`, `routes` or `app`. That separation is the part worth
  keeping.

### Tests

Hand-written in-memory repositories (GB `internal/modules/user/application/service_test.go`) and
`httptest` handler tests (GB `internal/modules/user/interfaces/http/handler_test.go`). No testify or
gomock imports in the repo's own code (testify is indirect, via testcontainers). Integration tests use
`//go:build integration` and testcontainers (GB `tests/integration/testcontainer.go`) but create the
schema with `db.AutoMigrate`, not with the goose files.

### Conflicts with the project rules

| Project rule                                                             | go-boilerplate                                                                                                                         | Action                                                                     |
| ------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| stdlib `net/http`                                                        | chi v5 router: GB `internal/routes/routes.go`, GB `internal/modules/user/interfaces/http/routes.go`                                    | Use `http.ServeMux` patterns (or Huma humago).                             |
| `log/slog`                                                               | zap: GB `internal/platform/logger/zap.go`, `*zap.Logger` injected into handlers                                                        | Use slog.                                                                  |
| No ORM                                                                   | GORM: GB `internal/modules/user/infrastructure/persistence/gorm_model.go`, `mapper.go`, GB `internal/platform/database/transaction.go` | sqlc and pgx.                                                              |
| goose                                                                    | goose binary in GB `scripts/migrate-up.sh`; but tests use `AutoMigrate`                                                                | Keep goose in tests too.                                                   |
| No testify, no gomock                                                    | Compatible (hand-written fakes)                                                                                                        | Keep the fakes style.                                                      |
| No DI framework                                                          | Compatible (hand wiring)                                                                                                               | Keep.                                                                      |
| No globals, no `init()`                                                  | No `init()`; only sentinel error vars in GB `internal/modules/user/domain/errors.go`                                                   | Compatible (sentinel errors are allowed by the typed-error rule).          |
| RFC 9457 problem+json                                                    | Custom envelope: GB `internal/platform/response/response.go`, GB `internal/platform/httperror/error.go`                                | Do not copy; map domain errors to problem+json.                            |
| Config via caarlos0/env                                                  | godotenv + kelseyhightower/envconfig: GB `go.mod`                                                                                      | Keep caarlos0/env.                                                         |
| No `utils`/`common`/`helpers`                                            | Compatible (`internal/platform/*` names are functional)                                                                                | Keep.                                                                      |
| Ports in `internal/app/ports.go`; domain imports nothing from `internal` | Repository interface lives in domain; adapter layer is called `infrastructure` and imports only `domain`                               | Keep ports in the module's application package; name the layer `adapters`. |
| Dependency rules enforced by depguard                                    | Not enforced                                                                                                                           | Extend the depguard config per module.                                     |
| Pinned, no CDN at runtime                                                | Swagger UI loaded from jsDelivr, spec read from `docs/openapi.yaml` in the working directory (GB `internal/routes/routes.go`)          | Do not copy.                                                               |
| Licence                                                                  | None                                                                                                                                   | Do not copy code.                                                          |

## (b) evrone/go-clean-template

Repo: https://github.com/evrone/go-clean-template. Default branch `master`, read at e72dda1c. MIT.
Go 1.27, Fiber v2.52.15, nats.go v1.54.0, amqp091-go v1.15.0, grpc v1.84.0, testify v1.12.1,
go.uber.org/mock v0.6.0, golang-migrate v4.20.1, zerolog (EV `go.mod`, EV `pkg/logger/logger.go`).

### Controller folder per transport, versioned by folder

```
internal/controller/restapi/{router.go,middleware/,v1/{router.go,controller.go,request/,response/,*.go}}
internal/controller/grpc/{router.go,middleware/,v1/{router.go,controller.go,*_controller.go,response/}}
internal/controller/amqp_rpc/{router.go,v1/{router.go,controller.go,request/,response/,*.go}}
internal/controller/nats_rpc/{router.go,v1/{router.go,controller.go,request/,response/,*.go}}
```

- Each transport has its own `request/` and `response/` DTO packages per version
  (EV `internal/controller/amqp_rpc/v1/request/task.go`, EV `.../response/error.go`).
- Versioning is copy-the-folder: EV `README.md` ("Simple RPC versioning. For v2, we will need to add
  the `amqp_rpc/v2` folder with the same content" and one line in `router.go`). REST uses
  `app.Group("/v1")` (EV `internal/controller/restapi/router.go`); gRPC uses `docs/proto/v1` plus
  `grpc/v1` (EV `internal/controller/grpc/router.go`); RPC handler keys are strings such as
  `v1.task.create` (EV `internal/controller/amqp_rpc/v1/router.go`).
- Controllers never import each other (EV `AGENTS.md`, "Dependency rule").

### Use case interfaces

EV `internal/usecase/contracts.go` declares `Translation`, `User` and `Task` interfaces that the
controllers consume; EV `internal/repo/contracts.go` declares what use cases need. Both carry a
`//go:generate mockgen` line and the tests use the generated `mocks_*_test.go`. Interfaces sit next to the
implementations (producer side). The project puts driven ports in `internal/app/ports.go` and
should declare the driving interface in the transport package that consumes it.

### AMQP RPC request/reply (EV `pkg/rabbitmq/rmq_rpc/`)

| Aspect          | What the code does                                                                                                                                                                                                                                                          | Source                                                                          |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Topology        | Both sides declare a non-durable `fanout` exchange and an anonymous exclusive queue bound to it (`rpc_server`, `rpc_client`).                                                                                                                                               | EV `pkg/rabbitmq/rmq_rpc/connection.go`                                         |
| Request message | `Publish` to the server exchange with `Type` = handler key, `CorrelationId` = new UUID, `ReplyTo` = client exchange, `ContentType` application/json, trace context in `Headers`.                                                                                            | EV `pkg/rabbitmq/rmq_rpc/client/client.go`                                      |
| Reply message   | `Publish` to `d.ReplyTo` with the same `CorrelationId`; `Type` carries the status: `success`, `internal server error` or `unregistered handler`; body is JSON on success, empty otherwise.                                                                                  | EV `pkg/rabbitmq/rmq_rpc/server/server.go`                                      |
| Router          | `map[string]server.CallHandler`, `CallHandler func(context.Context, *amqp.Delivery) (any, error)`; filled by `v1.NewRoutes`.                                                                                                                                                | EV `internal/controller/amqp_rpc/v1/router.go`                                  |
| Error handling  | Server logs the real error and sends only the generic status; client maps to `ErrInternalServer` or `ErrBadHandler`. Any other status falls through the `switch` and returns `nil` (silent success). Validation, not-found and conflict all become "internal server error". | EV `pkg/rabbitmq/rmq_rpc/client/client.go`, EV `pkg/rabbitmq/rmq_rpc/errors.go` |
| Timeouts        | Client waits `_defaultTimeout` = 2 s on a timer, then returns `ErrTimeout`. `Server.timeout` is stored and settable (`options.go`) but never applied to a handler. `RemoteCall` takes no `context.Context`.                                                                 | EV `pkg/rabbitmq/rmq_rpc/client/client.go`, `.../server/server.go`              |
| Concurrency     | `errgroup` with `SetLimit(1)` and a loop that calls `serveCall` inline: one request at a time per server.                                                                                                                                                                   | EV `pkg/rabbitmq/rmq_rpc/server/server.go`                                      |
| Reconnect       | Re-dial only when the `Delivery` channel closes: 10 attempts, 5 s apart; failure is sent on `Notify()`.                                                                                                                                                                     | EV `pkg/rabbitmq/rmq_rpc/connection.go`                                         |
| Ack             | `defer ack` after the handler, success or failure (a failed request is never redelivered).                                                                                                                                                                                  | EV `pkg/rabbitmq/rmq_rpc/server/server.go`                                      |

Consequence: with N server replicas every request runs N times, because each replica has its own
exclusive queue bound to the same fanout exchange. For ledger writes that is unacceptable. RabbitMQ
documents the usual alternative, one shared durable request queue (competing consumers) and the
`amq.rabbitmq.reply-to` pseudo-queue for replies: https://www.rabbitmq.com/docs/direct-reply-to

### NATS RPC request/reply (EV `pkg/nats/nats_rpc/`)

| Aspect          | What the code does                                                                                                                                                                                                                                                               | Source                                                                             |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Request         | `connection.RequestMsg` to the server subject with header `Handler` = route key and trace context in headers; JSON body.                                                                                                                                                         | EV `pkg/nats/nats_rpc/client/client.go`                                            |
| Reply           | `PublishMsg` to `msg.Reply` with header `Status`; same three status strings as AMQP.                                                                                                                                                                                             | EV `pkg/nats/nats_rpc/server/server.go`                                            |
| Subscribe       | `connection.Subscribe(subject, handler)`, not `QueueSubscribe`, so every replica answers every request.                                                                                                                                                                          | EV `pkg/nats/nats_rpc/server/server.go`                                            |
| Timeout         | Client passes a fixed 2 s to `RequestMsg`, then checks `errors.Is(err, context.DeadlineExceeded)`. nats.go returns `ErrTimeout` ("nats: timeout") from `newRequest`, so that branch never matches and a timeout surfaces as a wrapped generic error. `Server.timeout` is unused. | EV `pkg/nats/nats_rpc/client/client.go`; NG `nats.go` (`ErrTimeout`, `newRequest`) |
| Connection loss | `nats.Connect` with `MaxReconnects(10)`; no `ClosedHandler`, so the server never reports a dead connection through `Notify()`.                                                                                                                                                   | EV `pkg/nats/nats_rpc/server/server.go`                                            |

NATS ships a service framework that already does queue-group load balancing and an error header:
`DefaultQueueGroup = "q"`, `ErrorHeader = "Nats-Service-Error"` in NG `micro/service.go`
(package list at https://github.com/nats-io/nats.go/tree/v1.55.0, directory `micro`).

### Each server in its own package, one place for startup and shutdown

- Packages: EV `pkg/httpserver/server.go`, EV `pkg/grpcserver/server.go`, EV `pkg/rabbitmq/rmq_rpc/server/server.go`,
  EV `pkg/nats/nats_rpc/server/server.go`. All expose `Start()`, `Notify() <-chan error`, `Shutdown() error`
  and functional options (`options.go`).
- Orchestration is only in EV `internal/app/app.go`: `initUseCases` builds repositories and use cases,
  `initServers` builds the four routers and servers, `startServers()` starts them,
  `waitForShutdown()` selects on the OS signal or any server's `Notify()`, `shutdownServers()` stops
  http, grpc, rmq, nats in that order, logging each error.
- Gaps to fix when reusing the shape: servers use `context.Background()`; shutdown is sequential with no
  overall deadline and `grpcserver.Shutdown` calls `GracefulStop()` with no timeout (EV
  `pkg/grpcserver/server.go`); constructor failures call `l.Fatal`, which is `os.Exit(1)` and skips
  deferred cleanup (EV `pkg/logger/logger.go`, EV `internal/app/app.go`). Use `signal.NotifyContext`
  plus an `errgroup` and one `Run(ctx) error` per server.

### Integration tests against the docker stack

EV `docker-compose-integration-test.yml` adds an `integration-test` service (image `golang:1.27-alpine3.23`)
that runs `go test -v ./integration-test/...` inside the compose network, started with
`make compose-up-integration-test` (`--abort-on-container-exit --exit-code-from integration-test`)
(EV `Makefile`). The tests poll `/healthz` (20 attempts) and call REST, gRPC, RabbitMQ and NATS through
the real clients (EV `integration-test/helpers_test.go`). EV `AGENTS.md` warns that
`make integration-test` on the host always fails because the container host names do not resolve there.

### Conflicts with the project rules

| Project rule                           | evrone                                                                                              | Action                                                    |
| -------------------------------------- | --------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| stdlib `net/http`                      | Fiber v2 (fasthttp): EV `pkg/httpserver/server.go`, EV `internal/controller/restapi/router.go`      | Do not copy the REST layer.                               |
| `log/slog`                             | zerolog behind a custom interface: EV `pkg/logger/logger.go`                                        | Use slog.                                                 |
| No testify, no gomock                  | testify and `mockgen` mocks: EV `internal/usecase/mocks_usecase_test.go`, `go.mod`                  | Hand-written fakes with go-cmp.                           |
| No ORM                                 | Squirrel query builder over pgx, not an ORM: EV `pkg/postgres/postgres.go`                          | Compatible in spirit; project uses sqlc.                  |
| goose                                  | golang-migrate, `func init()` runs migrations under `-tags migrate`: EV `internal/app/migrate.go`   | Keep goose, no `init()`.                                  |
| No globals / `init()`                  | `init()` in EV `internal/app/migrate.go` and generated `docs/docs.go` (swaggo); sentinel error vars | Do not copy the swaggo setup.                             |
| No `utils`/`common`/`helpers`          | `pkg/` holds named packages only                                                                    | Compatible.                                               |
| No DI framework                        | Hand wiring in `internal/app/app.go`                                                                | Compatible.                                               |
| `context` first parameter              | `RemoteCall(handler string, request, response any)` has no context                                  | Add `ctx` first.                                          |
| RFC 9457                               | Per-transport `{"error": "..."}` and generic status strings                                         | Define one domain-error to transport-status table.        |
| Testcontainers-based integration tests | Compose-network test container                                                                      | Keep testcontainers; a compose e2e can be an extra layer. |
| Spec is the contract                   | swaggo comments generate the spec: EV `internal/controller/restapi/router.go`                       | Superseded by Huma.                                       |

## (c) Libraries and tools

All Go floors are at or below the project's Go 1.27 (`apps/api/go.mod`: `go 1.27.0`, toolchain go1.27.3).
Versions come from `go list -m <mod>@latest`, `npm view`, and the GitHub releases API on 2026-10-25.

### Go libraries

| Library                                                       | Latest stable (date)                                | Licence                                                                     | go floor | cgo                                                                                              | Maturity notes and release page                                                                                                               |
| ------------------------------------------------------------- | --------------------------------------------------- | --------------------------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| github.com/danielgtaylor/huma/v2                              | v2.39.1 (2026-07-29)                                | MIT                                                                         | 1.25.0   | No: the root and humago packages import only stdlib and Huma subpackages                         | About 4.5k stars, pushed 2026-10-23 (https://github.com/danielgtaylor/huma). https://github.com/danielgtaylor/huma/releases/tag/v2.39.1       |
| github.com/99designs/gqlgen                                   | v0.17.98 (2026-10-22)                               | MIT                                                                         | 1.26.0   | No                                                                                               | Still 0.x with very frequent releases; websocket via coder/websocket (GQ `go.mod`). https://github.com/99designs/gqlgen/releases/tag/v0.17.98 |
| google.golang.org/grpc                                        | v1.84.0 (2026-09-17)                                | Apache-2.0                                                                  | 1.25.0   | No                                                                                               | https://github.com/grpc/grpc-go/releases/tag/v1.84.0                                                                                          |
| google.golang.org/protobuf                                    | v1.36.12 (2026-08-10); v1.37.0-rc.1 is a prerelease | BSD-3-Clause                                                                | 1.23     | No                                                                                               | https://github.com/protocolbuffers/protobuf-go/releases/tag/v1.36.12                                                                          |
| google.golang.org/grpc/cmd/protoc-gen-go-grpc                 | v1.6.2 (2026-05-11)                                 | Apache-2.0                                                                  | 1.25.0   | No                                                                                               | Separate module and tag line: https://github.com/grpc/grpc-go/releases/tag/cmd/protoc-gen-go-grpc/v1.6.2                                      |
| github.com/rabbitmq/amqp091-go                                | v1.16.0 (2026-10-19)                                | BSD-2-Clause (LICENSE text has two conditions; GitHub API says NOASSERTION) | 1.20     | No: go.mod requires only goleak (test)                                                           | https://github.com/rabbitmq/amqp091-go/releases/tag/v1.16.0                                                                                   |
| github.com/nats-io/nats.go                                    | v1.55.0 (2026-10-23)                                | Apache-2.0                                                                  | 1.26.0   | No: deps are klauspost/compress, nkeys, nuid, x/crypto                                           | https://github.com/nats-io/nats.go/releases/tag/v1.55.0                                                                                       |
| github.com/twmb/franz-go                                      | v1.22.2 (2026-10-13, tag only)                      | BSD-3-Clause                                                                | 1.26.0   | No: README "Written in pure Go (no wrapper lib for a C library)"                                 | https://github.com/twmb/franz-go/tags                                                                                                         |
| github.com/segmentio/kafka-go                                 | v0.4.51 (2026-04-23)                                | MIT                                                                         | 1.23     | No                                                                                               | Last commit 2026-04-23; 271 open issues+PRs. https://github.com/segmentio/kafka-go/releases/tag/v0.4.51                                       |
| github.com/confluentinc/confluent-kafka-go/v2                 | v2.16.0 (2026-10-07)                                | Apache-2.0                                                                  | 1.26.0   | Yes: wraps librdkafka; README: "`CGO_ENABLED` must NOT be set to `0`"; Alpine needs `-tags musl` | https://github.com/confluentinc/confluent-kafka-go/releases/tag/v2.16.0                                                                       |
| testcontainers-go and modules rabbitmq, nats, kafka, redpanda | v0.44.0 (2026-08-07)                                | MIT                                                                         | 1.25.0   | No                                                                                               | Same version as the repo's current testcontainers-go. https://github.com/testcontainers/testcontainers-go/releases/tag/v0.44.0                |

cgo status for every broker client: amqp091-go no, nats.go no, franz-go no, kafka-go no,
confluent-kafka-go yes. Sources: go.mod files at the tags above,
https://github.com/twmb/franz-go/blob/v1.22.2/README.md ,
https://github.com/segmentio/kafka-go/blob/v0.4.51/README.md (describes librdkafka wrappers as the cgo
alternative) and https://github.com/confluentinc/confluent-kafka-go/blob/v2.16.0/README.md

Registry pages for the Go versions: the module proxy serves one record per version at
`https://proxy.golang.org/<module>/@v/<version>.info`, for example
https://proxy.golang.org/github.com/danielgtaylor/huma/v2/@v/v2.39.1.info ,
https://proxy.golang.org/github.com/99designs/gqlgen/@v/v0.17.98.info ,
https://proxy.golang.org/google.golang.org/grpc/@v/v1.84.0.info ,
https://proxy.golang.org/google.golang.org/protobuf/@v/v1.36.12.info ,
https://proxy.golang.org/google.golang.org/grpc/cmd/protoc-gen-go-grpc/@v/v1.6.2.info ,
https://proxy.golang.org/github.com/rabbitmq/amqp091-go/@v/v1.16.0.info ,
https://proxy.golang.org/github.com/nats-io/nats.go/@v/v1.55.0.info ,
https://proxy.golang.org/github.com/twmb/franz-go/@v/v1.22.2.info ,
https://proxy.golang.org/github.com/testcontainers/testcontainers-go/@v/v0.44.0.info .

### Huma v2 notes (HU = v2.39.1)

- humago adapter: HU `adapters/humago/humago.go`. `humago.New(mux, huma.DefaultConfig(...))` takes any
  `Mux` with `HandleFunc` and `ServeHTTP` (so `*http.ServeMux`). It registers `METHOD path` patterns
  and reads path params with `r.PathValue`, i.e. Go 1.22+ routing. `NewWithPrefix` adds a prefix that is
  not written into the spec; set `config.Servers` to match.
- OpenAPI version: `DefaultConfig` emits `3.1.0` (HU `defaults.go`). With `OpenAPIPath` `/openapi` it
  serves `/openapi.json`, `/openapi.yaml`, `/openapi-3.0.json`, `/openapi-3.0.yaml`; the 3.0.3 variants
  come from `OpenAPI().Downgrade()` and `DowngradeYAML()` (HU `api.go`, HU `openapi.go`;
  https://github.com/danielgtaylor/huma/blob/v2.39.1/docs/docs/features/openapi-generation.md).
- Spec without a server: register operations on `humago.New(http.NewServeMux(), cfg)` and call
  `api.OpenAPI().YAML()`; nothing listens. Huma's own tutorial does this with a CLI `openapi` command
  (https://github.com/danielgtaylor/huma/blob/v2.39.1/docs/docs/tutorial/client-sdks.md). Byte-stable
  output is not documented; check it in a spike because CI will diff the file.
- Error model: `ErrorModel` has `type`, `title`, `status`, `detail`, `instance`, `errors[]`, and its
  `ContentType` method returns `application/problem+json` (HU `error.go`); the docs call it RFC 9457
  (https://github.com/danielgtaylor/huma/blob/v2.39.1/docs/docs/features/response-errors.md). Custom
  model: implement `huma.StatusError` (`GetStatus`, `Error`), optionally `huma.ContentTypeFilter`, then
  assign `huma.NewError` (and `huma.NewErrorWithContext` to read the request) in `main`. Extension
  fields (for example `code`, `request_id`) are plain fields on your struct. Huma adds a `default`
  response with the error schema to each operation (HU `huma.go`, around the `NewError(0, "")` call).
- `DefaultConfig` also installs `NewSchemaLinkTransformer`: JSON bodies gain a `$schema` property and
  responses a `Link` header (HU `defaults.go`, `CreateHooks`). Clear `CreateHooks` if responses must be
  plain.
- Nullability: `DefaultArrayNullable = true` (HU `schema.go`, line 29) makes slices `["array","null"]`;
  pointer scalars are nullable by default. Set the variable to false at startup, then check generated
  TypeScript for `T[] | null`.
- Operation IDs and tags: set `huma.Operation{OperationID, Tags, ...}` explicitly; the default
  generator is `huma.GenerateOperationID` (overridable global) (HU `docs/docs/features/operations.md`).
- Header parameters: input struct fields tagged `header:"Idempotency-Key"` (add `required:"true"`
  where needed) and output struct `header:"..."` fields; documented in HU
  `docs/docs/features/request-inputs.md`. A header on every response (`X-Request-Id`) is best set in
  middleware; to list it in the spec use `Config.OnAddOperation` (the hook the link transformer uses).
- Docs page: `DocsPath` (`/docs` by default) serves Stoplight Elements, Scalar or Swagger UI from
  unpkg with pinned URLs and SRI hashes (HU `api.go`, `DocsRenderer*`); set `DocsPath = ""` to turn it
  off. Same for `OpenAPIPath` and `SchemasPath`.

### gqlgen notes (GQ = v0.17.98)

- Config reference: GQ `docs/content/config.md`.
- Resolver layout `follow-schema` writes one `{name}.resolvers.go` per schema file into one package
  (`resolver.dir`, `filename_template: "{name}.resolvers.go"`); `exec.layout: follow-schema` splits the
  generated executor the same way. Per-module split: one `.graphqls` per module (`schema` accepts
  globs, and SDL `extend type Query` merges, https://spec.graphql.org/October2021/#sec-Object-Extensions),
  which yields one resolver file per module in one Go package. `gqlgen.yml` has a single
  `resolver.package` and `resolver.dir`, so resolvers for all modules land in one package; their
  bodies should delegate to each module's application API.
- `autobind: [<package paths>]` makes gqlgen bind schema types to Go types with the same name in those
  packages (for example a module's GraphQL DTO package); `models:` binds individual types and scalars.
- Custom scalars: built-in helpers `Int64`, `Time`, `UUID`, `Map`, `Any`, `Upload`, `Duration`; user types
  implement `graphql.Marshaler` and `graphql.Unmarshaler` (or the Context variants), mapped under
  `models:` (GQ `docs/content/reference/scalars.md`). Amounts that must stay bigint-safe need a custom
  string scalar, not `Int64`.
- Runtime is stdlib-friendly: `handler.New(es)` returns `*Server` implementing `http.Handler`
  (GQ `graphql/handler/server.go`). The playground loads GraphiQL and React from jsDelivr
  (GQ `graphql/playground/playground.go`); keep it dev-only.
- Running the generator as a `go tool` puts gqlgen's dependencies (x/tools, urfave/cli) in `go.mod`.

### gRPC, protobuf and buf

- grpc-go v1.84.0 and protobuf-go v1.36.12 are libraries; `protoc-gen-go` lives in the protobuf module
  (`cmd/protoc-gen-go`), `protoc-gen-go-grpc` is a separate module (go 1.25.0, requires grpc v1.81.0 and
  protobuf v1.36.11 in its own go.mod).
- buf v1.75.0 (2026-10-22, Apache-2.0): https://github.com/bufbuild/buf/releases/tag/v1.75.0. A `buf.gen.yaml`
  plugin with `local:` runs a binary found on `$PATH` (https://buf.build/docs/configuration/v2/buf-gen-yaml/),
  so the generators are not fetched from the BSR at generate time.
- Pinning as binaries with checksums (matches the sqlc block in `scripts/install-tools.sh`): buf ships
  `sha256.txt` and `sha256.txt.minisig` per release; protoc-gen-go ships per-platform tarballs on the
  protobuf-go release page; protoc-gen-go-grpc ships tarballs on its own release. The sha256 values
  for the four platforms the script supports are in Appendix A and were cross-checked against the
  release checksum files where one exists.
- Alternative that needs no binary download: a `tool` block in `go.mod` (Go 1.24 and later,
  https://go.dev/doc/go1.24#go-command), which is how evrone pins `protoc-gen-go`,
  `protoc-gen-go-grpc`, mockgen and migrate (EV `go.mod`). go.sum then holds the checksums, but the
  tool dependencies enter the module graph.

### oasdiff and just

- oasdiff v1.34.1 (2026-10-21, Apache-2.0, go 1.26.0): https://github.com/oasdiff/oasdiff/releases/tag/v1.34.1.
  `oasdiff breaking <base> <revision>` lists breaking changes only, `changelog` lists all consumer-visible
  changes, `diff` is the full machine-readable diff; `--fail-on ERR|WARN|INFO` sets the exit code
  (https://github.com/oasdiff/oasdiff/blob/main/docs/BREAKING-CHANGES.md). OpenAPI 3.1 is supported
  since v1.15.0 (https://github.com/oasdiff/oasdiff/blob/main/docs/OPENAPI-31.md). Release assets
  include `checksums.txt`; archives for darwin (universal), linux amd64 and linux arm64.
- just 1.59.0 (2026-10-15, CC0-1.0): https://github.com/casey/just/releases/tag/1.59.0. Rust binary, release
  assets for aarch64 and x86_64 on macOS and Linux (musl) plus `SHA256SUMS`. Supports modules
  (`mod`), recipe groups and `.env` loading (https://github.com/casey/just/blob/master/README.md and
  the manual at https://just.systems/man/en/).

### Broker clients, short comparison

| Client             | cgo | Notes                                                                                                                                                                                                                                                           |
| ------------------ | --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| franz-go           | No  | Kafka 0.8.0 through 4.4+, idempotent and transactional producers, exactly-once consume-transform-produce, works with Redpanda (README). Has `kadm` (admin, v1.19.0) and `kfake` (in-process fake cluster, no tagged release, pseudo-version only). Recommended. |
| kafka-go           | No  | Simple API, MIT, but the last release and commit are from April 2026 and 271 issues and PRs are open.                                                                                                                                                           |
| confluent-kafka-go | Yes | librdkafka performance and features, but `CGO_ENABLED=1`, per-platform prebuilt library, `-tags musl` on Alpine, and the Go floor is "1.26.10+ or 1.27.3+" (README line "Supports Go 1.26.10+ (or 1.27.3+)"). Conflicts with static, cgo-free builds.           |

amqp091-go and nats.go are the only sensible clients for RabbitMQ and NATS; amqp091-go has no automatic
reconnect, so a wrapper must watch `NotifyClose` and re-declare topology. TigerBeetle CDC speaks AMQP
0.9.1, which is what amqp091-go implements (see (e)).

### TypeScript generators and the TypeScript compiler API

Context from the repo: root TypeScript is 7.0.2; `packages/openapi` pins `typescript` 5.9.3 for
`openapi-typescript` 7.13.0 (`packages/openapi/package.json`).

TypeScript 7.0 ships no programmatic API: "While TypeScript 7.0 is here, it does not ship with an API.
We expect TypeScript 7.1 to ship with a new (and different) API". The compatibility package
`@typescript/typescript6` re-exports the 6.0 API and provides `tsc6`; the post recommends npm aliases
(`"typescript": "npm:@typescript/typescript6@^6.0.2"`) and a second alias for 7.0's `tsc`
(https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/). The 7.1 beta
(2026-10-20) adds entry points `typescript/sync`, `typescript/async`, `typescript/ast/factory` and others;
generators have to port to them, because they are not the 6.x default export where `ts.factory` and
`createPrinter` live today
(https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/). In the npm package,
`require("typescript")` only resolves `lib/version.cjs`
(`npm view typescript@7.0.2 exports`; https://www.npmjs.com/package/typescript).

Verification method for each row: scanned every published `.js/.mjs/.cjs` file through jsDelivr for
`from "typescript"`, `require("typescript")` and `import("typescript")`, and read `npm view` peer and
dependency lists.

| Generator                                                                                                                  | Latest stable                                 | Needs the TS compiler API?                                                                                                                                                                                                                                                                                                                                                           | Evidence                                                                                                                                                |
| -------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| @hey-api/openapi-ts                                                                                                        | 0.99.0 (2026-06-22, MIT)                      | Yes. Bundle `dist/init-D6Y8JFUS.mjs` has `from "typescript"`, 133 uses of `ts.factory` and `createPrinter`. The peer range `typescript >=5.5.3 \|\| >=6.0.0 \|\| 6.0.1-rc` nominally accepts 7.0.2, so pnpm will not warn, but TypeScript 7.0.2 exports no `ts.factory` and the generator would fail at run time.                                                                    | https://cdn.jsdelivr.net/npm/@hey-api/openapi-ts@0.99.0/dist/init-D6Y8JFUS.mjs ; `npm view @hey-api/openapi-ts peerDependencies`                        |
| @hey-api/openapi-ts `next`                                                                                                 | 0.0.0-next-20260930190945 (prerelease)        | No. No `typescript` import and no peer; the repo has its own `ts-compiler` printer (commit cb7e283a, 2026-07-02, "refactor: printer"). Not a stable release.                                                                                                                                                                                                                         | https://cdn.jsdelivr.net/npm/@hey-api/openapi-ts@0.0.0-next-20260930190945/dist/init-CxIMDyRp.mjs ; HA `packages/openapi-ts/src/ts-compiler/printer.ts` |
| openapi-typescript                                                                                                         | 7.13.0 (2026-02-11, MIT)                      | Yes. 28 of 38 dist files import `typescript` (`dist/lib/ts.mjs`, `dist/transform/*.mjs`); peer `typescript ^5.x`. Keep it isolated on 5.9.3 as today.                                                                                                                                                                                                                                | https://cdn.jsdelivr.net/npm/openapi-typescript@7.13.0/dist/lib/ts.mjs                                                                                  |
| openapi-fetch                                                                                                              | 0.17.0 (2026-02-11, MIT)                      | No (runtime only; its only dependency is `openapi-typescript-helpers`).                                                                                                                                                                                                                                                                                                              | `npm view openapi-fetch dependencies`                                                                                                                   |
| orval                                                                                                                      | 8.45.0 (2026-10-25), 8.44.0 (2026-10-21), MIT | No for `@orval/core`, `@orval/fetch`, `@orval/query` (no `typescript` import in dist, no dependency). `@orval/zod` depends on its own `typescript` 6.0.3, only if the zod client is used. Needs Node >=22.18.                                                                                                                                                                        | `npm view @orval/core dependencies`; https://cdn.jsdelivr.net/npm/@orval/core@8.45.0/                                                                   |
| @graphql-codegen/cli 7.4.6, client-preset 6.2.3, typed-document-node 7.2.0, typescript 6.1.2, typescript-operations 6.1.10 | MIT, published 2026-10-05 to 2026-10-12       | No. No dist file in cli, client-preset, core, typescript, typescript-operations, typed-document-node, visitor-plugin-common or gql-tag-operations imports `typescript`; the resolved dependency graph of cli has no `typescript` (deps.dev). The cli depends on `jiti`, not `typescript`; `cosmiconfig` lists `typescript` only as an optional peer. Peer is `graphql ^0.8 ... ^17`. | https://api.deps.dev/v3/systems/npm/packages/%40graphql-codegen%2Fcli/versions/7.4.6:dependencies                                                       |

Registry pages for the npm versions: https://www.npmjs.com/package/@hey-api/openapi-ts/v/0.99.0 ,
https://www.npmjs.com/package/orval/v/8.44.0 , https://www.npmjs.com/package/orval/v/8.45.0 ,
https://www.npmjs.com/package/openapi-typescript/v/7.13.0 , https://www.npmjs.com/package/openapi-fetch/v/0.17.0 ,
https://www.npmjs.com/package/@graphql-codegen/cli/v/7.4.6 ,
https://www.npmjs.com/package/@graphql-codegen/client-preset/v/6.2.3 ,
https://www.npmjs.com/package/@graphql-codegen/typed-document-node/v/7.2.0 ,
https://www.npmjs.com/package/graphql/v/17.0.2 , https://www.npmjs.com/package/typescript/v/7.0.2 ,
https://www.npmjs.com/package/@typescript/typescript6/v/6.0.2 . Publish times come from
`https://registry.npmjs.org/<package>` (field `time`).

#### @hey-api/openapi-ts details

- Fetch client: `@hey-api/client-fetch` is bundled into the output (`client/` and `core/` folders), the
  generated `client.gen.ts` exports `client = createClient(createConfig())`; `setConfig()`, a
  `runtimeConfigPath` file exporting `createClientConfig`, per-call `client` override and request and
  response interceptors (`use`, `eject`, `update`)
  (HA `web/src/content/docs/docs/openapi/typescript/clients/fetch.mdx`). A `ky` client also exists in the
  0.99.0 package (`dist/clients/ky/`), which matches the repo's current `ky` 2.2.0 client.
- Output structure (default): `client/`, `core/`, `client.gen.ts`, `index.ts`, `sdk.gen.ts`,
  `types.gen.ts` (HA `web/src/content/docs/docs/openapi/typescript/output.mdx`).
- SDK shape: `operations.strategy` `flat` (tree-shakeable functions) or `single` (one class);
  `operations.nesting` customises the tree; the 0.99.0 bundle also implements `byTags` (one container per
  OpenAPI tag) (HA `web/src/content/docs/docs/openapi/typescript/plugins/sdk.mdx`; bundle chunk
  `init-D6Y8JFUS.mjs`). Tag splitting is by class or namespace inside a single `sdk.gen.ts`, not one file
  per tag.
- Release state: the `latest` dist-tag is still 0.99.0 (2026-06-22, four months old); newer publishes
  are `next` builds, the newest dated 2026-09-30 (`npm view @hey-api/openapi-ts dist-tags time`).

#### orval, openapi-typescript and openapi-fetch

- orval `output.mode`: `single`, `split`, `tags` (one implementation file per tag plus a shared schemas
  file), `tags-split` (a folder per tag), plus `tags-operations` variants; `client` includes `fetch`,
  `axios`, `react-query`, `zod`, `mcp` (https://orval.dev/docs/reference/configuration/output). Release
  cadence is high (8.44.0 and 8.45.0 four days apart).
- openapi-typescript plus openapi-fetch: types-only generation plus a tiny typed `fetch` wrapper
  (`createClient<paths>()`); no per-tag output; the lowest-change option because the repo already runs it.

### GraphQL client generation

`@graphql-codegen/cli` 7.4.6 with preset `client` (client-preset 6.2.3). Document modes: default (a
`TypedDocumentNode` AST, needs `graphql` at runtime to print) or `documentMode: 'string'`, which emits a
`TypedDocumentString` whose `toString()` goes straight into `fetch`, so a small fetch client needs no
GraphQL runtime in the browser bundle
(https://the-guild.dev/graphql/codegen/plugins/presets/preset-client). The standalone
`@graphql-codegen/typed-document-node` 7.2.0 plugin emits `TypedDocumentNode` objects and is already
used inside the preset. `graphql` is 17.0.2 (2026-07-03, Node `^22 || ^24 || ^25 || >=26`); all codegen
peers accept ^17. Persisted documents are available through `presetConfig.persistedDocuments`.

### Testcontainers modules (v0.44.0)

| Module           | Default or example image                                    | Notes                                                                                                                                                                                                                                                                                                                                                                                                  |
| ---------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| modules/rabbitmq | example `rabbitmq:3.7.25-management-alpine`                 | `Run(ctx, image, opts...)`.                                                                                                                                                                                                                                                                                                                                                                            |
| modules/nats     | example `nats:2.9`                                          | `Run(ctx, image, opts...)`.                                                                                                                                                                                                                                                                                                                                                                            |
| modules/kafka    | `confluentinc/confluent-local:7.5.0`                        | KRaft; the start script sources `/etc/confluent/docker/bash-config`, so it is written for Confluent images; it validates `confluent-local` tags against a 7.4.0 minimum and the docs say other images must be KRaft compatible themselves (https://github.com/testcontainers/testcontainers-go/blob/modules/kafka/v0.44.0/modules/kafka/kafka.go). Treat `apache/kafka` as unsupported by this module. |
| modules/redpanda | example `docker.redpanda.com/redpandadata/redpanda:v23.1.7` | Starts `--mode=dev-container --smp=1 --memory=1G` (https://github.com/testcontainers/testcontainers-go/blob/modules/redpanda/v0.44.0/modules/redpanda/redpanda.go), has an admin API helper and Schema Registry options (https://github.com/testcontainers/testcontainers-go/blob/modules/redpanda/v0.44.0/modules/redpanda/options.go).                                                               |

Docs: https://github.com/testcontainers/testcontainers-go/tree/v0.44.0/docs/modules . Each module is a
separate Go module at v0.44.0 with `go 1.25.0`.

## (d) Docker images

Pin by tag and digest (`image: name:tag@sha256:...`). Sizes are compressed amd64 from the Docker Hub
API. Startup times quoted are from the cited documents, not measured here.

| Image                        | Tag                                       | Digest                                                                  | Size                              | Licence                                                                                                                                | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ---------------------------- | ----------------------------------------- | ----------------------------------------------------------------------- | --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| rabbitmq (management)        | 4.4.0-management (2026-10-20)             | sha256:c200fd496482cda18c412be4e6f244830e1e75949b8cf5f0e887cea9ea456109 | 136 MB                            | MPL 2.0, some files Apache 2.0                                                                                                         | https://hub.docker.com/_/rabbitmq . 4.4 community support to 2027-07-31; 4.3 to 2027-01-31 (https://www.rabbitmq.com/release-information). Conservative alternative 4.3.7-management, sha256:79fb281ef7e0a15e3c5e0e12e66e944cebbf14a129493ef464369abd2c48d01a, 119 MB. Licence: https://github.com/rabbitmq/rabbitmq-server/blob/main/LICENSE                                                                                                                                      |
| nats                         | 2.15.1-alpine (server release 2026-10-09) | sha256:7ae34c25dfd6fe2e01c6483eabef20d796690fdcac6c4d94ca780a6cf9877fde | 12 MB (non-alpine 2.15.1 is 7 MB) | Apache-2.0                                                                                                                             | https://hub.docker.com/_/nats . JetStream is off unless started with `-js`. https://github.com/nats-io/nats-server/releases/tag/v2.15.1                                                                                                                                                                                                                                                                                                                                            |
| apache/kafka                 | 4.3.2 (2026-10-20)                        | sha256:852a0d49fc4e656b022e7f23785411802c9a088c78a7c5e39f33969de9c227a7 | 239 MB                            | Apache-2.0                                                                                                                             | Official ASF image; `docker run -d apache/kafka:latest` starts a single-node KRaft broker (https://hub.docker.com/r/apache/kafka). 4.4.0 (2026-10-22, sha256:833b867a9585a141da455dc2b57431c65cbccb27f11c91f694444cca9f777e0f, 240 MB) is three days old.                                                                                                                                                                                                                          |
| apache/kafka-native          | 4.3.2                                     | sha256:7c4fd2ffcbd9b468a7c672c772d355b8d5138bc76697ae5dd71c18f6bcffad10 | 59 MB                             | Apache-2.0                                                                                                                             | GraalVM native build; "experimental and intended for local development and testing purposes only" (https://github.com/apache/kafka/blob/trunk/docker/native/README.md). KIP-974 benchmark: broker start about 110-140 ms native versus about 1150-1200 ms JVM, max memory about 250 MB versus about 1 GB (https://cwiki.apache.org/confluence/display/KAFKA/KIP-974%3A+Docker+Image+for+GraalVM+based+Native+Kafka+Broker). Limits: serial GC only, no user-supplied runtime jars. |
| redpandadata/redpanda        | v26.2.6 (2026-10-23)                      | sha256:8b1076e0d61bfceb66594dc01536427ce0b529f8ead35fdab9763ab6085891ce | 122 MB                            | BSL (core) and Redpanda Community License (enterprise features): https://github.com/redpanda-data/redpanda/blob/dev/licenses/README.md | Kafka API compatible; dev flags `--mode dev-container --smp 1 --memory 1G` (testcontainers module). The licence FAQ allows any use of non-enterprise features "unless you want to offer Redpanda as a service". Startup time not measured. https://hub.docker.com/r/redpandadata/redpanda                                                                                                                                                                                          |
| confluentinc/confluent-local | 8.2.4                                     | sha256:7434d641ecf0 (prefix only)                                       | 397 MB                            | Images repo is Apache-2.0 (https://github.com/confluentinc/kafka-images)                                                               | Only image the testcontainers Kafka module accepts; largest option.                                                                                                                                                                                                                                                                                                                                                                                                                |
| bitnami/kafka                | none                                      | n/a                                                                     | n/a                               | n/a                                                                                                                                    | No public tags on `bitnami/kafka`; `bitnamilegacy/kafka` last pushed 2025-08-22 (https://hub.docker.com/r/bitnamilegacy/kafka). Not credible.                                                                                                                                                                                                                                                                                                                                      |

Registry tag records (Docker Hub API, the source of the digests and sizes above):
https://hub.docker.com/v2/repositories/library/rabbitmq/tags/4.4.0-management ,
https://hub.docker.com/v2/repositories/library/rabbitmq/tags/4.3.7-management ,
https://hub.docker.com/v2/repositories/library/nats/tags/2.15.1-alpine ,
https://hub.docker.com/v2/repositories/apache/kafka/tags/4.3.2 ,
https://hub.docker.com/v2/repositories/apache/kafka/tags/4.4.0 ,
https://hub.docker.com/v2/repositories/apache/kafka-native/tags/4.3.2 ,
https://hub.docker.com/v2/repositories/redpandadata/redpanda/tags/v26.2.6 ,
https://hub.docker.com/v2/repositories/confluentinc/confluent-local/tags/8.2.4 .

Comparison for single-node dev and test:

| Criterion                 | apache/kafka                                                                                                                                                                                                                             | apache/kafka-native                    | Redpanda                                                                                        |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Size (compressed)         | 239 MB                                                                                                                                                                                                                                   | 59 MB                                  | 122 MB                                                                                          |
| Startup                   | JVM, about 1.2 s broker start (KIP-974) plus container start                                                                                                                                                                             | Sub-second (KIP-974 author numbers)    | Not measured here and no source found; measure in the Kafka spike                               |
| Single-node ergonomics    | `docker run -d apache/kafka` gives a single-node KRaft broker (Docker Hub quick start); reaching it through a random mapped port needs a matching advertised listener, which is why the testcontainers kafka module ships a start script | Same config surface, dev and test only | Dev-container mode and an official testcontainers module with Schema Registry and admin helpers |
| Fidelity to Kafka         | Highest                                                                                                                                                                                                                                  | High (same code, AOT compiled)         | Compatible API, different implementation                                                        |
| Licence                   | Apache-2.0                                                                                                                                                                                                                               | Apache-2.0                             | BSL plus RCL                                                                                    |
| testcontainers-go v0.44.0 | Generic container only                                                                                                                                                                                                                   | Generic container only                 | Official module                                                                                 |

Recommendation: compose and CI use `apache/kafka:4.3.2` (licence-clean, same broker as production
Kafka). For integration tests use the same image through a generic container with a fixed host
port and matching advertised listener; if that proves fiddly in a spike, fall back to the Redpanda
testcontainers module with `v26.2.6`. `apache/kafka-native:4.3.2` is an optional test-speed upgrade.

## (e) TigerBeetle change data capture

Sources: https://docs.tigerbeetle.com/operating/cdc/ (read in full),
https://github.com/tigerbeetle/tigerbeetle/blob/main/CHANGELOG.md , source at tag 0.17.9
(TB `src/cdc/runner.zig`, TB `src/state_machine.zig`), and the Go client at v0.17.10
(TBGO `tb_client.go`, TBGO `bindings.go`).

### Facts

- Introduced: TigerBeetle 0.16.43, released 2025-05-30, PR #2917 "CDC support to stream TigerBeetle
  state to AMQP targets, such as RabbitMQ and other compatible brokers"; AMQP 0.9.1 was chosen over 1.0
  (CHANGELOG entry "TigerBeetle 0.16.43"). Groundwork: 0.16.29 (2025-02-24, PR #2507) "Add new indexes
  to the account balances to enable CDC". Later fixes: 0.16.45 header integers became signed, 0.16.47
  CDC for objects created before 0.16.29, 0.16.76 `--amqp-timeout-seconds` and
  `--tigerbeetle-timeout-seconds`, 0.17.5 stricter AMQP enum parsing (same CHANGELOG).
- Publishes to: an AMQP 0.9.1 server (RabbitMQ and compatible), through the `tigerbeetle amqp`
  subcommand of the same binary, with `--publish-exchange` and/or `--publish-routing-key`. Only PLAIN
  authentication; no TLS (docs recommend a TLS tunnel); `--host` is `ip:port`, "Multiple addresses
  (for clustered environments) and DNS names are not supported".
- Message format: AMQP headers `event_type`, `ledger`, `transfer_code`, `debit_account_code`,
  `credit_account_code`, plus `app_id=tigerbeetle`, `content_type=application/json`, `delivery_mode=2`
  (persistent), `timestamp` (seconds). Body is compact UTF-8 JSON: `type` (`single_phase`,
  `two_phase_pending`, `two_phase_posted`, `two_phase_voided`, `two_phase_expired`), `timestamp`
  (nanoseconds), `ledger`, `transfer{...}`, `debit_account{...}` and `credit_account{...}` with balances as
  of the event. The docs say u128 and u64 are JSON strings, yet their example shows `id` and `amount`
  as JSON numbers; verify the encoding before relying on bigint-safe parsing.
- Delivery guarantee: "at-least-once semantics ... best effort to prevent duplicate messages ... during
  crash recovery, the CDC job may replay unacknowledged messages ... It is the consumer's
  responsibility to perform idempotency checks". The runner publishes with `confirm.select`,
  `mandatory=true` and persistent delivery, and states that neither confirms nor transactions can give
  exactly-once across crashes (TB `src/cdc/runner.zig`, `.publish` branch).
- Ordering: events come from `get_change_events` in ascending timestamp order ("CDC is always in
  ascending order", TB `src/state_machine.zig`, `get_scan_from_change_events_filter`). A single publisher
  is enforced by a temporary exclusive RabbitMQ queue `tigerbeetle.internal.locker`; a second
  `tigerbeetle amqp` for the same cluster exits non-zero (docs, "High Availability").
- Resume after restart: stateless job. Progress is a persistent one-message queue
  `tigerbeetle.internal.progress` (drop-head), written at the end of every published batch; on start the
  job resumes after the last acknowledged timestamp. `--timestamp-last=<ns>` overrides it and `0`
  replays everything (docs "Event Replay"; TB `src/cdc/runner.zig` comments above `recover`). If the
  RabbitMQ queue is lost the job starts from the beginning.
- Batch and pacing: `--event-count-max` default 2730, `--idle-interval-ms` default 1000,
  `--requests-per-second-limit`, timeouts 30 s.
- `get_change_events` query: operation code 137 (`TB_OPERATION_GET_CHANGE_EVENTS`, TB
  `src/clients/c/tb_client.h`). It is not in the public requests reference
  (https://docs.tigerbeetle.com/reference/requests/). The Go client v0.17.10 exposes it:
  `GetChangeEvents(filter ChangeEventsFilter) ([]ChangeEvent, error)` with the comment "Experimental:
  GetChangeEvents API is undocumented" (TBGO `tb_client.go`), `ChangeEventsFilter{TimestampMin,
  TimestampMax, Limit}` and a `ChangeEvent` struct with the same fields as the JSON body
  (TBGO `bindings.go`).
- Version coupling: "The CDC job requires TigerBeetle cluster version 0.16.43 or greater ... The CDC job
  version must not be newer than the cluster version". Transfers created before 0.16.29 have limits
  (no `two_phase_expired`, both accounts need `flags.history`); irrelevant for a new cluster.

### Does 0.17.9 support it?

Yes. The documented floor is 0.16.43; `src/cdc/runner.zig` and `src/cdc/amqp.zig` exist at tag 0.17.9
(https://github.com/tigerbeetle/tigerbeetle/tree/0.17.9/src/cdc); the 0.17.9 CHANGELOG entry
(released 2026-07-03) lists no CDC change (https://github.com/tigerbeetle/tigerbeetle/blob/main/CHANGELOG.md);
the Go client v0.17.10 has `GetChangeEvents`. The compose
file's image entrypoint is the `tigerbeetle` binary (`deploy/compose/compose.yaml`: "The image's
entrypoint is the tigerbeetle binary"), so the CDC job can run from the same
`ghcr.io/tigerbeetle/tigerbeetle:0.17.9` image with `command: ['amqp', '--addresses=...', ...]`, which
also satisfies the "job not newer than cluster" rule. Release page:
https://github.com/tigerbeetle/tigerbeetle/releases/tag/0.17.9

### Explicit answer: Kafka without a dual-write gap, or an outbox?

1. Directly from TigerBeetle to Kafka: no. CDC publishes only to AMQP 0.9.1; there is no Kafka producer
   in the job.
2. Through a bridge: yes, for ledger events, with at-least-once delivery. Path: TigerBeetle CDC ->
   RabbitMQ durable queue -> bridge consumer (franz-go producer, `acks=all`, idempotent producer) ->
   Kafka topic. The bridge acks the RabbitMQ delivery only after Kafka acknowledges. There is no
   dual-write gap because the event source is TigerBeetle's committed state read by timestamp, not
   application code that runs after the commit: a crash after commit cannot lose an event, it only
   delays it, provided RabbitMQ keeps what it accepted (durable queue bound to the exchange; the job
   publishes with `mandatory=true` and persistent delivery). RabbitMQ durability is therefore the
   loss boundary. Duplicates can arise at two hops (CDC replay after a crash, bridge redelivery), so
   consumers must be idempotent; a natural key is the event `timestamp` (nanosecond) with `type`.
   This is a design recommendation, not a TigerBeetle statement.
3. A bridge that skips RabbitMQ and calls `GetChangeEvents` from Go is possible, but the API is
   undocumented and experimental, and the bridge must own single-writer locking and its checkpoint.
   Keeping the checkpoint in Kafka (transactional producer, read the last committed record on start)
   gives no bridge-side duplicates; keeping it in Postgres reintroduces a small dual-write window.
4. Postgres outbox: not needed for ledger events, and not sufficient on its own. The write path in
   ADR-0002 and `docs/architecture.md` marks the transaction committed (step 5) after the TigerBeetle
   commit (step 4), and the outbox row is written with that mark. A crash between the two leaves a
   committed transfer with no outbox row until the reconciler repairs it, so an outbox cannot close the
   TigerBeetle-to-Postgres gap; CDC does not depend on Postgres at all. The outbox remains the right
   tool for events born in Postgres (ledger and metadata changes, webhooks that need Postgres context)
   and for anything that must be atomic with a Postgres write.

### Trade-offs

|                              | TigerBeetle CDC (+ bridge to Kafka)                                                                                                                                                                   | Postgres outbox                                        |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| Gap after TigerBeetle commit | None for ledger events                                                                                                                                                                                | Remains until the reconciler runs                      |
| Request-path cost            | None                                                                                                                                                                                                  | One more Postgres write per transaction                |
| Transport                    | AMQP 0.9.1 only; Kafka needs RabbitMQ plus a bridge                                                                                                                                                   | Any, including Kafka directly                          |
| Payload                      | TigerBeetle fields only (ids, amount, codes, `user_data_*`, balances at event time); Postgres metadata must be joined by `user_data_128` (the API transaction id), which can race the Postgres commit | Any payload, including metadata                        |
| Delivery                     | At-least-once, consumers dedupe                                                                                                                                                                       | At-least-once unless the relay uses Kafka transactions |
| Ordering                     | Ascending TigerBeetle timestamp, single publisher                                                                                                                                                     | Per outbox ordering key, relay dependent               |
| Availability                 | Single CDC instance, restart on failure, about 1 s idle poll                                                                                                                                          | Relay can run N workers with row locks                 |
| Operations                   | Extra component (RabbitMQ), no TLS, PLAIN auth, IP-only host, progress state lives in RabbitMQ (losing it replays or needs `--timestamp-last`)                                                        | Postgres only, vacuum and backlog monitoring           |
| Version coupling             | Job must not be newer than the cluster                                                                                                                                                                | None                                                   |
| Captures out-of-band writes  | Yes (any client)                                                                                                                                                                                      | No                                                     |

Suggested split for the plan (a recommendation): ledger events through CDC and a bridge; keep the outbox
for Postgres-born domain events and webhooks.

## (f) Recommendation table

Pins as of 2026-10-25. "Flag" lists conflicts with project rules, Go 1.27, Node 24 or pnpm 12.

### Go modules (`apps/api/go.mod`)

| Item                                                     | Proposed pin | Flag                                                                                                                                                                                                                                                                     |
| -------------------------------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| github.com/danielgtaylor/huma/v2                         | v2.39.1      | Supersedes ADR-0008. Assign `huma.NewError` and `huma.DefaultArrayNullable` once in `main` (no `init()`). Decide on `$schema` link transformer and docs page (CDN). Removes oapi-codegen and `github.com/oapi-codegen/runtime` v1.8.0. go 1.25 floor is fine on Go 1.27. |
| github.com/99designs/gqlgen                              | v0.17.98     | go 1.26 floor is fine on Go 1.27. Pin as a `tool` or a pinned `go run`. 0.x semver. Generated code committed and excluded from lint.                                                                                                                                     |
| google.golang.org/grpc                                   | v1.84.0      | None.                                                                                                                                                                                                                                                                    |
| google.golang.org/protobuf                               | v1.36.12     | Do not take v1.37.0-rc.1.                                                                                                                                                                                                                                                |
| protoc-gen-go                                            | v1.36.12     | Install from release tarball or `go tool`; must match the protobuf runtime.                                                                                                                                                                                              |
| protoc-gen-go-grpc                                       | v1.6.2       | Own tag line `cmd/protoc-gen-go-grpc/v1.6.2`.                                                                                                                                                                                                                            |
| github.com/rabbitmq/amqp091-go                           | v1.16.0      | No auto-reconnect: write a small supervisor.                                                                                                                                                                                                                             |
| github.com/nats-io/nats.go                               | v1.55.0      | go 1.26 floor is fine. Prefer `micro` or queue groups for RPC.                                                                                                                                                                                                           |
| github.com/twmb/franz-go                                 | v1.22.2      | pure Go. `pkg/kfake` has no tagged release, so use it only through a pseudo-version or skip it.                                                                                                                                                                          |
| testcontainers-go and modules rabbitmq, nats, (redpanda) | v0.44.0      | Same as current. The kafka module is not usable with `apache/kafka`.                                                                                                                                                                                                     |

### Tools (`scripts/install-tools.sh`, checksums in Appendix A)

| Item    | Proposed pin | Flag                                                                                                                                                                             |
| ------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| buf     | v1.75.0      | Needs protoc-gen-go and protoc-gen-go-grpc on PATH with `local:` plugins.                                                                                                        |
| oasdiff | v1.34.1      | Darwin archive is universal (`darwin_all`). Supports 3.1.                                                                                                                        |
| just    | 1.59.0       | A justfile as the single entry point contradicts AGENTS.md ("use the `pnpm` root commands"), the CI workflows and `package.json` scripts; update AGENTS.md, ADR and CI together. |

### npm (workspace)

| Item                     | Proposed pin                                                                                                     | Flag                                                                                                                                                                                                                                                              |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| REST client generator    | `orval` 8.44.0, `mode: 'tags-split'`, `client: 'fetch'`                                                          | No TS compiler API needed. 8.45.0 is under pnpm `minimumReleaseAge` (1440 min) until 2026-10-26 12:33 UTC. Node >=22.18, fine on Node 24. Verify OpenAPI 3.1 and nullable arrays from Huma in a spike.                                                            |
| Fallback A               | `@hey-api/openapi-ts` 0.99.0                                                                                     | Needs the TS 6 API: in the generator package alias `"typescript": "npm:@typescript/typescript6@6.0.2"`; peer range `>=6.0.0` accepts it. Generated `client/` and `core/` folders must be excluded from `vp check`. Wait for a stable release without the TS peer. |
| Fallback B (zero change) | `openapi-typescript` 7.13.0 + `openapi-fetch` 0.17.0                                                             | Needs `typescript` 5.9.3 isolated in `packages/openapi` (as now); peer `^5.x`; no per-tag split.                                                                                                                                                                  |
| GraphQL codegen          | `@graphql-codegen/cli` 7.4.6, `@graphql-codegen/client-preset` 6.2.3, `graphql` 17.0.2, `documentMode: 'string'` | No TS compiler API. All published more than 2 weeks ago, so pnpm age rule passes.                                                                                                                                                                                 |
| TypeScript               | stays 7.0.2 for type-checking                                                                                    | Only the hey-api option needs a TS 6 alias, only inside one package. TS 7.1 beta (2026-10-20) brings a new API but not the 6.x one.                                                                                                                               |

### Docker images (`deploy/compose/compose.yaml`)

| Item        | Proposed pin                                                                                        | Flag                                                                                                                  |
| ----------- | --------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| RabbitMQ    | `rabbitmq:4.4.0-management@sha256:c200fd496482cda18c412be4e6f244830e1e75949b8cf5f0e887cea9ea456109` | First 4.4 release (2026-10-20); 4.3.7 is the conservative fallback. TigerBeetle CDC needs the broker reachable by IP. |
| NATS        | `nats:2.15.1-alpine@sha256:7ae34c25dfd6fe2e01c6483eabef20d796690fdcac6c4d94ca780a6cf9877fde`        | Add `-js` only if JetStream is used.                                                                                  |
| Kafka       | `apache/kafka:4.3.2@sha256:852a0d49fc4e656b022e7f23785411802c9a088c78a7c5e39f33969de9c227a7`        | Needs advertised listener config for host access. Fallback Redpanda `v26.2.6` (BSL).                                  |
| TigerBeetle | `ghcr.io/tigerbeetle/tigerbeetle:0.17.9` (unchanged)                                                | CDC job runs from the same image; Go client stays at 0.17.10; `scripts/check-versions.sh` unchanged.                  |

### Cross-cutting conflicts to carry into the plan

- Docker 25+ blocks io_uring by default and TigerBeetle needs it (existing compose comment in
  `deploy/compose/compose.yaml`); the CDC runner also uses io_uring ("Both consumer and producer run
  concurrently using `io_uring`", TB `src/cdc/runner.zig`), so the CDC container needs the same
  security option as the server container.
- ADR-0008, `docs/architecture.md`, AGENTS.md ("edit the spec, run `pnpm generate`", "`packages/openapi/openapi.yaml`
  is the contract") and CI's "`pnpm generate` must produce no diff" all change with Huma.
- GraphQL scalars for money must not be `Int` or `Int64`; use a string scalar to keep the console's
  bigint rule.
- Domain errors need one table mapping to HTTP problem+json, gRPC codes, GraphQL error extensions and
  the RPC status envelopes (evrone collapses everything to three strings).

## Appendix A: pinned checksums (sha256)

Cross-checked against each release's checksum file where one exists (just `SHA256SUMS`, oasdiff
`checksums.txt`, buf `sha256.txt`); protoc-gen-go and protoc-gen-go-grpc values are the GitHub release
asset digests.

buf v1.75.0 (https://github.com/bufbuild/buf/releases/tag/v1.75.0), asset `buf-<OS>-<arch>.tar.gz`:

```
buf-Darwin-arm64.tar.gz    5478531c4645c5b80866e87d76d73f0e8bf5a35644b12c92cbdcc408e7c8b9d3
buf-Darwin-x86_64.tar.gz   d04292081e76c19c613d2411b2d0b3619d6d49d54ce6cbcf9e12f72edce0532b
buf-Linux-aarch64.tar.gz   0fe2cf43cff973e27a09219e611d98101cb22b0fee398f10f2be2256dd7f892e
buf-Linux-x86_64.tar.gz    7168a06e29bd5eb8d1530848f242264f7a86b9b8af3b2e811570fbea0c929a30
```

just 1.59.0 (https://github.com/casey/just/releases/tag/1.59.0):

```
just-1.59.0-aarch64-apple-darwin.tar.gz            6574cea774ed4cae12e24b6aa11857de33e80dab4667cf628fd5840df96a2a9d
just-1.59.0-x86_64-apple-darwin.tar.gz             036ead9df9bc31dbae9934ae3b4236ed124d0bafcffd33439460ae084d5c9cb7
just-1.59.0-aarch64-unknown-linux-musl.tar.gz      23871b1702bfd550fb5f901778fd4290ecf2ee034a45a889fbe96d89b188086d
just-1.59.0-x86_64-unknown-linux-musl.tar.gz       409044617b6f06517c1035e55ca38c8c856ff8b650e41bc2aa1ef82d5903a23e
```

oasdiff v1.34.1 (https://github.com/oasdiff/oasdiff/releases/tag/v1.34.1):

```
oasdiff_1.34.1_darwin_all.tar.gz    64948ba30c5555d3d15b95a7f3eda22fa83c15cf37fd4dad1720c0deb0a07cb5
oasdiff_1.34.1_linux_amd64.tar.gz   75775f65b0adebd26a8e013af51a799342dfdc6262d89efa9e3b7bdf89657d90
oasdiff_1.34.1_linux_arm64.tar.gz   e8f99bbf102a4b7f235104da72d0b9d67702e78622d1d4d37169e698ee90a249
```

protoc-gen-go v1.36.12 (https://github.com/protocolbuffers/protobuf-go/releases/tag/v1.36.12):

```
protoc-gen-go.v1.36.12.darwin.arm64.tar.gz   951d1d2558bbbc8c6680dcb6d302263fa35302cb6e1fccad297a50d21d827f2b
protoc-gen-go.v1.36.12.darwin.amd64.tar.gz   1df5af4a047a54fb15b1f9c4a174562a692f55cab13e94b8fbf274e766800314
protoc-gen-go.v1.36.12.linux.arm64.tar.gz    bf385e22e18661b044f7b96ec28384f3f76aa6da10e956b450ffb949306768de
protoc-gen-go.v1.36.12.linux.amd64.tar.gz    14abe70d1557026d3dce6676b7a44d3ada91d79902c71cb075fbb8d315943320
```

protoc-gen-go-grpc v1.6.2 (https://github.com/grpc/grpc-go/releases/tag/cmd/protoc-gen-go-grpc/v1.6.2):

```
protoc-gen-go-grpc.v1.6.2.darwin.arm64.tar.gz   eefe7ad5e1d578c5d1124a9084333cb938274b7a2eb54c006e7769530119cba3
protoc-gen-go-grpc.v1.6.2.darwin.amd64.tar.gz   e891ebaf91f5ac959234cf9b323fecb14947f8b6f989989c50023da55bd55af3
protoc-gen-go-grpc.v1.6.2.linux.arm64.tar.gz    343307d42a93c0219692302296e5fa38b89a9563732511b18a93479be4b0f43d
protoc-gen-go-grpc.v1.6.2.linux.amd64.tar.gz    3d36f376624c73eb62c7a46e34c2b21a1e97684fb1812e882ba775ba05b7bfc9
```

## Appendix B: spikes the plan should schedule

1. Huma spike: generate the spec with `humago` on a bare `ServeMux`, confirm the YAML is byte-stable,
   problem+json with `code` and `request_id` extension fields, `Idempotency-Key` and `X-Request-Id`
   in the spec, `DefaultArrayNullable=false`, `CreateHooks` cleared, docs page off.
2. REST client spike: run orval 8.44.0 (`tags-split`, `fetch`) against that spec; compare with
   hey-api 0.99.0 under the `@typescript/typescript6` alias.
3. CDC spike: run `tigerbeetle amqp` from the 0.17.9 image against RabbitMQ 4.4.0 with a fixed IP, capture
   one message of each type, check how `id` and `amount` are encoded (string or number), and time a
   kill-and-restart replay.
4. Kafka spike: start `apache/kafka:4.3.2` through testcontainers with a fixed port and advertised
   listener; measure start time against `apache/kafka-native:4.3.2` and the Redpanda module.
5. RPC spike: one shared durable request queue plus `amq.rabbitmq.reply-to` on RabbitMQ, and `micro`
   with a queue group on NATS, both with context deadlines and a domain-error status envelope.
