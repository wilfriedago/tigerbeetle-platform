# @tigerbeetle-platform/api-client

TypeScript client for the platform API (ADR-0019). orval generates it from
`packages/openapi/openapi.yaml` with `just generate`: one module per OpenAPI tag and the model
types under `src/gen`, committed and never edited. Every generated function sends its request
through the hand-written layer in `src/client`:

- `config.ts`: `configureClient()` sets the base URL, `fetch`, the timeout per attempt (15 s), the
  retries (2) and the request ID generator.
- `headers.ts`: an `X-Request-Id` on every call, the same across its retries.
- `fetcher.ts`: the orval mutator. GET, HEAD and OPTIONS retry after a network error or a 502,
  503 or 504, waiting for a 503's `Retry-After` up to 10 s; writes never retry. The caller's
  abort signal and the timeout are combined. `/readyz` resolves its 503 body.
- `errors.ts`: `ApiError` for problem+json answers (status, type, request ID, `Retry-After` in
  seconds), `HttpStatusError` for any other failed status.
- `query.ts`: `queryParams()` drops unset filter members before a list call.

```ts
import { configureClient, createTransaction, listLedgers, queryParams } from '@tigerbeetle-platform/api-client'

configureClient({ baseUrl: 'http://localhost:8080' })

const ledgers = await listLedgers(queryParams({ limit: 100, cursor: undefined }))
const transaction = await createTransaction(
  'eu-payments',
  { postings: [{ source: 'world', destination: 'users:1:wallet', asset: 'EUR/2', amount: '1250' }] },
  { 'Idempotency-Key': 'idem_1' }
)
```

The five money writes take the `Idempotency-Key` header as a required argument, so a call
without it does not compile. Reuse the key to retry a write safely (ADR-0007).
