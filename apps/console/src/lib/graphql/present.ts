/**
 * A GraphQL result as the REST types declare it: a nullable member is optional and absent rather
 * than null. The console's components take the REST types of packages/api-client, which are the
 * same Go types the API answers on both transports (plan section 8.8); present() is the one place
 * the GraphQL form becomes that one.
 */
export type Present<T> = T extends readonly (infer U)[]
  ? Present<U>[]
  : T extends object
    ? { [K in keyof T as null extends T[K] ? never : undefined extends T[K] ? never : K]: Present<T[K]> } & {
        [K in keyof T as null extends T[K] ? K : undefined extends T[K] ? K : never]?: Present<Exclude<T[K], null | undefined>>
      }
    : T

/** Removes the null members of a GraphQL result, deeply. */
export function present<T>(value: T): Present<T> {
  return strip(value) as Present<T>
}

function strip(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(strip)
  }
  if (value === null || typeof value !== 'object') {
    return value
  }
  const out: Record<string, unknown> = {}
  for (const [key, member] of Object.entries(value)) {
    if (member !== null && member !== undefined) {
      out[key] = strip(member)
    }
  }
  return out
}
