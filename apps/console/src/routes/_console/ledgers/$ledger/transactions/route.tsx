import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_console/ledgers/$ledger/transactions')({
  staticData: { crumb: { label: 'Transactions' } }
})
