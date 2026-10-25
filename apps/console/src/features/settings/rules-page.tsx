import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'

import { type Rule } from '@tigerbeetle-platform/api-client'
import { CircleCheckIcon, CircleIcon, EllipsisIcon, MinusIcon, PencilIcon, PlusIcon, Trash2Icon, TriangleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'

import { SearchFilter } from '~/components/data-table/filters'
import { Page, PageHeader } from '~/components/layout/page'
import { Hint } from '~/components/ledger/hint'
import { Tag } from '~/components/ledger/metadata'
import { ProblemPanel } from '~/components/ledger/problem-panel'
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert'
import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '~/components/ui/alert-dialog'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '~/components/ui/dropdown-menu'

import { cn } from '~/lib/utils'

import { settingsQueries, useDeleteRule } from './api'
import { RuleDialog } from './components/rule-dialog'

// docs/console.md, Account rules: the flags an account gets when it is created (ADR-0006). The
// built-in * and world rules apply until a stored rule replaces them; deleting that rule restores
// the default. "Which rule wins?" asks the API, so the console never re-implements the matching.

const route = getRouteApi('/_console/ledgers/$ledger/settings/rules')

const LIMIT_SHORT = {
  credits_must_not_exceed_debits: 'credits <= debits',
  debits_must_not_exceed_credits: 'debits <= credits',
  none: 'none'
} as const

function RuleTags({ rule }: { rule: Rule }) {
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      <Tag name="limit" value={LIMIT_SHORT[rule.limit]} />
      <Tag name="history" value={rule.history ? 'on' : 'off'} />
      <Tag name="code" value={String(rule.code)} />
    </span>
  )
}

function describe(rule: Rule): string {
  if (rule.pattern === '*') {
    return rule.builtIn ? 'catch-all, built-in' : 'catch-all, replaces the built-in default'
  }
  if (!rule.pattern.endsWith(':')) {
    return rule.builtIn ? 'exact address, built-in' : 'exact address'
  }
  return `${rule.segments} ${rule.segments === 1 ? 'segment' : 'segments'}`
}

