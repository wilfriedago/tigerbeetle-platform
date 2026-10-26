import { useEffect } from 'react'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { HealthDot } from '~/components/ledger/health'

import { useOrganization } from '~/lib/organization'

import { systemQueries } from './api'
import { recordProbe } from './probe-history'

/** "All systems up" in the top bar, fed by polling GET /v1/_info (docs/console.md, Top bar). */
export function HealthIndicator() {
  const org = useOrganization()
  const { data, isError, isPending, dataUpdatedAt, errorUpdatedAt } = useQuery(systemQueries.info())
  // Every answer, or failure, is a probe for the System health chart.
  const probedAt = Math.max(dataUpdatedAt, errorUpdatedAt)
  useEffect(() => {
    if (probedAt > 0) {
      recordProbe(probedAt, isError ? undefined : data)
    }
  }, [probedAt, isError, data])
  let up: boolean | undefined
  let label = 'Checking'
  if (isError) {
    up = false
    label = 'API unreachable'
  } else if (data) {
    const down = [
      data.dependencies.postgres.status === 'down' && 'Postgres',
      data.dependencies.tigerbeetle.status === 'down' && 'TigerBeetle'
    ].filter(Boolean)
    up = down.length === 0
    label = up ? 'All systems up' : `${down.join(' and ')} down`
  }
  return (
    <Link
      to="/$org/system"
      params={{ org }}
      className="hidden rounded-sm px-1 outline-none hover:opacity-80 focus-visible:ring-3 focus-visible:ring-focus-ring sm:block"
      aria-busy={isPending}
    >
      <HealthDot up={up} label={label} />
    </Link>
  )
}

/** "api 0.1.0  tigerbeetle 0.17.9" under the sidebar footer. */
export function VersionLine() {
  const { data } = useQuery(systemQueries.info())
  if (!data) {
    return null
  }
  return (
    <>
      <span>api {data.version}</span>
      {data.runtime ? <span>tigerbeetle {data.runtime.tigerbeetleClientVersion.replace(/^v/, '')}</span> : null}
    </>
  )
}
