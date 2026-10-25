import { lazy, Suspense } from 'react'

// The devtools are loaded only in development, so production bundles carry none of them.
const RouterDevtools = import.meta.env.DEV
  ? lazy(async () => import('@tanstack/react-router-devtools').then((m) => ({ default: m.TanStackRouterDevtools })))
  : null
const QueryDevtools = import.meta.env.DEV
  ? lazy(async () => import('@tanstack/react-query-devtools').then((m) => ({ default: m.ReactQueryDevtools })))
  : null

export function DevTools() {
  if (!RouterDevtools || !QueryDevtools) {
    return null
  }
  return (
    <Suspense fallback={null}>
      {/* Both toggles sit bottom right, clear of the sidebar footer and the top bar actions. */}
      <RouterDevtools position="bottom-right" toggleButtonProps={{ style: { marginRight: 56 } }} />
      <QueryDevtools buttonPosition="bottom-right" />
    </Suspense>
  )
}
