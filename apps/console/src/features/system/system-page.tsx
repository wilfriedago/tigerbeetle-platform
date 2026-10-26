import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { CircleCheckIcon, CircleXIcon, DatabaseIcon, RefreshCwIcon, ServerIcon, ZapIcon } from 'lucide-react'

import { Page, PageHeader } from '~/components/layout/page'
import { DescriptionItem, DescriptionList } from '~/components/ledger/description-list'
import { DependencyRow } from '~/components/ledger/health'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import { Button } from '~/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip'

import { useOrganization } from '~/lib/organization'
import { formatRelative, formatTime } from '~/lib/time'
import { useNow } from '~/lib/use-now'
import { cn } from '~/lib/utils'

import { HEALTH_INTERVAL_MS, systemQueries } from './api'
import { PROBE_HISTORY_SIZE, type ProbeSample, useProbeHistory } from './probe-history'

// docs/console.md, System health: the API and its two stores from GET /v1/_info, refreshed every
// few seconds, the versions in use, and the reconciler that settles interrupted writes.

function uptime(startedAt: string, now: Date): string {
  const seconds = Math.max(0, Math.floor((now.getTime() - Date.parse(startedAt)) / 1000))
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) {
    return `${days} d ${hours} h`
  }
  return hours > 0 ? `${hours} h ${minutes} min` : `${minutes} min`
}

/** One bar per probe; a probe well above the usual latency is drawn in the warning tone. */
function LatencyBars({ samples }: { samples: readonly ProbeSample[] }) {
  const values = samples.map((sample) => sample.tigerbeetleMs ?? 0)
  const sorted = values.toSorted((a, b) => a - b)
  const median = sorted[Math.floor(sorted.length / 2)] ?? 0
  const max = Math.max(1, ...values)
  const slow = (ms: number) => ms >= 5 && ms > median * 3
  const peakIndex = values.indexOf(Math.max(...values))
  const peak = samples[peakIndex]
  return (
    <div className="flex flex-col gap-2">
      <div className="flex h-28 items-end gap-0.5" role="img" aria-label={`TigerBeetle probe latency, ${samples.length} probes, peak ${max} ms`}>
        {samples.map((sample, index) => {
          const ms = sample.tigerbeetleMs
          return (
            <Tooltip key={sample.at}>
              <TooltipTrigger
                render={
                  <div
                    className={cn(
                      'min-w-1 flex-1 rounded-t-[2px]',
                      ms === undefined ? 'h-full bg-danger/40' : slow(ms) ? 'bg-warning' : 'bg-bar-available',
                      index === samples.length - 1 && 'opacity-90'
                    )}
                    style={ms === undefined ? undefined : { height: `${Math.max(4, (ms / max) * 100)}%` }}
                  />
                }
              />
              <TooltipContent>
                <span className="font-mono font-medium">{ms === undefined ? 'down' : `${ms} ms`}</span>
                <span className="opacity-75">{formatTime(new Date(sample.at))}</span>
              </TooltipContent>
            </Tooltip>
          )
        })}
      </div>
      <div className="flex justify-between font-mono text-[11px] text-subtle-foreground">
        <span>{samples[0] ? formatTime(new Date(samples[0].at)) : ''}</span>
        {peak?.tigerbeetleMs !== undefined && slow(peak.tigerbeetleMs) ? (
          <span className="text-warning-fg">
            peak {peak.tigerbeetleMs} ms at {formatTime(new Date(peak.at))}
          </span>
        ) : null}
        <span>now</span>
      </div>
    </div>
  )
}

