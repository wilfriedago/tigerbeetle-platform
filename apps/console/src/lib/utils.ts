import { createCn } from 'cn/config'

/**
 * Joins class names and resolves Tailwind conflicts (the last class of a group wins).
 *
 * The merge engine only knows the default Tailwind scales, so the named values from
 * styles/globals.css are registered here. Without this, `text-small` would be read as a text
 * colour and `cn('text-small', 'text-muted-foreground')` would drop the size.
 */
export const cn = createCn({
  extend: {
    theme: {
      shadow: ['float', 'overlay'],
      spacing: ['row', 'header-row'],
      text: ['display', 'page-title', 'section-title', 'card-title', 'body', 'small', 'caption', 'amount-display']
    }
  }
})
