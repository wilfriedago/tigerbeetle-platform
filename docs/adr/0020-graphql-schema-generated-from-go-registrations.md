# 20. GraphQL schema generated from Go registrations; gqlgen executor

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

The console moves from REST to GraphQL (ADR-0023), and the GraphQL schema must be an output of
the Go code, like the REST spec (ADR-0018), never written by hand. No mature Go library does both
halves: gqlgen is schema-first (it reads SDL and generates typed resolver interfaces), and
graphql-go builds a schema in Go but hands resolvers untyped `map[string]any` arguments. We want
the schema derived from Go types and resolvers that the compiler checks.

## Decision Drivers

- The schema in `packages/graphql/schema.graphql` is generated, committed, never edited, and
  checked for drift in CI; graphql-codegen in the console reads it.
- Each module registers its own queries, mutations and fields with typed Go argument and result
  structs (ADR-0017).
- A registration that disagrees with its resolver must fail to compile.
- The schema must still regenerate when the gqlgen output is stale.
- Result types are shared with REST where the shapes match, so the two APIs cannot drift apart.
- Money and 128-bit IDs are strings on the wire; no number a JavaScript client could round.
- The schema is designed for GraphQL: nested fields where the console needs them, cursor
  connections for lists.
- The generator stays small and readable for a Go newcomer.

## Considered Options

1. A small generator in the repository that renders SDL from registrations by reflection, with
   gqlgen generating the executor from that SDL (chosen).
2. Hand-written SDL with gqlgen.
3. graphql-go/graphql, schema built in Go.

Option 2 is the hand-written contract the owner ruled out. Option 3 loses typed arguments and
the compiler check, and its schema exists only at run time.

## Decision Outcome

Chosen option: 1. The generator lives in `internal/platform/graphqlschema`, uses only `reflect`,
`sort` and `strings`, and stays under about 600 lines plus tests.

Registration. `graphqlschema.New()` returns a `*Schema` with no package-level state. Generic
functions take it as their first argument, because Go methods cannot have their own type
parameters: `Query[Args, Result](s, name, doc)`, `Mutation[Args, Result](s, name, doc)`,
`Field[Args, Result](s, parent, name, doc)` for a resolver field on an object type,
`Scalar[T](s, name, doc)` and `Describe[T](s, doc)`. `graphqlschema.NoArgs` marks a field
without arguments. `Field` names its parent type by its GraphQL name, so bookkeeping can add
`Ledger.accounts` to a type the ledgers module owns without importing it; `SDL()` fails when the
parent does not exist. `SDL()` checks every registration, returns all problems together with
`errors.Join`, each naming the Go type and field, and renders a deterministic document.

Reflection rules:

| Go                                                      | GraphQL                                                      |
| ------------------------------------------------------- | ------------------------------------------------------------ |
| struct as a result / inside arguments                   | `type` / `input`; one struct used as both is an error        |
| field name                                              | the `json` tag; `json:"-"` and unexported fields are skipped |
| embedded struct                                         | fields flattened, as `encoding/json` does                    |
| `doc`, `default`, `deprecated` tags                     | description, default value, `@deprecated(reason: ...)`       |
| non-pointer / pointer                                   | non-null `T!` / nullable `T`                                 |
| slice / pointer to slice / slice of pointers            | `[T!]!` / `[T!]` / `[T]!`                                    |
| named string type with `EnumValues() []string`          | `enum`                                                       |
| `string`, `bool`; `int`, `int32` and smaller            | `String`, `Boolean`; `Int`                                   |
| `int64`, `uint32`; `time.Time`                          | `Int64` (counts only); `DateTime`                            |
| `uint64`, floats, maps, interfaces, channels, functions | error: money and IDs must use a named scalar                 |

Scalars, defined in `internal/platform/scalar` and shared with REST, where the same types
provide the OpenAPI schemas (ADR-0018): `Amount` (unsigned 128-bit, decimal string),
`SignedAmount` (balances), `U128` (32 lowercase hexadecimal characters, big-endian), `U64`
(TigerBeetle timestamps as decimal strings), `Metadata` (an object of strings), `JSON` (the stored
request and response of a write), `DateTime` (RFC 3339) and `Int64` (a JSON number, for counts
and TigerBeetle ledger numbers only). Each has `MarshalGQL` and `UnmarshalGQL` methods, plain
signatures that need no gqlgen import. `UnmarshalGQL` for the 128-bit and 64-bit scalars accepts
strings only, so no client loses precision through a float.

