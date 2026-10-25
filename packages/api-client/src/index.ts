import ky, { type KyInstance, type Options as KyOptions, type KyResponse } from 'ky'

// schema.d.ts has no runtime module: an inline `type` specifier would leave `import './schema'` in
// the bundle, so this one import stays top-level.
// oxlint-disable-next-line import/consistent-type-specifier-style
import type {
  Account,
  AccountList,
  AggregateBalances,
  AssetList,
  BalanceHistory,
  CaptureHoldRequest,
  CreateEventTypeRequest,
  CreateHoldRequest,
  CreateLedgerRequest,
  CreateTransactionRequest,
  EventType,
  EventTypeList,
  Hold,
  HoldList,
  Info,
  Ledger,
  LedgerList,
  LedgerStats,
  Liveness,
  Metadata,
  operations,
  Problem,
  PutRuleRequest,
  Readiness,
  ReconcilerStatus,
  RevertTransactionRequest,
  Rule,
  RuleList,
  RuleMatch,
  SearchResults,
  SegmentList,
  Transaction,
  TransactionList,
  WriteRequestDetail,
  WriteRequestList
} from './schema'

export type * from './schema'

/** Header carried on every request and echoed back by the API. */
export const REQUEST_ID_HEADER = 'X-Request-Id'

/** Header that makes a write that moves money safe to retry (ADR-0007). */
export const IDEMPOTENCY_KEY_HEADER = 'Idempotency-Key'

export interface ClientOptions {
  /** Base URL of the API, for example "http://localhost:8080" or "/api" behind a dev proxy. */
  baseUrl: string
  /** Override fetch (tests, server-side rendering). */
  fetch?: typeof globalThis.fetch
  /** Per-request timeout in milliseconds. */
  timeoutMs?: number
  /** Retries for idempotent requests on network errors and 5xx answers. */
  retries?: number
  /** Generates the request ID. Defaults to a random 128-bit hex string. */
  requestId?: () => string
}

/** Options every call accepts. TanStack Query passes its abort signal here. */
export interface CallOptions {
  signal?: AbortSignal
}

/** Thrown when the API answers with an RFC 9457 problem. */
export class ApiError extends Error {
  readonly problem: Problem
  readonly status: number
  readonly requestId: string | undefined
  /** Seconds from the Retry-After header, for 409 request-in-flight and 503 write-pending. */
  readonly retryAfter: number | undefined

  constructor(problem: Problem, requestId: string | undefined, retryAfter?: number) {
    super(problem.detail ? `${problem.title}: ${problem.detail}` : problem.title)
    this.name = 'ApiError'
    this.problem = problem
    this.status = problem.status
    this.requestId = requestId
    this.retryAfter = retryAfter
  }

  /** The problem type, such as "/problems/exceeds-credits". */
  get type(): string {
    return this.problem.type ?? 'about:blank'
  }
}

/** Thrown when the API answers with a non-2xx status that is not a problem document. */
export class HttpStatusError extends Error {
  readonly status: number
  readonly requestId: string | undefined

  constructor(status: number, statusText: string, requestId: string | undefined) {
    super(`unexpected status ${status} ${statusText}`.trim())
    this.name = 'HttpStatusError'
    this.status = status
    this.requestId = requestId
  }
}

type QueryOf<Op extends keyof operations> = operations[Op]['parameters'] extends { query?: infer Q } ? NonNullable<Q> : never

/** The query parameters of an operation. An undefined value is not sent, so callers can pass filters as they are. */
type Query<Op extends keyof operations> = { [K in keyof QueryOf<Op>]?: QueryOf<Op>[K] | undefined }

