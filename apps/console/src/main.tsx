import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import '@fontsource/ibm-plex-mono/latin-400.css'
import '@fontsource/ibm-plex-mono/latin-500.css'
import './styles/globals.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { RouterProvider } from '@tanstack/react-router'

import { configureApi } from '~/lib/api'
import { applyPreferences, getPreferences, watchSystemTheme } from '~/lib/preferences'

import { createAppRouter } from './router'

configureApi()
const router = createAppRouter()

applyPreferences(getPreferences())
watchSystemTheme()

const root = document.getElementById('root')
if (!root) {
  throw new Error('missing #root element')
}
createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>
)