export function SystemPage() {
  const org = useOrganization()
  const info = useQuery(systemQueries.info())
  const reconciler = useQuery(systemQueries.reconciler())
  const history = useProbeHistory()
  const now = useNow(1000)
  const { data } = info
  const postgres = data?.dependencies.postgres
  const tigerbeetle = data?.dependencies.tigerbeetle
  const down = [postgres?.status === 'down' && 'Postgres', tigerbeetle?.status === 'down' && 'TigerBeetle'].filter(Boolean)
  const checkedAt = info.dataUpdatedAt > 0 ? new Date(info.dataUpdatedAt) : undefined
  const runtime = data?.runtime

  return (
    <Page>
      <PageHeader
        title="System health"
        description={`The API and its two stores, from GET /v1/_info every ${HEALTH_INTERVAL_MS / 1000} seconds.`}
        actions={
          <Button variant="secondary" loading={info.isFetching} onClick={() => void info.refetch()}>
            <RefreshCwIcon />
            Refresh
          </Button>
        }
      />
      {info.isError ? (
        <ProblemPanel error={info.error} title="The API does not answer" onRetry={() => void info.refetch()} />
      ) : data ? (
        <Alert variant={down.length === 0 ? 'success' : 'danger'}>
          {down.length === 0 ? <CircleCheckIcon /> : <CircleXIcon />}
          <AlertTitle>{down.length === 0 ? 'All systems up' : `${down.join(' and ')} down`}</AlertTitle>
          <AlertDescription>
            {checkedAt ? `Last check ${formatTime(checkedAt)}, ${formatRelative(checkedAt, now)}. ` : ''}
            {down.length === 0
              ? 'Writes and reads are served.'
              : 'Writes that need the store answer 503 until it is back; pending writes are settled by the reconciler.'}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="flex flex-col gap-3">
          <DependencyRow
            icon={ServerIcon}
            name="API"
            up={!info.isError}
            detail={data ? `${data.service} ${data.version}${runtime ? `, up ${uptime(runtime.startedAt, now)}` : ''}` : 'checking'}
          />
          <DependencyRow
            icon={DatabaseIcon}
            name="Postgres"
            up={postgres?.status === 'up'}
            {...(postgres?.latencyMs === undefined ? {} : { latencyMs: postgres.latencyMs })}
            {...(postgres?.error ? { reason: postgres.error } : {})}
            detail={
              runtime
                ? `${runtime.postgresVersion?.split(' ')[0] ?? 'version unknown'}, schema version ${runtime.schemaVersion ?? 'unknown'}`
                : undefined
            }
          />
          <DependencyRow
            icon={ZapIcon}
            name="TigerBeetle"
            up={tigerbeetle?.status === 'up'}
            {...(tigerbeetle?.latencyMs === undefined ? {} : { latencyMs: tigerbeetle.latencyMs })}
            {...(tigerbeetle?.error ? { reason: tigerbeetle.error } : {})}
            detail={runtime ? `cluster ${runtime.tigerbeetleClusterId}, client ${runtime.tigerbeetleClientVersion}` : undefined}
          />
          <Card>
            <CardHeader>
              <CardTitle>Probe latency, last {PROBE_HISTORY_SIZE} checks</CardTitle>
              <CardDescription>TigerBeetle round trip, recorded while the console is open.</CardDescription>
            </CardHeader>
            <CardContent>
              {history.length > 1 ? <LatencyBars samples={history} /> : <p className="text-small text-muted-foreground">Collecting probes.</p>}
            </CardContent>
          </Card>
        </div>
        <div className="flex flex-col gap-6">
          <Card>
            <CardContent className="py-1">
              <DescriptionList>
                <DescriptionItem term="API" mono termWidth="sm">
                  {data?.version ?? '-'}
                </DescriptionItem>
                <DescriptionItem term="TigerBeetle client" mono termWidth="sm">
                  {runtime?.tigerbeetleClientVersion.replace(/^v/, '') ?? '-'}
                </DescriptionItem>
                <DescriptionItem term="Postgres" mono termWidth="sm">
                  {runtime?.postgresVersion?.split(' ')[0] ?? '-'}
                </DescriptionItem>
                <DescriptionItem term="Schema" mono termWidth="sm">
                  {runtime?.schemaVersion ?? '-'}
                </DescriptionItem>
                <DescriptionItem term="Go" mono termWidth="sm">
                  {runtime?.goVersion.replace(/^go/, '') ?? '-'}
                </DescriptionItem>
              </DescriptionList>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Reconciler</CardTitle>
              <CardDescription>Settles pending write records older than {reconciler.data?.gracePeriodSeconds ?? 30} s.</CardDescription>
              <CardAction>
                <Button variant="link" size="sm" render={<Link to="/$org/system/reconciler" params={{ org }} />}>
                  Open
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent className="grid grid-cols-3 gap-3">
              <div className="flex flex-col gap-1">
                <span className="text-caption text-muted-foreground">Pending</span>
                <span className="font-mono text-card-title">{reconciler.data?.pending ?? '-'}</span>
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-caption text-muted-foreground">Last run</span>
                <span className="font-mono text-card-title">{reconciler.data?.lastRunAt ? formatTime(reconciler.data.lastRunAt) : '-'}</span>
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-caption text-muted-foreground">Stuck</span>
                <span className={cn('font-mono text-card-title', (reconciler.data?.stuck ?? 0) > 0 && 'text-danger-fg')}>
                  {reconciler.data?.stuck ?? '-'}
                </span>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </Page>
  )
}
