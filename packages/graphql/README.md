# @tigerbeetle-platform/graphql

`schema.graphql` is the GraphQL schema of the platform API. It is generated from the Go
registrations of the modules by `apps/api/cmd/graphqlschema` (ADR-0020) and committed; never edit
it. gqlgen generates the server executor from it, and the console generates its typed documents
from it (ADR-0023).

Workflow: change a module's `interfaces/graphql` registrations, run `just generate` at the root,
and commit the Go change with the regenerated schema, executor and console types.

GraphQL is served at `POST /graphql` next to REST (`GRAPHQL_ENABLED`, default true). It is the
console's API; REST stays the stable API for third parties.