export interface ApiClient {
  /** GET /healthz */
  getLiveness: (options?: CallOptions) => Promise<Liveness>
  /** GET /readyz. Resolves on both 200 and 503; the body says which. */
  getReadiness: (options?: CallOptions) => Promise<Readiness>
  /** GET /v1/_info */
  getInfo: (options?: CallOptions) => Promise<Info>
  /** GET /v1/_reconciler */
  getReconcilerStatus: (options?: CallOptions) => Promise<ReconcilerStatus>
  ledgers: {
    list: (query?: Query<'listLedgers'>, options?: CallOptions) => Promise<LedgerList>
    get: (ledger: string, options?: CallOptions) => Promise<Ledger>
    create: (ledger: string, body: CreateLedgerRequest, options?: CallOptions) => Promise<Ledger>
    replaceMetadata: (ledger: string, metadata: Metadata, options?: CallOptions) => Promise<Ledger>
    stats: (ledger: string, options?: CallOptions) => Promise<LedgerStats>
    assets: (ledger: string, options?: CallOptions) => Promise<AssetList>
    search: (ledger: string, q: string, options?: CallOptions) => Promise<SearchResults>
  }
  accounts: {
    list: (ledger: string, query?: Query<'listAccounts'>, options?: CallOptions) => Promise<AccountList>
    get: (ledger: string, address: string, options?: CallOptions) => Promise<Account>
    replaceMetadata: (ledger: string, address: string, metadata: Metadata, options?: CallOptions) => Promise<Account>
    history: (ledger: string, address: string, query: Query<'getAccountHistory'>, options?: CallOptions) => Promise<BalanceHistory>
    aggregate: (ledger: string, address: string, options?: CallOptions) => Promise<AggregateBalances>
    segments: (ledger: string, prefix?: string, options?: CallOptions) => Promise<SegmentList>
  }
  transactions: {
    list: (ledger: string, query?: Query<'listTransactions'>, options?: CallOptions) => Promise<TransactionList>
    get: (ledger: string, id: string, options?: CallOptions) => Promise<Transaction>
    create: (ledger: string, body: CreateTransactionRequest, idempotencyKey: string, options?: CallOptions) => Promise<Transaction>
    revert: (ledger: string, id: string, body: RevertTransactionRequest, idempotencyKey: string, options?: CallOptions) => Promise<Transaction>
    replaceMetadata: (ledger: string, id: string, metadata: Metadata, options?: CallOptions) => Promise<Transaction>
  }
  holds: {
    list: (ledger: string, query?: Query<'listHolds'>, options?: CallOptions) => Promise<HoldList>
    get: (ledger: string, id: string, options?: CallOptions) => Promise<Hold>
    create: (ledger: string, body: CreateHoldRequest, idempotencyKey: string, options?: CallOptions) => Promise<Hold>
    capture: (ledger: string, id: string, body: CaptureHoldRequest, idempotencyKey: string, options?: CallOptions) => Promise<Hold>
    void: (ledger: string, id: string, idempotencyKey: string, options?: CallOptions) => Promise<Hold>
  }
  requests: {
    list: (ledger: string, query?: Query<'listWriteRequests'>, options?: CallOptions) => Promise<WriteRequestList>
    get: (ledger: string, key: string, options?: CallOptions) => Promise<WriteRequestDetail>
  }
  rules: {
    list: (ledger: string, options?: CallOptions) => Promise<RuleList>
    match: (ledger: string, address: string, options?: CallOptions) => Promise<RuleMatch>
    put: (ledger: string, pattern: string, body: PutRuleRequest, options?: CallOptions) => Promise<Rule>
    delete: (ledger: string, pattern: string, options?: CallOptions) => Promise<void>
  }
  eventTypes: {
    list: (ledger: string, options?: CallOptions) => Promise<EventTypeList>
    create: (ledger: string, body: CreateEventTypeRequest, options?: CallOptions) => Promise<EventType>
  }
  /** The underlying ky instance for endpoints not wrapped yet. */
  readonly http: KyInstance
}

function isProblem(value: unknown): value is Problem {
  return typeof value === 'object' && value !== null && typeof (value as Problem).title === 'string' && typeof (value as Problem).status === 'number'
}

/**
 * Random request ID that also works outside secure contexts (plain http on a LAN address),
 * where crypto.randomUUID is not available but getRandomValues is.
 */
export function randomRequestId(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

/** Path segments are encoded, so an address such as "users:1234:wallet" or the pattern "*" is safe. */
function segment(value: string): string {
  return encodeURIComponent(value)
}

/** Drops undefined values so they do not become "undefined" in the query string. */
function searchParams(query: object | undefined): Record<string, string | number | boolean> | undefined {
  if (!query) {
    return undefined
  }
  const out: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') {
      out[key] = value as string | number | boolean
    }
  }
  return out
}

