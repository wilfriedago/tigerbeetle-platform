import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The console only talks to the platform API. In development Vite proxies /api to it, so the
// browser stays on one origin and the API needs no CORS configuration.
const apiProxyTarget = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: apiProxyTarget,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
