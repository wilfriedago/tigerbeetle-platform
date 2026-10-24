# @tigerbeetle-platform/openapi

`openapi.yaml` is the contract. Everything else is generated from it:
- Go server interfaces, handlers and models: `apps/api/internal/adapters/httpapi/gen` (oapi-codegen).
- TypeScript types: `packages/api-client/src/schema.d.ts` (openapi-typescript).

Workflow: edit the spec, run `pnpm generate` at the root, commit the generated files with the change.

`pnpm lint` runs Redocly with the `recommended-strict` ruleset.

This package pins TypeScript 5.9 because openapi-typescript 7 needs the TypeScript 5 compiler API,
which TypeScript 7 (the native compiler) no longer provides. The rest of the workspace uses TypeScript 7.
