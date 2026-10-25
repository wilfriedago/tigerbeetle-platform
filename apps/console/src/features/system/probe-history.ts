import { useSyncExternalStore } from 'react'

import { type Info } from '@tigerbeetle-platform/api-client'

/**
 * The last probes of GET /v1/_info, kept in memory while the console is open, for the latency
 * chart on System health. The top bar polls the endpoint anyway, so recording costs nothing.
 */
export interface ProbeSample {
  at: number
  postgresMs: number | undefined
  tigerbeetleMs: number | undefined
  up: boolean
}

export const PROBE_HISTORY_SIZE = 60

let samples: readonly ProbeSample[] = []
const listeners = new Set<() => void>()

export function recordProbe(at: number, info: Info | undefined): void {
  if (samples.at(-1)?.at === at) {
    return
  }
  const { postgres, tigerbeetle } = info?.dependencies ?? {}
  const sample: ProbeSample = {
    at,
    postgresMs: postgres?.status === 'up' ? postgres.latencyMs : undefined,
    tigerbeetleMs: tigerbeetle?.status === 'up' ? tigerbeetle.latencyMs : undefined,
    up: postgres?.status === 'up' && tigerbeetle?.status === 'up'
  }
  samples = [...samples, sample].slice(-PROBE_HISTORY_SIZE)
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

const EMPTY: readonly ProbeSample[] = []

export function useProbeHistory(): readonly ProbeSample[] {
  return useSyncExternalStore(
    subscribe,
    () => samples,
    () => EMPTY
  )
}
