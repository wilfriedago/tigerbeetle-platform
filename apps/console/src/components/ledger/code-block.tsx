import { useMemo } from 'react'

import { cn } from '~/lib/utils'

import { CopyButton } from './copy-button'

// 15 Data display, Code Block: request bodies and stored responses on the sunken surface, with the
// method and path in the header and token colours for keys, strings, numbers and punctuation.

type TokenKind = 'key' | 'string' | 'number' | 'punct' | 'plain'

interface Token {
  kind: TokenKind
  text: string
}

const TOKEN_PATTERN =
  /(?<str>"(?:[^"\\]|\\.)*")(?<colon>\s*:)?|(?<num>-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|(?<literal>true|false|null)|(?<punct>[{}[\],:])|(?<space>\s+)|(?<other>.)/gu

/** Splits pretty-printed JSON into tokens; enough for display, not a validating parser. */
export function tokenizeJson(source: string): Token[][] {
  return source.split('\n').map((line) => {
    const tokens: Token[] = []
    for (const match of line.matchAll(TOKEN_PATTERN)) {
      const { str, colon, num, literal, punct, space, other } = match.groups ?? {}
      if (str !== undefined) {
        tokens.push({ kind: colon === undefined ? 'string' : 'key', text: str })
        if (colon !== undefined) {
          tokens.push({ kind: 'punct', text: colon })
        }
      } else if (num !== undefined || literal !== undefined) {
        tokens.push({ kind: 'number', text: num ?? literal ?? '' })
      } else if (punct !== undefined) {
        tokens.push({ kind: 'punct', text: punct })
      } else {
        tokens.push({ kind: 'plain', text: space ?? other ?? '' })
      }
    }
    return tokens
  })
}

const TOKEN_CLASS: Record<TokenKind, string> = {
  key: 'text-code-key',
  number: 'text-code-number',
  plain: 'text-foreground',
  punct: 'text-code-punct',
  string: 'text-code-string'
}

interface CodeBlockProps {
  /** A value to pretty-print as JSON, or a preformatted string. */
  value: unknown
  label?: React.ReactNode
  /** The text that the copy button copies; defaults to the rendered source. */
  copyValue?: string
  className?: string
  maxHeight?: number
}

export function CodeBlock({ value, label, copyValue, className, maxHeight }: CodeBlockProps) {
  const source = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  const lines = useMemo(() => (typeof value === 'string' && !source.trimStart().startsWith('{') ? null : tokenizeJson(source)), [source, value])
  return (
    <div className={cn('flex min-w-0 flex-col overflow-hidden rounded-md border border-border bg-surface-sunken', className)}>
      <div className="flex h-9 items-center justify-between gap-2 border-b border-border pr-1.5 pl-3">
        <span className="truncate font-mono text-[11px] text-muted-foreground">{label}</span>
        <CopyButton value={copyValue ?? source} label="Copy" size="icon-sm" />
      </div>
      <pre className="overflow-auto px-4 py-3 font-mono text-caption leading-[1.6]" style={maxHeight ? { maxHeight } : undefined}>
        {lines
          ? lines.map((tokens, lineIndex) => (
              // Lines have no identity beyond their position.
              // oxlint-disable-next-line react/no-array-index-key
              <div key={lineIndex}>
                {tokens.map((token, tokenIndex) => (
                  // oxlint-disable-next-line react/no-array-index-key
                  <span key={tokenIndex} className={TOKEN_CLASS[token.kind]}>
                    {token.text}
                  </span>
                ))}
                {tokens.length === 0 ? '\n' : null}
              </div>
            ))
          : source}
      </pre>
    </div>
  )
}