/**
 * Builds a typed client. Every request carries a fresh X-Request-Id, idempotent calls are
 * retried, and problem+json answers become ApiError.
 *
 * ky's own HTTPError path reads the body before any hook runs, so error bodies are decoded here
 * instead: HTTP errors are not thrown by ky (throwHttpErrors: false) and `decode` inspects the
 * status itself.
 */
export function createClient(options: ClientOptions): ApiClient {
  const requestId = options.requestId ?? randomRequestId

  const kyOptions: KyOptions = {
    // ky resolves relative inputs such as "v1/_info" against baseUrl; the trailing slash keeps
    // the last path segment of a base like "/api".
    baseUrl: options.baseUrl.endsWith('/') ? options.baseUrl : `${options.baseUrl}/`,
    timeout: options.timeoutMs ?? 15_000,
    throwHttpErrors: false,
    // Writes are never retried by the client: retrying is the caller's decision, made safe by the
    // Idempotency-Key it keeps.
    retry: {
      limit: options.retries ?? 2,
      methods: ['get', 'head', 'options'],
      statusCodes: [502, 503, 504]
    },
    hooks: {
      beforeRequest: [
        ({ request }) => {
          request.headers.set(REQUEST_ID_HEADER, requestId())
          request.headers.set('Accept', 'application/json, application/problem+json')
        }
      ]
    }
  }
  if (options.fetch) {
    kyOptions.fetch = options.fetch
  }
  const http = ky.create(kyOptions)

  async function decode<T>(response: KyResponse, acceptStatuses: readonly number[] = []): Promise<T> {
    if (response.status === 204) {
      return undefined as T
    }
    if (response.ok || acceptStatuses.includes(response.status)) {
      return await response.json()
    }
    const id = response.headers.get(REQUEST_ID_HEADER) ?? undefined
    const contentType = response.headers.get('content-type') ?? ''
    if (contentType.includes('application/problem+json')) {
      const body: unknown = await response.json()
      if (isProblem(body)) {
        const retry = Number(response.headers.get('Retry-After'))
        throw new ApiError(body, id, Number.isFinite(retry) && retry > 0 ? retry : undefined)
      }
    }
    throw new HttpStatusError(response.status, response.statusText, id)
  }

  function signal(o: CallOptions | undefined): KyOptions {
    return o?.signal ? { signal: o.signal } : {}
  }

  const get = async <T>(path: string, query?: object, o?: CallOptions) => {
    const params = searchParams(query)
    return decode<T>(await http.get(path, { ...signal(o), ...(params ? { searchParams: params } : {}) }))
  }
  const send = async <T>(method: 'post' | 'put' | 'delete', path: string, body?: unknown, idempotencyKey?: string, o?: CallOptions) =>
    decode<T>(
      await http(path, {
        ...signal(o),
        method,
        ...(body === undefined ? {} : { json: body }),
        ...(idempotencyKey ? { headers: { [IDEMPOTENCY_KEY_HEADER]: idempotencyKey } } : {})
      })
    )

  const ledgerPath = (ledger: string) => `v1/ledgers/${segment(ledger)}`

  return {
    http,
    getLiveness: async (o) => get<Liveness>('healthz', undefined, o),
    // 503 is a valid readiness answer, so it is neither retried nor treated as an error.
    getReadiness: async (o) => decode<Readiness>(await http.get('readyz', { retry: 0, ...signal(o) }), [503]),
    getInfo: async (o) => get<Info>('v1/_info', undefined, o),
    getReconcilerStatus: async (o) => get<ReconcilerStatus>('v1/_reconciler', undefined, o),
    ledgers: {
      list: async (query, o) => get<LedgerList>('v1/ledgers', query, o),
      get: async (ledger, o) => get<Ledger>(ledgerPath(ledger), undefined, o),
      create: async (ledger, body, o) => send<Ledger>('post', ledgerPath(ledger), body, undefined, o),
      replaceMetadata: async (ledger, metadata, o) => send<Ledger>('put', `${ledgerPath(ledger)}/metadata`, metadata, undefined, o),
      stats: async (ledger, o) => get<LedgerStats>(`${ledgerPath(ledger)}/stats`, undefined, o),
      assets: async (ledger, o) => get<AssetList>(`${ledgerPath(ledger)}/assets`, undefined, o),
      search: async (ledger, q, o) => get<SearchResults>(`${ledgerPath(ledger)}/search`, { q }, o)
    },
    accounts: {
      list: async (ledger, query, o) => get<AccountList>(`${ledgerPath(ledger)}/accounts`, query, o),
      get: async (ledger, address, o) => get<Account>(`${ledgerPath(ledger)}/accounts/${segment(address)}`, undefined, o),
      replaceMetadata: async (ledger, address, metadata, o) =>
        send<Account>('put', `${ledgerPath(ledger)}/accounts/${segment(address)}/metadata`, metadata, undefined, o),
      history: async (ledger, address, query, o) => get<BalanceHistory>(`${ledgerPath(ledger)}/accounts/${segment(address)}/history`, query, o),
      aggregate: async (ledger, address, o) => get<AggregateBalances>(`${ledgerPath(ledger)}/aggregate/balances`, { address }, o),
      segments: async (ledger, prefix, o) => get<SegmentList>(`${ledgerPath(ledger)}/aggregate/segments`, { prefix }, o)
    },
    transactions: {
      list: async (ledger, query, o) => get<TransactionList>(`${ledgerPath(ledger)}/transactions`, query, o),
      get: async (ledger, id, o) => get<Transaction>(`${ledgerPath(ledger)}/transactions/${segment(id)}`, undefined, o),
      create: async (ledger, body, key, o) => send<Transaction>('post', `${ledgerPath(ledger)}/transactions`, body, key, o),
      revert: async (ledger, id, body, key, o) => send<Transaction>('post', `${ledgerPath(ledger)}/transactions/${segment(id)}/revert`, body, key, o),
      replaceMetadata: async (ledger, id, metadata, o) =>
        send<Transaction>('put', `${ledgerPath(ledger)}/transactions/${segment(id)}/metadata`, metadata, undefined, o)
    },
    holds: {
      list: async (ledger, query, o) => get<HoldList>(`${ledgerPath(ledger)}/holds`, query, o),
      get: async (ledger, id, o) => get<Hold>(`${ledgerPath(ledger)}/holds/${segment(id)}`, undefined, o),
      create: async (ledger, body, key, o) => send<Hold>('post', `${ledgerPath(ledger)}/holds`, body, key, o),
      capture: async (ledger, id, body, key, o) => send<Hold>('post', `${ledgerPath(ledger)}/holds/${segment(id)}/capture`, body, key, o),
      void: async (ledger, id, key, o) => send<Hold>('post', `${ledgerPath(ledger)}/holds/${segment(id)}/void`, undefined, key, o)
    },
    requests: {
      list: async (ledger, query, o) => get<WriteRequestList>(`${ledgerPath(ledger)}/requests`, query, o),
      get: async (ledger, key, o) => get<WriteRequestDetail>(`${ledgerPath(ledger)}/requests/${segment(key)}`, undefined, o)
    },
    rules: {
      list: async (ledger, o) => get<RuleList>(`${ledgerPath(ledger)}/rules`, undefined, o),
      match: async (ledger, address, o) => get<RuleMatch>(`${ledgerPath(ledger)}/rules/match`, { address }, o),
      put: async (ledger, pattern, body, o) => send<Rule>('put', `${ledgerPath(ledger)}/rules/${segment(pattern)}`, body, undefined, o),
      delete: async (ledger, pattern, o) => send<undefined>('delete', `${ledgerPath(ledger)}/rules/${segment(pattern)}`, undefined, undefined, o)
    },
    eventTypes: {
      list: async (ledger, o) => get<EventTypeList>(`${ledgerPath(ledger)}/event-types`, undefined, o),
      create: async (ledger, body, o) => send<EventType>('post', `${ledgerPath(ledger)}/event-types`, body, undefined, o)
    }
  }
}
