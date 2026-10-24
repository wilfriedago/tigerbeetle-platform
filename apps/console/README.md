# console

React, TypeScript and Vite. The console talks only to the platform API through
`@tigerbeetle-platform/api-client`.
- `pnpm --filter console dev` starts Vite on port 5173 and proxies `/api` to `API_PROXY_TARGET`
  (default `http://localhost:8080`).
- `VITE_API_URL` points a production build at an API origin directly.
