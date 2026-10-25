/** Query parameters without their unset members. */
export type QueryParams<T> = { [K in keyof T]?: Exclude<T[K], undefined | null> }

/**
 * Drops the members of a query object that are undefined, null or empty, so filters can be passed
 * as the console holds them ({ cursor: undefined, search: '' }) to the generated functions, whose
 * optional parameters do not accept undefined under exactOptionalPropertyTypes.
 */
export function queryParams<T extends object>(params: T): QueryParams<T> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      out[key] = value
    }
  }
  return out as QueryParams<T>
}
