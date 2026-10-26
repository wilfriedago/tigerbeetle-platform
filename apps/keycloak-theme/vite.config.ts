import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { keycloakify } from 'keycloakify/vite-plugin'
import { defineConfig, lazyPlugins } from 'vite-plus'

// The login theme Keycloak renders: sign-in, account setup, invitations (.design/auth.pen, the
// "Keycloak:" frames). `keycloakify build` packages it as a jar for Keycloak 26; the account
// console stays Keycloak's own.
export default defineConfig({
  plugins:
    lazyPlugins(() => [
      react(),
      tailwindcss(),
      keycloakify({
        accountThemeImplementation: 'none',
        keycloakVersionTargets: { '22-to-25': false, 'all-other-versions': 'tbp-keycloak-theme.jar' },
        themeName: 'tbp'
      })
    ]) ?? [],
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('src', import.meta.url))
    }
  },
  server: {
    port: 5174,
    strictPort: true
  }
})
