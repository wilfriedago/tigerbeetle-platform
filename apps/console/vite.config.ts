import react from '@vitejs/plugin-react'
import { defineConfig, lazyPlugins } from 'vite-plus'

// The console only talks to the platform API. In development Vite proxies /api to it, so the
// Browser stays on one origin and the API needs no CORS configuration.
const apiProxyTarget = process.env.API_PROXY_TARGET ?? 'http://localhost:8080'

export default defineConfig({
  // LazyPlugins skips loading plugins when vp only reads lint/fmt metadata and is typed as possibly
  // Undefined, which exactOptionalPropertyTypes rejects for `plugins`.
  plugins: lazyPlugins(() => [react()]) ?? [],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
        target: apiProxyTarget
      }
    },
    strictPort: true
  }
})
