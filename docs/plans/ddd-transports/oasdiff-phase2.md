# oasdiff report: the hand-written spec against the spec generated from Go

Phase 2 of `plan.md` (section 7.5), 2026-10-25. Base: `packages/openapi/openapi.yaml` as written by
hand at commit 75f4581, the last commit before the switch. Revision: the file
`cmd/openapi` writes from the Huma registrations of `apps/api/internal/adapters/httpapi/v1`.
oasdiff 1.34.1, Redocly CLI 2.71.0.

## Commands and results

```sh
git show HEAD:packages/openapi/openapi.yaml > "$TMPDIR/base.yaml"
just generate-contracts

oasdiff breaking "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --flatten-params --fail-on WARN
# No breaking changes to report, but the specs are different.

oasdiff changelog "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --flatten-params
# No changes to report, but the specs are different.

oasdiff diff "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --flatten-params \
  --exclude-elements description,examples,title,summary -f yaml
# components.parameters.deleted: AccountAddress, Cursor, IdempotencyKey, LedgerName, Limit,
# ResourceId

oasdiff diff "$TMPDIR/base.yaml" packages/openapi/openapi.yaml --flatten-params -f yaml
# the same, plus info.description

just lint-spec
# Woohoo! Your API description is valid.
```

`--flatten-params` is required. The hand-written spec declares `ledger`, `address`, `id`, `key`
and `pattern` once per path; Huma declares every parameter on each operation. Without the flag,
`oasdiff breaking` reports 42 `new-request-path-parameter` errors, one per operation and path
parameter, for parameters that exist on both sides. With it, the two specs describe the same
operations. The CI gate therefore runs with the flag.

The plan's procedure passed `--exclude-elements` to `oasdiff changelog`, which only `oasdiff
diff` accepts; the documentation-free comparison is the third command above.

## Every difference, explained

| #   | Difference                                                                                                                                                              | Explanation                                                                                                                                                                            |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | The file is reordered: paths and components in alphabetical order, keys sorted.                                                                                         | Huma writes the document from Go maps. Not a change; oasdiff ignores order.                                                                                                            |
| D2  | `components.parameters` is gone: `AccountAddress`, `Cursor`, `IdempotencyKey`, `LedgerName`, `Limit`, `ResourceId`.                                                     | Huma writes parameters inline on each operation. Every operation still declares the same parameters with the same schema, description and required flag, as the empty changelog shows. |
| D6  | `info.description` now says the file is generated from the Go code by `cmd/openapi` and must not be edited, instead of "Edit the spec first, then run `pnpm generate`". | Intended (ADR-0018).                                                                                                                                                                   |

Everything else is identical: every path, operation ID, tag, parameter, request body,
response status, schema (named or inline, including the `allOf` of `Account`, `PostingRecord`
and `WriteRequestDetail`, `propertyNames` on `Metadata`, `const: ok` on `Liveness`), example,
description, the `Problem` response and its `application/problem+json` media type,
`security: []`, `servers`, `license` and `info.summary`. None of the fallbacks of plan section
7.4 was needed.

## Behaviour oasdiff cannot see

The end-to-end integration test (`cmd/api/api_integration_test.go`) runs unchanged against the
Huma server and passes: ledgers, rules, event types, multi-posting and multi-asset transactions,
replays and key reuse, declines with per-posting results, reverts, holds, aggregates and both
crash windows of ADR-0007. The table below lists what differs on the wire.

| Behaviour                                                                 | Before (oapi-codegen)                                                     | After (Huma)                                                                                                            |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Status and problem type of every answer                                   | as specified                                                              | unchanged: validation stays in the domain, Huma's own input errors map to `400 invalid-request`                         |
| `detail` of a value that cannot be decoded (`limit=abc`, malformed JSON)  | oapi-codegen's message, such as `Invalid format for parameter limit: ...` | Huma's, such as `query.limit: invalid integer` and `body: unexpected end of JSON input`                                 |
| `detail` of an empty required body                                        | `can't decode JSON body: EOF`                                             | `request body is required`                                                                                              |
| A money write with no Idempotency-Key and an empty or malformed body      | `missing-idempotency-key`: the header was bound first                     | `invalid-request`: Huma reads the body before the handler checks the header. With a valid body, unchanged.              |
| A request body larger than 1 MiB                                          | decoded, then refused by the domain (`400 invalid-request`)               | `400 invalid-request` with `request body is too large limit=1048576 bytes` (Huma's default). No valid body comes close. |
| Time to read a request body                                               | the server's 30 s `ReadTimeout`                                           | also at most 5 s once the handler starts (Huma's default)                                                               |
| `<`, `>` and `&` inside JSON strings                                      | escaped as `<`, `>`, `&`                                                  | written as is; the same JSON value                                                                                      |
| `holds.reserved` of `GET .../stats`                                       | in map order, which varied between calls                                  | sorted by asset                                                                                                         |
| Body sent without a JSON `Content-Type` (`curl -d`)                       | decoded as JSON                                                           | unchanged, through a small middleware (plan question 6)                                                                 |
| Unknown members in a body, values beyond the documented limits            | ignored, clamped by the domain                                            | unchanged (plan question 4)                                                                                             |
| 404, 405, panics, `X-Request-Id`, `Retry-After`, `X-Content-Type-Options` | problem+json, headers as specified                                        | unchanged                                                                                                               |

## After the parity commit: the headers the API already sent

The next commit declares two headers the API has always sent without documenting them (plan
question 3): `X-Request-Id` as an optional request header and a response header of every answer,
and `Retry-After` on the shared Problem response. Against the hand-written spec, `oasdiff breaking`
still reports nothing; the changelog lists 141 INFO entries: 35 `new-optional-request-parameter`
and 106 `response-header-added`. The generated client can now read `Retry-After` from its types.
