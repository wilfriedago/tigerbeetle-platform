import { createRouter } from '@tanstack/react-router'

import { RouteError, RouteNotFound } from '~/components/layout/route-states'

import { createQueryClient } from '~/lib/query-client'

import { routeTree } from './route-tree.gen'

export function createAppRouter() {
  const queryClient = createQueryClient()
  return createRouter({
    context: { queryClient },
    defaultErrorComponent: RouteError,
    defaultNotFoundComponent: RouteNotFound,
    // Hovering a link starts its loaders; TanStack Query decides what is fresh, so the router keeps
    // nothing of its own.
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    defaultStructuralSharing: true,
    routeTree,
    scrollRestoration: true
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
