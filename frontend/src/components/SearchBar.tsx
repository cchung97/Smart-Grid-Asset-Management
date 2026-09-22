import { useQueryClient } from '@tanstack/react-query'
import { LoaderCircle, Search } from 'lucide-react'
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { errorMessage } from '../api/client'
import { fetchAncestors, useSearch } from '../api/queries'
import type { AssetSummary } from '../api/types'
import { useDebounced } from '../hooks'
import { useExplorer } from '../state/explorerContext'
import { TypeBadge } from './ui/Badges'

export const SEARCH_DEBOUNCE_MS = 300
const MAX_QUERY = 200 // the API accepts 1–200 characters

/**
 * Type-ahead search (ARIA combobox + listbox). Picking a result runs the
 * search → select flow: fetch ancestors → expand them in the tree → navigate to
 * /assets/:id; the tree highlights and scrolls to the node from the route.
 */
export function SearchBar() {
  const listId = useId()
  const client = useQueryClient()
  const navigate = useNavigate()
  const { expandMany } = useExplorer()

  const [text, setText] = useState('')
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const [selecting, setSelecting] = useState(false)
  const [selectError, setSelectError] = useState<string | null>(null)
  const activeRef = useRef<HTMLLIElement>(null)

  const query = useDebounced(text.trim(), SEARCH_DEBOUNCE_MS)
  const waiting = text.trim() !== query // still inside the debounce window
  const search = useSearch(waiting ? '' : query)
  const results = search.data?.results ?? []
  const showPanel = open && text.trim() !== ''

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest' })
  }, [activeIndex])

  const optionId = (i: number) => `${listId}-opt-${i}`

  async function choose(asset: AssetSummary) {
    setSelecting(true)
    setSelectError(null)
    try {
      const { path } = await fetchAncestors(client, asset.asset_id)
      // path runs root → asset; everything above the asset must be open to reveal it.
      expandMany(path.slice(0, -1).map((a) => a.asset_id))
      void navigate(`/assets/${encodeURIComponent(asset.asset_id)}`)
      setText('')
      setOpen(false)
      setActiveIndex(-1)
    } catch (err) {
      setSelectError(errorMessage(err))
    } finally {
      setSelecting(false)
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    switch (e.key) {
      case 'ArrowDown':
        if (!showPanel) setOpen(true)
        else if (results.length) setActiveIndex((i) => (i + 1) % results.length)
        break
      case 'ArrowUp':
        if (results.length) setActiveIndex((i) => (i <= 0 ? results.length - 1 : i - 1))
        break
      case 'Enter': {
        const target = results[activeIndex] ?? (results.length === 1 ? results[0] : undefined)
        if (!showPanel || !target || selecting) return
        void choose(target)
        break
      }
      case 'Escape':
        if (!open) return
        setOpen(false)
        setActiveIndex(-1)
        break
      default:
        return
    }
    e.preventDefault()
  }

  let body: React.ReactNode
  if (selectError) {
    body = (
      <p role="alert" className="px-3 py-3 text-sm text-danger">
        Could not open that asset: {selectError}
      </p>
    )
  } else if (waiting || search.isFetching) {
    body = (
      <p role="status" className="flex items-center gap-2 px-3 py-3 text-sm text-text-muted">
        <LoaderCircle aria-hidden className="size-4 animate-spin" />
        Searching…
      </p>
    )
  } else if (search.isError) {
    body = (
      <div role="alert" className="px-3 py-3 text-sm">
        <p className="text-danger">Search failed: {errorMessage(search.error)}</p>
        <button
          type="button"
          onClick={() => void search.refetch()}
          className="mt-1 font-medium text-primary underline"
        >
          Try again
        </button>
      </div>
    )
  } else if (results.length === 0) {
    body = (
      <p role="status" className="px-3 py-3 text-sm text-text-muted">
        No assets match “{query}”.
      </p>
    )
  }

  return (
    <div role="search" className="relative">
      <Search
        aria-hidden
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-text-muted"
      />
      <input
        type="text"
        role="combobox"
        aria-label="Search assets by ID or name"
        aria-expanded={showPanel}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={showPanel && activeIndex >= 0 ? optionId(activeIndex) : undefined}
        autoComplete="off"
        placeholder="Search by asset ID or name…"
        maxLength={MAX_QUERY}
        value={text}
        onChange={(e) => {
          setText(e.target.value)
          setOpen(true)
          setActiveIndex(-1)
          setSelectError(null)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={onKeyDown}
        className="h-9 w-full rounded-md border border-border-strong bg-surface-muted pr-3 pl-10 text-sm text-text placeholder:text-text-muted"
      />

      <div
        id={listId}
        hidden={!showPanel}
        // Keep input focus when the pointer goes down on the panel.
        onMouseDown={(e) => e.preventDefault()}
        className="absolute inset-x-0 top-full z-40 mt-1 max-h-80 overflow-y-auto rounded-lg border border-border bg-surface shadow-lg"
      >
        {body ?? (
          <>
            <ul role="listbox" aria-label="Search results">
              {results.map((a, i) => (
                <li
                  key={a.asset_id}
                  id={optionId(i)}
                  ref={i === activeIndex ? activeRef : undefined}
                  role="option"
                  aria-selected={i === activeIndex}
                  onMouseMove={() => setActiveIndex(i)}
                  onClick={() => void choose(a)}
                  className={`flex cursor-pointer items-center gap-3 px-3 py-2 text-sm ${
                    i === activeIndex ? 'bg-primary-soft' : ''
                  }`}
                >
                  <TypeBadge type={a.asset_type} compact />
                  <span className="min-w-0 flex-1 truncate font-medium">{a.asset_name}</span>
                  <span className="shrink-0 text-xs text-text-muted">{a.asset_id}</span>
                </li>
              ))}
            </ul>
            {search.data?.truncated && (
              <p className="border-t border-border px-3 py-2 text-xs text-text-muted">
                Showing the first {results.length} matches — refine your search to narrow them.
              </p>
            )}
          </>
        )}
      </div>

      <p aria-live="polite" className="sr-only">
        {showPanel && !waiting && search.isSuccess ? `${results.length} results` : ''}
      </p>
    </div>
  )
}
