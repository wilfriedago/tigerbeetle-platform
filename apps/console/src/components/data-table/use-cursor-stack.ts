import { useState } from 'react'

/**
 * Next and Previous over API cursors (docs/console.md, Paging). The current cursor lives in the
 * URL; the cursors of the pages before it live here. A shared link that starts mid-list has no
 * history, so Previous returns to the first page.
 *
 * `scope` identifies the list (its filters): when it changes, the history starts over.
 */
export function useCursorStack({
  cursor,
  next,
  scope,
  onChange
}: {
  cursor: string | undefined
  next: string | undefined
  scope: string
  onChange: (cursor: string | undefined) => void
}) {
  const [state, setState] = useState<{ scope: string; stack: (string | undefined)[] }>({ scope, stack: [] })
  const stack = state.scope === scope ? state.stack : []
  return {
    hasNext: Boolean(next),
    hasPrevious: cursor !== undefined,
    next: () => {
      if (!next) {
        return
      }
      setState({ scope, stack: [...stack, cursor] })
      onChange(next)
    },
    previous: () => {
      setState({ scope, stack: stack.slice(0, -1) })
      onChange(stack.at(-1))
    }
  }
}
