import { notFound } from '@tanstack/react-router'

import { isNotFound } from './problem'

/**
 * Turns a 404 from the API into the router's not-found state, so a link to a record that does not
 * exist renders "Nothing here" instead of an error panel. Other failures stay errors.
 */
export async function orNotFound<T>(load: Promise<T>): Promise<T> {
  try {
    return await load
  } catch (error) {
    if (isNotFound(error)) {
      throw notFound()
    }
    throw error
  }
}
