import { formOptions } from '@tanstack/react-form'

import { emptyPosting, type TransactionDraft } from './draft'

const defaultValues: TransactionDraft = {
  businessTime: '',
  eventType: 'default',
  metadata: [],
  postings: [emptyPosting()],
  reference: ''
}

export const transactionFormOptions = formOptions({ defaultValues })
