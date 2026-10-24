import ky, { HTTPError, type KyInstance, type Options as KyOptions } from "ky";
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
  /** Generates the request ID. Defaults to crypto.randomUUID(). */
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
 * Builds a typed client. Every request carries a fresh X-Request-Id, retries idempotent calls
 * and turns problem+json answers into ApiError.
 */
export function createClient(options: ClientOptions): ApiClient {
  const requestId = options.requestId ?? (() => crypto.randomUUID());

  const kyOptions: KyOptions = {
    // ky resolves relative inputs such as "v1/_info" against baseUrl; the trailing slash keeps
    // the last path segment of a base like "/api".
    baseUrl: options.baseUrl.endsWith("/") ? options.baseUrl : `${options.baseUrl}/`,
    timeout: options.timeoutMs ?? 10_000,
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
      beforeError: [
        async ({ error }) => {
          if (!(error instanceof HTTPError)) {
            return error;
          }
          const { response } = error;
          const contentType = response.headers.get("content-type") ?? "";
          if (contentType.includes("application/problem+json")) {
            const body: unknown = await response.clone().json();
            if (isProblem(body)) {
              return new ApiError(body, response.headers.get(REQUEST_ID_HEADER) ?? undefined);
            }
          }
          return error;
        },
      ],
    },
  };
  if (options.fetch) {
    kyOptions.fetch = options.fetch;
  }
  const http = ky.create(kyOptions);

  return {
    http,
    getLiveness: () => http.get("healthz").json<Liveness>(),
    getReadiness: async () => {
      try {
        return await http.get("readyz", { retry: 0 }).json<Readiness>();
      } catch (error) {
        // 503 is a valid readiness answer, not a failure of the call.
        if (error instanceof HTTPError && error.response.status === 503) {
          return (await error.response.json()) as Readiness;
        }
        throw error;
      }
    },
    getInfo: () => http.get("v1/_info").json<Info>(),
  };
}
