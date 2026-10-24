import ky, { type KyInstance, type Options as KyOptions, type KyResponse } from "ky";
import type { Info, Liveness, Problem, Readiness } from "./schema";

export type {
  components,
  DependencyCheck,
  Info,
  Liveness,
  operations,
  Problem,
  paths,
  Readiness,
} from "./schema";

/** Header carried on every request and echoed back by the API. */
export const REQUEST_ID_HEADER = "X-Request-Id";

export interface ClientOptions {
  /** Base URL of the API, for example "http://localhost:8080" or "/api" behind a dev proxy. */
  baseUrl: string;
  /** Override fetch (tests, server-side rendering). */
  fetch?: typeof globalThis.fetch;
  /** Per-request timeout in milliseconds. */
  timeoutMs?: number;
  /** Retries for idempotent requests on network errors and 5xx answers. */
  retries?: number;
  /** Generates the request ID. Defaults to a random 128-bit hex string. */
  requestId?: () => string;
}

/** Thrown when the API answers with an RFC 9457 problem. */
export class ApiError extends Error {
  readonly problem: Problem;
  readonly status: number;
  readonly requestId: string | undefined;

  constructor(problem: Problem, requestId: string | undefined) {
    super(problem.detail ? `${problem.title}: ${problem.detail}` : problem.title);
    this.name = "ApiError";
    this.problem = problem;
    this.status = problem.status;
    this.requestId = requestId;
  }
}

/** Thrown when the API answers with a non-2xx status that is not a problem document. */
export class HttpStatusError extends Error {
  readonly status: number;
  readonly requestId: string | undefined;

  constructor(status: number, statusText: string, requestId: string | undefined) {
    super(`unexpected status ${status} ${statusText}`.trim());
    this.name = "HttpStatusError";
    this.status = status;
    this.requestId = requestId;
  }
}

export interface ApiClient {
  /** GET /healthz */
  getLiveness(): Promise<Liveness>;
  /** GET /readyz. Resolves on both 200 and 503; the body says which. */
  getReadiness(): Promise<Readiness>;
  /** GET /v1/_info */
  getInfo(): Promise<Info>;
  /** The underlying ky instance for endpoints not wrapped yet. */
  readonly http: KyInstance;
}

function isProblem(value: unknown): value is Problem {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as Problem).title === "string" &&
    typeof (value as Problem).status === "number"
  );
}

/**
 * Random request ID that also works outside secure contexts (plain http on a LAN address),
 * where crypto.randomUUID is not available but getRandomValues is.
 */
export function randomRequestId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
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
  const requestId = options.requestId ?? randomRequestId;

  const kyOptions: KyOptions = {
    // ky resolves relative inputs such as "v1/_info" against baseUrl; the trailing slash keeps
    // the last path segment of a base like "/api".
    baseUrl: options.baseUrl.endsWith("/") ? options.baseUrl : `${options.baseUrl}/`,
    timeout: options.timeoutMs ?? 10_000,
    throwHttpErrors: false,
    retry: {
      limit: options.retries ?? 2,
      methods: ["get", "head", "options"],
      statusCodes: [502, 503, 504],
    },
    hooks: {
      beforeRequest: [
        ({ request }) => {
          request.headers.set(REQUEST_ID_HEADER, requestId());
          request.headers.set("Accept", "application/json, application/problem+json");
        },
      ],
    },
  };
  if (options.fetch) {
    kyOptions.fetch = options.fetch;
  }
  const http = ky.create(kyOptions);

  async function decode<T>(
    response: KyResponse,
    acceptStatuses: readonly number[] = [],
  ): Promise<T> {
    if (response.ok || acceptStatuses.includes(response.status)) {
      return (await response.json()) as T;
    }
    const id = response.headers.get(REQUEST_ID_HEADER) ?? undefined;
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("application/problem+json")) {
      const body: unknown = await response.json();
      if (isProblem(body)) {
        throw new ApiError(body, id);
      }
    }
    throw new HttpStatusError(response.status, response.statusText, id);
  }

  return {
    http,
    getLiveness: async () => decode<Liveness>(await http.get("healthz")),
    // 503 is a valid readiness answer, so it is neither retried nor treated as an error.
    getReadiness: async () => decode<Readiness>(await http.get("readyz", { retry: 0 }), [503]),
    getInfo: async () => decode<Info>(await http.get("v1/_info")),
  };
}
