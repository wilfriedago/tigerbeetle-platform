# tigerbeetle-platform

An open-source platform that makes [TigerBeetle](https://tigerbeetle.com) easier to work with.
TigerBeetle is fast and strict because it is narrow: 128-bit IDs, fixed-shape accounts and
transfers, no string keys and no metadata. Every team that adopts it ends up building the same
layer around it. This project is that layer, with TigerBeetle still in charge of every balance.
A Go API puts named ledgers, hierarchical account addresses, multi-posting transactions,
metadata and holds on top of TigerBeetle's accounts, linked transfers and two-phase transfers,
Postgres keeps that context, and a web console sits on the API. A versioned posting-rule DSL
that compiles to batches of linked transfers comes later.

**Status: pre-alpha.** The API serves the ledger surface: ledgers, accounts with lazy creation
and address rules, multi-posting transactions with idempotency and a reconciler, holds,
reverts, prefix balances and the write-request log. The console covers every page of
[docs/console.md](docs/console.md). No authentication yet.

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

Then open http://localhost:5173 for the console. `scripts/seed-demo.sh` creates a demo ledger
(`eu-payments`) with accounts, transactions, a declined write and holds, through the API.
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
- **The console** (`apps/console`, React, Vite and TanStack, ADR-0014) talks only to the API.

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
scripts             Tool installer, version check, image smoke test, demo seed
```

## Documentation

- [docs/architecture.md](docs/architecture.md)
- [docs/adr](docs/adr) (start with ADR-0001)
- [docs/handoff.md](docs/handoff.md): state of the work and what to do next
- [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md)
- [SECURITY.md](SECURITY.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## Licence

Apache-2.0, the same licence as TigerBeetle. See [LICENSE](LICENSE).
