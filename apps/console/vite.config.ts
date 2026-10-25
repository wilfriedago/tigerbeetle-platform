import { fileURLToPath } from 'node:url'

import { tanstackRouter } from '@tanstack/router-plugin/vite'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, lazyPlugins } from 'vite-plus'

// The console only talks to the platform API. In development Vite proxies /api to it, so the
// browser stays on one origin and the API needs no CORS configuration.
const apiProxyTarget = process.env.API_PROXY_TARGET ?? 'http://localhost:8080'

export default defineConfig({
  // lazyPlugins skips loading plugins when vp only reads lint/fmt metadata and is typed as possibly
  // undefined, which exactOptionalPropertyTypes rejects for `plugins`. The router plugin reads
  // tsr.config.json and must run before the React plugin so route files are split first.
  plugins: lazyPlugins(() => [tanstackRouter({ target: 'react' }), react(), tailwindcss()]) ?? [],
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('src', import.meta.url))
    }
  },
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
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}']
  }
})
