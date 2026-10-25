# console

React, TypeScript and Vite. The console talks only to the platform API through
`@tigerbeetle-platform/api-client`.

- `pnpm --filter console dev` starts Vite on port 5173 and proxies `/api` to `API_PROXY_TARGET`
  (default `http://localhost:8080`).
- A production build expects the API under the same origin (a reverse proxy mapping `/api` to
  the API); `VITE_API_URL` overrides the path. The API sends no CORS headers.
