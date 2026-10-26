import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import '@fontsource/ibm-plex-mono/latin-400.css'
import '@fontsource/ibm-plex-mono/latin-500.css'
import './styles.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { type KcContext, KcPage } from './kc.gen'

/** A Keycloak template name, such as login-username.ftl; Keycloakify's mock knows every one. */
function isPageId(value: string): value is KcContext['pageId'] {
  return value.endsWith('.ftl')
}

/** Keycloak's page sets kcContext on the global object before the script runs. */
const page = globalThis as typeof globalThis & { kcContext?: KcContext }

// Keycloak serves the pages with page.kcContext set. `vp dev` has none, so it mocks the page the
// query names, to work on the theme without Keycloak.
if (import.meta.env.DEV && !page.kcContext) {
  const { getKcContextMock } = await import('./login/mock')
  const requested = new URLSearchParams(location.search).get('page') ?? ''
  page.kcContext = getKcContextMock({ pageId: isPageId(requested) ? requested : 'login-username.ftl' })
}

// The pages follow the system's light or dark mode, before the first paint.
if (globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches) {
  document.documentElement.classList.add('dark')
}

const root = document.querySelector('#root')
if (root) {
  createRoot(root).render(<StrictMode>{page.kcContext ? <KcPage kcContext={page.kcContext} /> : <h1>No Keycloak context</h1>}</StrictMode>)
}