export function RulesPage() {
  const { ledger } = route.useParams()
  const { address } = route.useSearch()
  const navigate = route.useNavigate()
  const rules = useQuery(settingsQueries.rules(ledger))
  const match = useQuery({ ...settingsQueries.match(ledger, address ?? ''), enabled: Boolean(address) })
  const [editing, setEditing] = useState<{ rule: Rule | undefined } | undefined>()
  const [deleting, setDeleting] = useState<Rule | undefined>()
  // Most specific first, the order in which they win.
  const sorted = (rules.data?.data ?? []).toSorted((a, b) => b.segments - a.segments || a.pattern.localeCompare(b.pattern))

  return (
    <Page>
      <PageHeader
        title="Account rules"
        description="Decide the flags an account gets when it is created. The most specific matching rule wins."
        actions={
          <Button onClick={() => setEditing({ rule: undefined })}>
            <PlusIcon />
            New rule
          </Button>
        }
      />
      <Alert variant="warning">
        <TriangleAlertIcon />
        <AlertTitle>Rule changes apply to new accounts only</AlertTitle>
        <AlertDescription>
          TigerBeetle account flags are immutable. Existing accounts keep the flags stored on them; route new activity to a new address to change
          them.
        </AlertDescription>
      </Alert>
      {rules.error ? <ProblemPanel error={rules.error} onRetry={() => void rules.refetch()} /> : null}
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        <ul className="flex flex-col gap-3" aria-label="Rules">
          {sorted.map((rule) => {
            const winner = match.data?.rule.pattern === rule.pattern
            return (
              <li
                key={rule.pattern}
                className={cn(
                  'flex items-center gap-4 rounded-lg border border-border bg-surface px-4 py-3',
                  winner && 'border-primary-border bg-primary-subtle'
                )}
              >
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate font-mono text-body font-medium">{rule.pattern}</span>
                  <span className="text-caption text-muted-foreground">{describe(rule)}</span>
                </div>
                <RuleTags rule={rule} />
                <DropdownMenu>
                  <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`Actions for ${rule.pattern}`} />}>
                    <EllipsisIcon />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => setEditing({ rule })}>
                      <PencilIcon />
                      {rule.builtIn ? 'Override the default' : 'Edit'}
                    </DropdownMenuItem>
                    {rule.builtIn ? null : (
                      <DropdownMenuItem variant="destructive" onClick={() => setDeleting(rule)}>
                        <Trash2Icon />
                        Delete
                      </DropdownMenuItem>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </li>
            )
          })}
        </ul>
        <div className="flex flex-col gap-3 self-start">
          <Card>
            <CardHeader>
              <CardTitle>Which rule wins?</CardTitle>
              <CardDescription>Type an address to see the flags it would get.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <SearchFilter
                label="Address to test"
                placeholder="treasury:fx:eur"
                value={address}
                onChange={(next) => void navigate({ replace: true, search: { address: next } })}
                className="sm:w-full"
              />
              {match.error ? <ProblemPanel error={match.error} /> : null}
              {match.data ? (
                <>
                  <ul className="flex flex-col gap-1">
                    {match.data.candidates
                      .toSorted((a, b) => Number(b.matches) - Number(a.matches) || b.rule.segments - a.rule.segments)
                      .map((candidate) => {
                        const wins = candidate.rule.pattern === match.data.rule.pattern
                        return (
                          <li
                            key={candidate.rule.pattern}
                            className={cn('flex items-center gap-2 rounded-md px-2 py-1.5 text-small', wins && 'bg-primary-subtle')}
                          >
                            {wins ? (
                              <CircleCheckIcon aria-hidden="true" className="size-4 text-primary-text" />
                            ) : candidate.matches ? (
                              <CircleIcon aria-hidden="true" className="size-4 text-muted-foreground" />
                            ) : (
                              <MinusIcon aria-hidden="true" className="size-4 text-subtle-foreground" />
                            )}
                            <span className={cn('flex-1 font-mono', !candidate.matches && 'text-subtle-foreground')}>{candidate.rule.pattern}</span>
                            <span className={cn('text-caption', wins ? 'text-primary-text' : 'text-muted-foreground')}>
                              {candidate.matches
                                ? `${wins ? 'wins' : 'matches'}, ${candidate.rule.segments} ${candidate.rule.segments === 1 ? 'segment' : 'segments'}`
                                : 'no match'}
                            </span>
                          </li>
                        )
                      })}
                  </ul>
                  <div className="border-t border-border-subtle pt-3">
                    <RuleTags rule={match.data.rule} />
                  </div>
                  {match.data.existing.length > 0 ? (
                    <p className="text-caption text-muted-foreground">
                      This address already has accounts in {match.data.existing.join(', ')}; they keep their flags. The rule applies to its new
                      assets.
                    </p>
                  ) : null}
                </>
              ) : null}
            </CardContent>
          </Card>
          <Hint>
            Prefixes match whole segments: <span className="font-mono">users:1</span> does not match <span className="font-mono">users:12</span>.
          </Hint>
        </div>
      </div>
      <RuleDialog ledger={ledger} rule={editing?.rule} open={editing !== undefined} onOpenChange={(open) => !open && setEditing(undefined)} />
      <DeleteRuleDialog ledger={ledger} rule={deleting} onClose={() => setDeleting(undefined)} />
    </Page>
  )
}

function DeleteRuleDialog({ ledger, rule, onClose }: { ledger: string; rule: Rule | undefined; onClose: () => void }) {
  const remove = useDeleteRule(ledger)
  const restoresDefault = rule?.pattern === '*' || rule?.pattern === 'world'
  return (
    <AlertDialog open={rule !== undefined} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete rule {rule?.pattern}?</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogBody>
          <AlertDialogDescription>
            {restoresDefault
              ? 'The built-in default applies again to accounts created from now on.'
              : 'Accounts created from now on fall back to the next most specific rule.'}{' '}
            Existing accounts keep their flags.
          </AlertDialogDescription>
          {remove.error ? <ProblemPanel error={remove.error} /> : null}
        </AlertDialogBody>
        <AlertDialogFooter>
          <AlertDialogCancel>Keep the rule</AlertDialogCancel>
          <Button
            variant="destructive"
            loading={remove.isPending}
            onClick={() => {
              if (!rule) {
                return
              }
              remove.mutate(rule.pattern, {
                onSuccess: () => {
                  toast.success(`Rule ${rule.pattern} deleted`)
                  onClose()
                }
              })
            }}
          >
            Delete rule
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
