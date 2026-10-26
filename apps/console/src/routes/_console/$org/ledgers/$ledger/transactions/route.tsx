import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/$org/ledgers/$ledger/transactions')({
  staticData: { crumb: { label: 'Transactions' } }
})
