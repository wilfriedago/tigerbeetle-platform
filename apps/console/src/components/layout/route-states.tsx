import { type ErrorComponentProps, Link, useRouter } from '@tanstack/react-router'

import { CompassIcon } from 'lucide-react'

import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'
import { Skeleton } from '~/components/ui/skeleton'

import { Page } from './page'

/** The error boundary of every route: the problem the API answered, and a retry. */
export function RouteError({ error, reset }: ErrorComponentProps) {
  const router = useRouter()
  return (
    <Page>
      <ProblemPanel
        error={error}
        onRetry={() => {
          reset()
          void router.invalidate()
        }}
      />
    </Page>
  )
}

export function RouteNotFound() {
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <CompassIcon />
          </EmptyMedia>
          <EmptyTitle>Nothing here</EmptyTitle>
          <EmptyDescription>This page does not exist, or the record it names was never created in this ledger.</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button render={<Link to="/" />}>Back to the console</Button>
        </EmptyContent>
      </Empty>
    </Page>
  )
}

/** Skeletons keep the layout while data loads; never a full-page spinner (12 Feedback). */
export function PageSkeleton() {
  return (
    <Page aria-busy="true">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-7 w-56" />
        <Skeleton className="w-80" />
      </div>
      <div className="flex flex-col gap-3 rounded-lg border border-border bg-surface p-5">
        <Skeleton className="w-1/3" />
        <Skeleton className="w-2/3" />
        <Skeleton className="w-1/2" />
        <Skeleton className="w-3/5" />
      </div>
    </Page>
  )
}