Shape. A query for one object by key returns a nullable type and `null` when it does not exist
(`ledger(name: String!): Ledger`); mutations return non-null types. Lists are connections:
`<Item>Connection { nodes: [<Item>!]!, pageInfo: PageInfo! }`, with
`PageInfo { endCursor: String, hasNextPage: Boolean! }` and the arguments `first` (default 25, at
most 100) and `after`. `endCursor` is the opaque page cursor REST returns as `next` (ADR-0004), so
a cursor works on either transport and the use cases do not change; per-item `edges` can be
added later without breaking anything. The console's overview becomes one request: a ledger with
its stats, assets and latest transactions.

Keeping registrations free of gqlgen. Registrations live in `modules/<m>/interfaces/graphql` and
are collected by `internal/transport/graphql/schema`; `cmd/graphqlschema` imports only those and
the generator. A depguard rule denies gqlgen and its generated executor in all of them, so when a
registration changes and the executor is stale, `just generate` still builds
`cmd/graphqlschema`, writes the new schema, then regenerates the executor.

gqlgen v0.17.98, run as `go tool gqlgen`, reads `packages/graphql/schema.graphql` through
`apps/api/gqlgen.yml` and writes the executor to `internal/transport/graphql/exec`. `autobind`
binds each schema type to the Go type of the same name in the scalar, `dto` and module `graphql`
packages, so `Transaction` is the very struct REST returns. There is no `resolver:` block:
gqlgen generates the resolver interfaces only, and regeneration never rewrites hand-written
code. Each module writes its resolver methods on its own types; `internal/transport/graphql`
builds the one root gqlgen expects by embedding them. Go note: embedding promotes the embedded
type's methods; a method two modules both define, or one nobody defines, is a compile error.
Fields one module adds to another module's type are delegated in the transport package with one
line each, so bookkeeping never imports ledgers.

Tests. Golden files in `internal/platform/graphqlschema/testdata` cover every reflection rule,
nullability case, connections, enums, scalars, embedding, cross-module fields, descriptions with
quotes and newlines, and determinism; error cases compare the joined error text with go-cmp.
gqlparser, which gqlgen already depends on, validates each golden file. `UPDATE_GOLDEN=1`
rewrites them; an environment variable keeps package-level `flag` variables out of the test.

Serving. `POST /graphql` on the REST mux and middleware, behind `GRAPHQL_ENABLED` (default true),
with the error mapping of ADR-0021, a recover function, complexity and depth limits,
introspection on (the schema is public in the repository) and the playground off (it loads its
code from a CDN). The console reaches it as `/api/graphql` through the existing proxy.

Spike S6 of the plan checks two gqlgen behaviours before the code: that a schema file outside
`apps/api` ends up inside the generated executor (the API image builds from `apps/api` only), and
that `autobind` resolves promoted fields of embedded structs. The fallbacks are a schema copy
written next to the executor, and listing fields instead of embedding.

GraphQL filters reuse the REST filters, which the `sqlc.narg(x) IS NULL OR ...` queries already
express, so no query builder and no query-builder ADR are needed.

### Consequences

- Good: one Go declaration per field feeds the schema, the console's types and the compiler
  check of the resolvers.
- Good: REST and GraphQL share result types and scalars, so a change to one shows in the other.
- Good: the schema regenerates even when the resolvers are out of date.
- Bad: a generator to maintain, with reflection code a newcomer must read carefully; the golden
  tests are its documentation.
- Bad: two generation steps for GraphQL (SDL, then gqlgen), in a fixed order.
- Bad: a list of ledgers with nested stats costs one stats call per ledger; no data loader until
  a page needs one.

## More Information

- [ADR-0017](0017-bounded-context-modules-and-dependency-rules.md): modules and the transport
  composition package.
- [ADR-0018](0018-rest-spec-generated-from-go-with-huma.md): the scalar types shared with REST.
- [ADR-0004](0004-ledger-api-surface.md): page cursors and amounts as strings.
- `docs/plans/ddd-transports/plan.md`, section 8, with a sample of the generated schema.
- gqlgen: https://gqlgen.com/ and https://github.com/99designs/gqlgen/releases/tag/v0.17.98
- GraphQL connections pattern: https://graphql.org/learn/pagination/
