import { type CreateTransactionRequest, type Posting } from '@tigerbeetle-platform/api-client'

import { type MetadataRow, rowsToMetadata } from '~/components/ledger/metadata-editor'

import { parseAsset, parseMajorUnits } from '~/lib/money'
import { addressError } from '~/lib/validation'

/** What the New transaction form edits. Amounts are typed in major units and converted on the way out. */
export interface PostingDraft {
  source: string
  destination: string
  asset: string
  /** Major units as typed: "1,250.00". */
  amount: string
}

export interface TransactionDraft {
  postings: PostingDraft[]
  reference: string
  eventType: string
  /** yyyy-mm-dd from the date input, or empty. */
  businessTime: string
  metadata: MetadataRow[]
}

/** The posting cap of a transaction (MaxPostings in apps/api/internal/domain/ledger/posting.go). */
export const MAX_POSTINGS = 64

export function emptyPosting(asset = '', source = '', destination = ''): PostingDraft {
  return { amount: '', asset, destination, source }
}

/** The amount integer the API receives, or undefined while the input is not valid. */
export function rawAmount(posting: PostingDraft): string | undefined {
  const asset = parseAsset(posting.asset)
  if (!asset) {
    return undefined
  }
  const parsed = parseMajorUnits(posting.amount, asset.scale)
  return parsed.ok && parsed.raw !== '0' ? parsed.raw : undefined
}

/** The postings that are complete enough to preview, in API form. Incomplete ones keep their place with an empty amount. */
export function previewPostings(postings: readonly PostingDraft[]): Posting[] {
  return postings.map((posting) => {
    const asset = parseAsset(posting.asset)
    return {
      amount: rawAmount(posting) ?? '',
      asset: asset ? `${asset.code}/${asset.scale}` : posting.asset,
      destination: posting.destination,
      source: posting.source
    }
  })
}

export function postingReady(posting: PostingDraft): boolean {
  return (
    addressError(posting.source, 'source') === undefined &&
    addressError(posting.destination, 'destination') === undefined &&
    posting.source !== posting.destination &&
    rawAmount(posting) !== undefined
  )
}

/** The request body. Call it on a validated draft; an invalid posting would carry an empty amount. */
export function toRequest(draft: TransactionDraft): CreateTransactionRequest {
  const body: CreateTransactionRequest = { postings: previewPostings(draft.postings) }
  if (draft.reference.trim()) {
    body.reference = draft.reference.trim()
  }
  if (draft.eventType && draft.eventType !== 'default') {
    body.eventType = draft.eventType
  }
  if (draft.businessTime) {
    // A calendar date is midnight UTC; the API stores business time in milliseconds.
    body.businessTime = `${draft.businessTime}T00:00:00Z`
  }
  const metadata = rowsToMetadata(draft.metadata)
  if (Object.keys(metadata).length > 0) {
    body.metadata = metadata
  }
  return body
}
