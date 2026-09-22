import { Moon, Sun } from 'lucide-react'
import { useTheme } from '../state/theme'

/** Icon-only light/dark switch for the top bar. */
export function ThemeToggle() {
  const { theme, toggle } = useTheme()
  const dark = theme === 'dark'
  const Icon = dark ? Sun : Moon
  return (
    <button
      type="button"
      onClick={toggle}
      aria-pressed={dark}
      aria-label="Dark mode"
      title={dark ? 'Switch to light mode' : 'Switch to dark mode'}
      className="rounded-md border border-border-strong bg-surface p-2 text-text hover:bg-surface-muted"
    >
      <Icon aria-hidden className="size-4" />
    </button>
  )
}
