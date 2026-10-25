import { useSyncExternalStore } from 'react'

import { type TimeZoneMode } from './time'

/**
 * Console preferences, stored in this browser only (docs/console.md, Preferences). The inline
 * script in index.html reads the same key to apply the theme and density before first paint.
 */
export interface Preferences {
  theme: 'system' | 'light' | 'dark'
  density: 'comfortable' | 'compact'
  amounts: 'major' | 'raw'
  timeZone: TimeZoneMode
}

export const PREFERENCES_KEY = 'tbp.preferences.v1'

export const DEFAULT_PREFERENCES: Preferences = { amounts: 'major', density: 'comfortable', theme: 'system', timeZone: 'local' }

const listeners = new Set<() => void>()
let current = read()

function read(): Preferences {
  try {
    const raw = globalThis.localStorage?.getItem(PREFERENCES_KEY)
    if (!raw) {
      return DEFAULT_PREFERENCES
    }
    const parsed = JSON.parse(raw) as Partial<Preferences>
    return { ...DEFAULT_PREFERENCES, ...parsed }
  } catch {
    return DEFAULT_PREFERENCES
  }
}

function systemPrefersDark(): boolean {
  return globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false
}

/** Applies the theme class and density attribute to <html>. */
export function applyPreferences(prefs: Preferences): void {
  const root = globalThis.document?.documentElement
  if (!root) {
    return
  }
  const dark = prefs.theme === 'dark' || (prefs.theme === 'system' && systemPrefersDark())
  root.classList.toggle('dark', dark)
  root.dataset.density = prefs.density
}

export function getPreferences(): Preferences {
  return current
}

export function setPreferences(update: Partial<Preferences>): void {
  current = { ...current, ...update }
  try {
    globalThis.localStorage?.setItem(PREFERENCES_KEY, JSON.stringify(current))
  } catch {
    // Private mode or a full quota: the preference still applies to this tab.
  }
  applyPreferences(current)
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function usePreferences(): Preferences {
  return useSyncExternalStore(subscribe, getPreferences, () => DEFAULT_PREFERENCES)
}

/** Keeps the "system" theme in sync when the OS switches between light and dark. */
export function watchSystemTheme(): () => void {
  const media = globalThis.matchMedia?.('(prefers-color-scheme: dark)')
  if (!media) {
    return () => {}
  }
  const onChange = () => {
    if (current.theme === 'system') {
      applyPreferences(current)
    }
  }
  media.addEventListener('change', onChange)
  return () => media.removeEventListener('change', onChange)
}
