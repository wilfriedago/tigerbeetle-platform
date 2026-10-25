import { lazy, Suspense } from 'react'

import { TanStackDevtools } from '@tanstack/react-devtools'
import { FormDevtoolsPanel } from '@tanstack/react-form-devtools'
import { HotkeysDevtoolsPanel } from '@tanstack/react-hotkeys-devtools'
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { TanStackRouterDevtoolsPanel } from '@tanstack/react-router-devtools'

// The devtools are loaded only in development, so production bundles carry none of them.
const RouterDevtools =
  import.meta.env.DEV && lazy(async () => import('@tanstack/react-router-devtools').then((m) => ({ default: m.TanStackRouterDevtools })))
const QueryDevtools =
  import.meta.env.DEV && lazy(async () => import('@tanstack/react-query-devtools').then((m) => ({ default: m.ReactQueryDevtools })))

export function DevTools() {
  if (!RouterDevtools || !QueryDevtools) {
    return null
  }

  return (
    <Suspense fallback={null}>
      <TanStackDevtools
        config={{ position: 'middle-right' }}
        plugins={[
          { id: 'form', name: 'TanStack Form', render: <FormDevtoolsPanel /> },
          { id: 'query', name: 'TanStack Query', render: <ReactQueryDevtoolsPanel /> },
          { id: 'router', name: 'TanStack Router', render: <TanStackRouterDevtoolsPanel /> },
          { id: 'hotkeys', name: 'TanStack Hotkeys', render: <HotkeysDevtoolsPanel theme="dark" devtoolsOpen={false} /> }
        ]}
      />
    </Suspense>
  )
}
