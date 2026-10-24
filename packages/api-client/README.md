# @tigerbeetle-platform/api-client

Typed client for the platform API. `src/schema.d.ts` is generated from `packages/openapi/openapi.yaml`
by `pnpm generate`; do not edit it. `src/index.ts` wraps ky with a request ID header, retries for
idempotent calls and RFC 9457 problem handling.

```ts
import { createClient } from "@tigerbeetle-platform/api-client";

const api = createClient({ baseUrl: "http://localhost:8080" });
const info = await api.getInfo();
```
