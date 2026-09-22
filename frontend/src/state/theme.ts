import { useSyncExternalStore } from 'react'

export type Theme = 'light' | 'dark'

const KEY = 'theme'
// The browser's address-bar colour: brand navy in light, the dark surface in dark.
const THEME_COLOR: Record<Theme, string> = { light: '#024d87', dark: '#0f1419' }
const listeners = new Set<() => void>()

/** The theme is whatever <html data-theme> says; index.html sets it before first paint. */
function current(): Theme {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function setTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', THEME_COLOR[theme])
  try {
    localStorage.setItem(KEY, theme)
  } catch {
    // Private mode or blocked storage: the choice still applies until the tab closes.
  }
  listeners.forEach((l) => l())
}

/** Light or dark, kept across visits in localStorage. Client state, so it lives in src/state/. */
export function useTheme() {
  const theme = useSyncExternalStore(subscribe, current, () => 'light' as Theme)
  return { theme, toggle: () => setTheme(theme === 'dark' ? 'light' : 'dark') }
}
