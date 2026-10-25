import { Fragment } from 'react'

import { Link, useMatches } from '@tanstack/react-router'

import { Breadcrumb, BreadcrumbItem, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '~/components/ui/breadcrumb'

import { cn } from '~/lib/utils'

/**
 * A route opts into the breadcrumb with `staticData.crumb`: a label, or a function of the route
 * params for dynamic segments such as the ledger name or a shortened ID. `mono` marks machine
 * values (13 Navigation, Breadcrumb).
 */
export interface Crumb {
  label: string | ((params: Record<string, string>) => string)
  mono?: boolean
}

declare module '@tanstack/react-router' {
  interface StaticDataRouteOption {
    crumb?: Crumb
  }
}

export function Breadcrumbs({ className }: { className?: string }) {
  const matches = useMatches()
  const crumbs = matches.flatMap((match) => {
    const { crumb } = match.staticData
    if (!crumb) {
      return []
    }
    const params = match.params as Record<string, string>
    const label = typeof crumb.label === 'function' ? crumb.label(params) : crumb.label
    return [{ id: match.id, label, mono: crumb.mono ?? false, pathname: match.pathname }]
  })
  if (crumbs.length === 0) {
    return null
  }
  return (
    <Breadcrumb className={cn('min-w-0', className)}>
      <BreadcrumbList className="flex-nowrap">
        {crumbs.map((crumb, index) => {
          const last = index === crumbs.length - 1
          return (
            <Fragment key={crumb.id}>
              {index > 0 ? <BreadcrumbSeparator /> : null}
              <BreadcrumbItem className={cn('min-w-0', crumb.mono && 'font-mono')}>
                {last ? (
                  <BreadcrumbPage className="truncate">{crumb.label}</BreadcrumbPage>
                ) : (
                  <Link
                    to={crumb.pathname}
                    className="truncate rounded-xs transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-focus-ring"
                  >
                    {crumb.label}
                  </Link>
                )}
              </BreadcrumbItem>
            </Fragment>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
