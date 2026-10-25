/**
 * Keyboard glyphs for hints. They are built from code points because the repository is ASCII only
 * (AGENTS.md) and the formatter rewrites escapes such as "\\u2318" into the literal character.
 */
export const KEY_GLYPHS = {
  command: String.fromCodePoint(0x23_18),
  down: String.fromCodePoint(0x21_93),
  enter: String.fromCodePoint(0x21_b5),
  up: String.fromCodePoint(0x21_91)
} as const
