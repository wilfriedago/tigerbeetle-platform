# console

The web console of the platform: React 19, TanStack Router, Query, Form, Table and Pacer, and the
Ledger Instrument design system on shadcn components (ADR-0014). It talks only to the platform
API through `@tigerbeetle-platform/api-client`.

- `pnpm --filter console dev` starts Vite on port 5173 and proxies `/api` to `API_PROXY_TARGET`
  (default `http://localhost:8080`).
- `pnpm --filter console generate` regenerates `src/route-tree.gen.ts` from `src/routes`; the
  Vite plugin does the same while `dev` runs. The file is committed.
- `pnpm --filter console test` runs the unit tests (money, preflight, validation, drafts).
- From the repository root, `just dev`, `just generate`, `just typecheck` and `just test-ts` run
  these tasks for every package (ADR-0016).
- A production build expects the API under the same origin (a reverse proxy mapping `/api` to
  the API); `VITE_API_URL` overrides the path, `VITE_API_PUBLIC_URL` sets the address shown in
  "Copy as curl". The API sends no CORS headers.
- `scripts/seed-demo.sh` (repository root) fills a demo ledger through the API.

Where things are: a page is a route file in `src/routes` (search schema, loader, breadcrumb) and a
folder in `src/features`. Shared pieces live in `src/components` (`ui` for the restyled shadcn
components, `ledger` for amounts, addresses, chains and balances) and `src/lib`.
