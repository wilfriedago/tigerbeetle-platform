// The generated operations, one module per OpenAPI tag, and the model types (ADR-0019).
export * from './gen'
export * from './gen/model'

// The hand-written layer every operation goes through.
export { configureClient, type ClientConfig } from './client/config'
export { ApiError, HttpStatusError, parseRetryAfter, PROBLEM_TYPE_BASE, problemFrom, problemReason, problemTypeOf } from './client/errors'
export { IDEMPOTENCY_KEY_HEADER, randomRequestId, REQUEST_ID_HEADER } from './client/headers'
export { queryParams, type QueryParams } from './client/query'
