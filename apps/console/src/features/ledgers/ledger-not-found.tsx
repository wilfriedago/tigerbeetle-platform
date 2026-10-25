import { useEffect } from 'react'

import { Link, useParams } from '@tanstack/react-router'

import { LibraryIcon } from 'lucide-react'

import { Page } from '~/components/layout/page'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

import { forgetLastLedger, getLastLedger } from '~/lib/last-ledger'

/** A ledger URL that names no ledger: offer the list, and stop "/" from landing here again. */
export function LedgerNotFound() {
  const { ledger } = useParams({ strict: false })
  useEffect(() => {
    if (ledger && getLastLedger() === ledger) {
      forgetLastLedger()
    }
  }, [ledger])
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <LibraryIcon />
          </EmptyMedia>
          <EmptyTitle>
            No ledger named <span className="font-mono">{ledger}</span>
          </EmptyTitle>
          <EmptyDescription>
            Ledger names are permanent, so this link was never valid here. Check the name, or pick a ledger from the list.
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button render={<Link to="/ledgers" />}>All ledgers</Button>
        </EmptyContent>
      </Empty>
    </Page>
  )
}
