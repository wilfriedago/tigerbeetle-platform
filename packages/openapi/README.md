# @tigerbeetle-platform/openapi

`openapi.yaml` is the REST contract of the platform API. It is generated from the Go code by
`apps/api/cmd/openapi` (ADR-0018) and committed; never edit it. The TypeScript client in
`packages/api-client` is generated from it with orval (ADR-0019).

Workflow: change the Huma registrations in `apps/api/internal/adapters/httpapi/v1`, run
`just generate` at the root, and commit the Go change with the regenerated spec and client.

`just lint-spec` at the root (or `pnpm lint` here) runs Redocly with the `recommended-strict`
ruleset. `just spec-diff` reports breaking changes against `main`, as CI does on pull requests.
