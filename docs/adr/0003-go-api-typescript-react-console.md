# 3. Go for the API, TypeScript and React for the console

Status: Accepted, amended by [ADR-0011](0011-vite-plus-toolchain-for-typescript.md)
Date: 2026-10-24

## Context and Problem Statement

The platform has two programs: an API service that sits in front of TigerBeetle and Postgres,
and a web console that talks only to that API. We need to pick a language and a code layout
for each, and we need the pick to survive the fact that the owner is new to Go while the
codebase will be read by engineers who are new to it too. Which languages, libraries and
structure do we commit to, and what does the TigerBeetle client force on the build?

## Decision Drivers

- The official TigerBeetle clients cover .NET, Go, Java, Node.js, Python, Ruby and Rust. The
  API language must have an official client; there is no HTTP interface to the cluster.
- The API is small and I/O bound: validate a request, write to Postgres, submit a batch to
  TigerBeetle, answer. A large framework buys little here.
- The owner is new to Go. Idiomatic standard-library code with short explanatory comments is
  easier to learn from and to review than framework conventions.
- Layering must be enforced by tooling, not by convention, so a newcomer cannot accidentally
  call pgx from a use case.
- The console must stay a thin client of the OpenAPI contract (ADR-0008); it must never talk
  to Postgres or TigerBeetle.
- One monorepo, one package manager (pnpm 12.15.0), one task runner (Turborepo 2.11.10), so
  `pnpm lint` and `pnpm test` mean the same thing for Go and TypeScript.

## Considered Options

API language and style:

1. Go, standard library only for HTTP and logging, generated code for the contract.
2. Go with a web framework (chi, echo, gin) and a dependency injection library.
3. Node.js with the TigerBeetle Node client, sharing TypeScript with the console.
4. Rust with the TigerBeetle Rust client.

Console stack:

1. React 19 with Vite, a hand-written ky client over generated OpenAPI types, Biome.
2. A meta-framework (Next.js, Remix) with server-side rendering.
3. No console until the API is further along.

## Decision Outcome

Option 1 for both.

The API is Go 1.27 (`go 1.27.0` plus `toolchain go1.27.3` in `apps/api/go.mod`). Routing uses
`net/http.ServeMux` with the method-and-pattern syntax available since Go 1.22, so no router
dependency; `apps/api/internal/adapters/httpapi/router.go` wraps the mux to turn its plain-text
404 and 405 answers into RFC 9457 problems. Logging is `log/slog` with the JSON handler
(`apps/api/internal/platform/logging/logging.go`). Configuration comes from environment
variables through caarlos0/env. `apps/api/cmd/api/main.go` wires everything by hand: no
injection framework, no globals, no `init` side effects. A newcomer can read `main.go` top to
bottom and see every dependency being built and closed in reverse order.

The layout is ports and adapters:

- `internal/domain/ledger` holds pure types and typed errors (`ErrNotFound`,
  `ErrAlreadyExists`, `ErrInvalid`, `ErrUnavailable`). It imports nothing from `internal`.
- `internal/app` holds use cases and `ports.go`, the interfaces the use cases need
  (`LedgerStore` for TigerBeetle, `MetadataStore` for Postgres, `Clock`, `IDGenerator`). It
  imports only `domain`.
- `internal/adapters/{tigerbeetle,postgres,httpapi}` implement the ports or expose them over
  HTTP. Generated code lives in `gen` subpackages (oapi-codegen for HTTP, sqlc for SQL).
  Adapters import `app` and `domain`, never each other.
- `internal/platform/{config,logging,httpserver}` is process plumbing with no business logic.

These rules are enforced by golangci-lint's depguard in `apps/api/.golangci.yml`: `domain` may
not import pgx, the TigerBeetle client or `net/http`; `app` may not import `adapters` or
`platform`; each adapter is denied the other two. CI runs the linter, so a wrong import fails
the build rather than a code review. Errors cross layers as typed domain errors wrapped with
`%w`; `adapters/httpapi/problem.go` maps them to `application/problem+json` and driver error
text never reaches a client.

The TigerBeetle Go client is a cgo binding around a static C library that links libdl and
libm, and this shapes the container (`apps/api/Dockerfile`):

