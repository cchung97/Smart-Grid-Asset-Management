import { BookOpen, HeartPulse } from 'lucide-react'
import { useNow } from '../hooks'
import { formatClock } from '../lib'

const LINK =
  'inline-flex items-center gap-2 rounded-md px-2 py-1 text-text-muted no-underline hover:bg-neutral-soft hover:text-text'

/** App name, version and copyright. It sits at the bottom of the asset-tree rail (or of the drawer). */
export function AppVersion() {
  return (
    <p className="m-0 text-xs text-text-muted">
      Grid Asset Explorer <span className="font-semibold text-text">v{__APP_VERSION__}</span>
      <span className="block">© {new Date().getFullYear()} Nicholas Ong. All rights reserved.</span>
    </p>
  )
}

/**
 * The last thing in the scrolling main panel, not a bar pinned to the screen: links to the API docs and
 * the health check, and a live clock, at the bottom right. Pages without a tree rail have nowhere else
 * to show the version, so they pass `withVersion` and get it on the left.
 */
export function AppFooter({ withVersion = false }: { withVersion?: boolean }) {
  const now = useNow(1000)

  return (
    <footer className="flex flex-wrap items-center justify-end gap-x-6 gap-y-2 border-t border-border px-4 py-3 text-xs text-text-muted sm:px-6 lg:px-8">
      {withVersion && (
        <div className="mr-auto">
          <AppVersion />
        </div>
      )}
      <nav aria-label="Footer" className="flex items-center gap-2">
        <a href="/swagger/index.html" target="_blank" rel="noreferrer" className={LINK}>
          <BookOpen aria-hidden className="size-4" />
          API docs
        </a>
        <a href="/healthz" target="_blank" rel="noreferrer" className={LINK}>
          <HeartPulse aria-hidden className="size-4" />
          Health
        </a>
      </nav>
      <time dateTime={now.toISOString()} className="tabular-nums">
        {formatClock(now)}
      </time>
    </footer>
  )
}
