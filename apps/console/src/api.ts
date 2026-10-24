import { createClient } from "@tigerbeetle-platform/api-client";

// "/api" is proxied to the API by the Vite dev server (see vite.config.ts). A production build
// expects the same arrangement: serve the console and the API behind one origin (a reverse proxy
// mapping /api to the API), or set VITE_API_URL to that path. The API sends no CORS headers.
export const api = createClient({
  baseUrl: import.meta.env.VITE_API_URL ?? "/api",
});
