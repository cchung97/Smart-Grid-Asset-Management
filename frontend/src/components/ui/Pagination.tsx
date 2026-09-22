import { ChevronLeft, ChevronRight } from 'lucide-react'

interface Props {
  page: number // 1-based
  pageSize: number
  total: number
  onPage: (page: number) => void
  noun?: string
}

/** "Showing 1–25 of 206" with Previous/Next; the counts come from the server. */
export function Pagination({ page, pageSize, total, onPage, noun = 'results' }: Props) {
  const first = total === 0 ? 0 : (page - 1) * pageSize + 1
  const last = Math.min(page * pageSize, total)
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const btn =
    'inline-flex items-center gap-1 rounded-md border border-border-strong bg-surface px-3 py-2 text-sm font-medium text-text hover:bg-surface-muted disabled:cursor-not-allowed disabled:opacity-50'
  return (
    <nav aria-label="Pagination" className="flex flex-wrap items-center justify-between gap-3 text-sm">
      <p className="text-text-muted" aria-live="polite">
        Showing {first}–{last} of {total} {noun}
      </p>
      <div className="flex items-center gap-2">
        <button type="button" className={btn} disabled={page <= 1} onClick={() => onPage(page - 1)}>
          <ChevronLeft aria-hidden className="size-4" />
          Previous
        </button>
        <span className="text-text-muted">
          Page {page} of {pages}
        </span>
        <button type="button" className={btn} disabled={page >= pages} onClick={() => onPage(page + 1)}>
          Next
          <ChevronRight aria-hidden className="size-4" />
        </button>
      </div>
    </nav>
  )
}