- `CGO_ENABLED=1` and a Debian builder (`golang:1.27.3-trixie`) with gcc and glibc. Alpine
  would build against musl and break.
- A glibc distroless runtime (`gcr.io/distroless/base-debian13:nonroot`), the same glibc 2.41
  as the builder. `scratch` lacks libc; `base-debian12` is deprecated and ships glibc 2.36, and
  a binary linked against 2.41 does not start on it.
- The client uses io_uring, which Docker 25+ blocks in its default seccomp profile. In
  `deploy/compose/compose.yaml` both the `tigerbeetle` and the `api` services therefore set
  `security_opt: seccomp=unconfined`; without it the API fails at startup with "io_uring is not
  available". A custom profile allowing only the io_uring syscalls is the production follow-up.
- The client version must not be newer than the replica. `scripts/check-versions.sh` compares
  `go.mod` with the compose image tag (0.17.9) and allows one documented exception, the
  client-only rebuild v0.17.10.

The console is React 19.3.1 on Vite 8.3.5 with TypeScript 7.0.2. HTTP goes through ky 2.2.0
in `packages/api-client/src/index.ts`, a small wrapper that adds `X-Request-Id`, retries
idempotent methods and converts problem+json answers into an `ApiError`. Types come from
openapi-typescript over `packages/openapi/openapi.yaml`, so the console cannot drift from the
contract without a type error. In development Vite proxies `/api` to the API
(`apps/console/vite.config.ts`), so the browser stays on one origin and the API needs no CORS
configuration. Biome 2.6.2 (`biome.json`) is the single formatter and linter for TypeScript:
one tool and one config file instead of ESLint plus Prettier plus their plugin matrix.

### Consequences

- Good: a newcomer learns Go itself, not a framework; the standard library is what every Go
  tutorial and the language documentation describe.
- Good: the dependency graph stays small, which keeps govulncheck noise low and makes Renovate
  bumps easy to review.
- Good: depguard makes the architecture self-checking; `docs/architecture.md` describes the
  rules and the linter enforces them.
- Good: use cases take interfaces, so `internal/app` tests run without Postgres or TigerBeetle;
  only adapter integration tests (testcontainers-go) need Docker.
- Good: the console is a plain static bundle; it can be served by any file server later.
- Bad: cgo rules out `CGO_ENABLED=0` static binaries, cross-compilation from macOS without a
  C toolchain, and Alpine images. Builds are slower and the image is larger than a scratch one.
- Bad: `seccomp=unconfined` on the API container is wider than needed until the custom
  profile exists.
- Bad: with no framework, cross-cutting concerns (request IDs, access logs, panic recovery) are
  hand-written middleware in `adapters/httpapi/middleware.go` that we must maintain.
- Bad: a hand-written ky client grows with every endpoint; if that becomes tedious we can
  switch to a generated fetch client, since the types already come from the spec.
- Bad: two languages mean two toolchains in CI (`.github/workflows/ci.yml` runs separate go,
  web and codegen-drift jobs) and two sets of version pins to keep aligned.

## More Information

- [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md): TigerBeetle is the only
  source of truth for money; Postgres holds context. The `LedgerStore` and `MetadataStore` ports
  mirror this split.
- [ADR-0008](0008-openapi-first-generated-server-and-client.md): the OpenAPI spec is the
  contract; the Go server interface and the TypeScript types are generated from it.
- [ADR-0009](0009-pnpm-turborepo-monorepo-with-go-module.md): the monorepo, the Turborepo shim
  and the tool pinning that run both toolchains from one root.
- Paths: `apps/api/cmd/api/main.go`, `apps/api/internal/app/ports.go`,
  `apps/api/.golangci.yml`, `apps/api/Dockerfile`, `deploy/compose/compose.yaml`,
  `scripts/check-versions.sh`, `packages/api-client/src/index.ts`,
  `apps/console/vite.config.ts`, `biome.json`, `turbo.json`.
- TigerBeetle docs: https://docs.tigerbeetle.com/coding/clients/go/ (Go client, cgo),
  https://docs.tigerbeetle.com/operating/deploying/docker/ (io_uring and seccomp),
  https://docs.tigerbeetle.com/operating/upgrading/ (clients must not be newer than replicas).
