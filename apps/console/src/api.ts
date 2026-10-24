import { createClient } from "@tigerbeetle-platform/api-client";

// "/api" is proxied to the API by the Vite dev server (see vite.config.ts). Set VITE_API_URL to
// point a production build somewhere else.
export const api = createClient({
  baseUrl: import.meta.env.VITE_API_URL ?? "/api",
});
