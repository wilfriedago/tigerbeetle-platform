import { type CreateTransactionRequest, type Transaction } from '@tigerbeetle-platform/api-client'

/**
 * The request that creates a transaction like this one: what "Copy as curl" and the request
 * view show. The default event type is left out, exactly as a client that names none would send.
 */
export function requestBodyOf(transaction: Transaction): CreateTransactionRequest {
  const body: CreateTransactionRequest = {
    postings: transaction.postings.map(({ source, destination, asset, amount }) => ({ amount, asset, destination, source }))
  }
  if (transaction.reference) {
    body.reference = transaction.reference
  }
  if (transaction.eventType.name !== 'default') {
    body.eventType = transaction.eventType.name
  }
  if (transaction.businessTime) {
    body.businessTime = transaction.businessTime
  }
  if (Object.keys(transaction.metadata).length > 0) {
    body.metadata = transaction.metadata
  }
  return body
}
