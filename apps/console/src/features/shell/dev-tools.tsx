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
      <RouterDevtools position="bottom-left" />
      <QueryDevtools buttonPosition="bottom-left" />
    </Suspense>
  )
}
