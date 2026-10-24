# tigerbeetle-platform

An open-source platform that makes [TigerBeetle](https://tigerbeetle.com) approachable, in the
spirit of what Formance built around its own ledger. A Go API exposes a Formance-style ledger
(ledgers as namespaces, hierarchical account addresses, multi-posting transactions, metadata,
holds) on top of TigerBeetle, Postgres keeps the metadata, and a web console sits on the API.
A versioned posting-rule DSL that compiles to batches of linked transfers comes later.

**Status: pre-alpha.** The repository holds the project setup and a thin vertical slice:
liveness, readiness and service-information endpoints, the generated server and client, and a
console page that shows the status of both dependencies. No ledger endpoints yet.

## Quickstart

Prerequisites: Docker with Compose v2, Node.js 24 and pnpm 12 (any pnpm 12 on your PATH works;
`packageManager` in package.json pins the exact version). Go and the Go tools are only needed to
work on the API itself (see [CONTRIBUTING.md](CONTRIBUTING.md)).

```sh
git clone https://github.com/wilfriedago/tigerbeetle-platform.git
cd tigerbeetle-platform
pnpm install
pnpm stack:up
curl -s localhost:8080/readyz
```

`pnpm stack:up` builds the API image, formats a TigerBeetle data file on first run, starts
TigerBeetle, Postgres, the API (which applies the migrations) and the console, and waits until
every service is healthy. The readiness call answers 200 with both dependencies reported `up`:

```json
{
  "status": "ready",
  "dependencies": {
    "postgres": { "status": "up", "latencyMs": 1 },
    "tigerbeetle": { "status": "up", "latencyMs": 1 }
  }
}
```

Then open http://localhost:5173 for the console, which polls `GET /v1/_info` through the API.
Stop everything with `pnpm stack:down`; `pnpm stack:reset` also deletes the data volumes.
Ports and credentials can be overridden by copying `.env.example` to `.env`.

## Architecture in short

- **TigerBeetle** is the only source of truth for money: accounts, transfers, balances.
- **Postgres** holds context only: ledger names, account addresses and metadata, idempotency
  records. It never stores a balance.
- **The API** (`apps/api`, Go, standard library HTTP) is the only writer to both. Its code is laid
  out as ports and adapters: `internal/domain` (pure rules), `internal/app` (use cases and ports),
  `internal/adapters/{tigerbeetle,postgres,httpapi}`, `internal/platform` (config, logging,
  server lifecycle). The layering is enforced by the linter.
- **The OpenAPI spec** (`packages/openapi/openapi.yaml`) is the contract. The Go server interfaces
  and the TypeScript client (`packages/api-client`) are generated from it.
- **The console** (`apps/console`, React and Vite) talks only to the API.

The full picture, including the write and read paths, is in
[docs/architecture.md](docs/architecture.md). Every design decision has an ADR in
[docs/adr](docs/adr).

## Repository layout

```
apps/api            Go API service (module github.com/wilfriedago/tigerbeetle-platform/apps/api)
apps/console        React console
packages/openapi    The OpenAPI contract and its linters and generators
packages/api-client Generated TypeScript types and the ky-based client
deploy/compose      Local stack
docs                Architecture overview and ADRs
scripts             Tool installer, version check, image smoke test
```

## Documentation

- [docs/architecture.md](docs/architecture.md)
- [docs/adr](docs/adr) (start with ADR-0001)
- [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md)
- [SECURITY.md](SECURITY.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## Licence

Apache-2.0, the same licence as TigerBeetle. See [LICENSE](LICENSE).
