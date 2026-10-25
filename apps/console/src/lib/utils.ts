import { createCn } from 'cn/config'
// oxlint-disable-next-line no-restricted-imports -- this module is the configured wrapper
import { createTV } from 'tailwind-variants'

export type { VariantProps } from 'tailwind-variants'

/**
 * The merge engines only know the default Tailwind scales, so the named values from
 * styles/globals.css are registered here. Without this, `text-small` would be read as a text
 * colour and `cn('text-small', 'text-muted-foreground')` would drop the size.
 */
const mergeConfig = {
  extend: {
    theme: {
      shadow: ['float', 'overlay'],
      spacing: ['row', 'header-row'],
      text: ['display', 'page-title', 'section-title', 'card-title', 'body', 'small', 'caption', 'amount-display']
    }
  }
}

/** Joins class names and resolves Tailwind conflicts (the last class of a group wins). */
export const cn = createCn(mergeConfig)

/**
 * Builds a variant recipe. tailwind-variants ships its own merge engine, separate from `cn`, so it
 * gets the same token config. Pass `className` to the recipe rather than wrapping it in `cn`.
 */
export const tv = createTV({ twMergeConfig: mergeConfig })
