import { Fragment } from 'react'

import { Link } from '@tanstack/react-router'

import { GlobeIcon } from 'lucide-react'

import { useOrganization } from '~/lib/organization'
import { cn } from '~/lib/utils'

// 20 Money and identity, Address: keeps its colon-separated segments, separators in a quieter tone
// (ADR-0004). `world`, where money enters and leaves the ledger, carries the globe icon.

export const WORLD = 'world'

interface AddressProps {
  value: string
  /** Links the address to its account page in this ledger. */
  ledger?: string
  className?: string
  /** Mono 13 px by default; `md` is 14 px for headers and pickers. */
  size?: 'sm' | 'md'
}

function Segments({ value }: { value: string }) {
  const segments = value.split(':')
  return (
    <>
      {segments.map((segment, index) => (
        // Segments can repeat (users:1:users), so the position is part of the key.
        // oxlint-disable-next-line react/no-array-index-key
        <Fragment key={`${index}-${segment}`}>
          {index > 0 ? <span className="text-subtle-foreground">:</span> : null}
          <span>{segment}</span>
        </Fragment>
      ))}
    </>
  )
}

export function Address({ value, ledger, className, size = 'sm' }: AddressProps) {
  const org = useOrganization()
  const isWorld = value === WORLD
  const content = (
    <>
      {isWorld ? <GlobeIcon aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" /> : null}
      {isWorld ? <span>{value}</span> : <Segments value={value} />}
    </>
  )
  const classes = cn(
    'inline-flex min-w-0 items-center font-mono whitespace-nowrap text-foreground',
    isWorld && 'gap-1.5',
    size === 'md' ? 'text-body' : 'text-small',
    className
  )
  if (ledger) {
    return (
      <Link
        to="/$org/ledgers/$ledger/accounts/$address"
        params={{ address: value, ledger, org }}
        className={cn(classes, 'rounded-xs underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-focus-ring')}
      >
        {content}
      </Link>
    )
  }
  return <span className={classes}>{content}</span>
}

/**
 * The account header form: each segment links to Balances at that prefix (docs/console.md,
 * Account), so `users` opens the sum of every account under `users:`.
 */
export function AddressPrefixLinks({ value, ledger, className }: { value: string; ledger: string; className?: string }) {
  const org = useOrganization()
  const segments = value.split(':')
  return (
    <span className={cn('inline-flex flex-wrap items-baseline font-mono', className)}>
      {segments.map((segment, index) => {
        const prefix = segments.slice(0, index + 1).join(':')
        const isLast = index === segments.length - 1
        return (
          // oxlint-disable-next-line react/no-array-index-key
          <Fragment key={`${index}-${segment}`}>
            {index > 0 ? <span className="text-subtle-foreground">:</span> : null}
            {isLast ? (
              <span>{segment}</span>
            ) : (
              <Link
                to="/$org/ledgers/$ledger/balances"
                params={{ ledger, org }}
                search={{ prefix }}
                title={`Sum balances under ${prefix}:`}
                className="rounded-xs outline-none hover:text-primary-text hover:underline hover:underline-offset-4 focus-visible:ring-3 focus-visible:ring-focus-ring"
              >
                {segment}
              </Link>
            )}
          </Fragment>
        )
      })}
    </span>
  )
}
