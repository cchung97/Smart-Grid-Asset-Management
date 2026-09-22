import { List, SearchX } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { assetTypeMeta, sortTypes, statusLabel } from '../api/assetTypes'
import { PAGE_SIZE, useAssetList, useLookups } from '../api/queries'
import type { AssetListQuery } from '../api/types'
import { useDebounced } from '../hooks'
import type { SortDir } from '../lib'
import { StatusBadge, TypeBadge } from './ui/Badges'
import { EmptyState } from './ui/EmptyState'
import { ErrorState } from './ui/ErrorState'
import { Pagination } from './ui/Pagination'
import { Select } from './ui/Select'
import { Skeleton, SkeletonRegion } from './ui/Skeleton'
import { SortHeader } from './ui/SortHeader'

const field =
  'h-9 rounded-md border border-border-strong bg-surface px-3 text-sm text-text'

type SortKey = NonNullable<AssetListQuery['sort']>

const COLUMNS: { key: SortKey; label: string; className?: string }[] = [
  { key: 'asset_id', label: 'Asset ID', className: 'w-64' },
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'parent', label: 'Parent' },
]

/** `/assets`: every stored asset, filtered and paged by the server. Filters live in the URL so refresh and back/forward keep them. */
export function AllAssetsPanel() {
  const [params, setParams] = useSearchParams()
  const type = params.get('type') ?? ''
  const status = params.get('status') ?? ''
  const q = params.get('q') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const sort = COLUMNS.find((c) => c.key === params.get('sort'))?.key ?? 'asset_id'
  const dir: SortDir = params.get('dir') === 'desc' ? 'desc' : 'asc'

  const lookups = useLookups()
  const [text, setText] = useState(q)
  const debounced = useDebounced(text.trim(), 300)

  const update = (next: Record<string, string>) => {
    const merged = new URLSearchParams(params)
    for (const [k, v] of Object.entries(next)) {
      if (v) merged.set(k, v)
      else merged.delete(k)
    }
    setParams(merged, { replace: true })
  }

  // Push the debounced text into the URL (back to page 1) — but only when it differs.
  useEffect(() => {
    if (debounced !== q) update({ q: debounced, page: '' })
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only the debounced text should trigger this
  }, [debounced])

  // The server sorts (the list is paged, so sorting only this page would mislead). Same column again
  // flips the direction; a new column starts ascending. Either way, back to page 1.
  const onSort = (key: SortKey) =>
    update({ sort: key === 'asset_id' ? '' : key, dir: key === sort && dir === 'asc' ? 'desc' : '', page: '' })

  const list = useAssetList({ q, type, status, sort, dir, page })
  const total = list.data?.total ?? 0
  const filtered = !!(q || type || status)

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">All assets</h1>
          <p className="mt-1 text-sm text-text-muted">Every stored asset. Click a column heading to sort.</p>
        </div>
      </header>

      <form role="search" aria-label="Filter assets" onSubmit={(e) => e.preventDefault()} className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
        <input
          type="search"
          aria-label="Filter by asset ID or name"
          placeholder="Filter by asset ID or name…"
          value={text}
          maxLength={200}
          onChange={(e) => setText(e.target.value)}
          className={field}
        />
        <Select aria-label="Filter by type" value={type} onChange={(e) => update({ type: e.target.value, page: '' })}>
          <option value="">All types</option>
          {sortTypes(lookups.data?.asset_types ?? [], (t) => t).map((t) => (
            <option key={t} value={t}>
              {assetTypeMeta(t).label}
            </option>
          ))}
        </Select>
        <Select aria-label="Filter by status" value={status} onChange={(e) => update({ status: e.target.value, page: '' })}>
          <option value="">All statuses</option>
          {(lookups.data?.operational_statuses ?? []).map((s) => (
            <option key={s} value={s}>
              {statusLabel(s)}
            </option>
          ))}
        </Select>
      </form>

      {list.isPending && (
        <SkeletonRegion label="assets" className="space-y-2">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} className="h-10" />
          ))}
        </SkeletonRegion>
      )}
      {list.isError && (
        <ErrorState title="Could not load assets" error={list.error} onRetry={() => void list.refetch()} className="rounded-lg border border-border" />
      )}
      {list.isSuccess && list.data.items.length === 0 && (
        <div className="rounded-lg border border-border">
          <EmptyState
            icon={filtered ? SearchX : List}
            title={filtered ? 'No assets match these filters' : 'No assets yet'}
            description={filtered ? 'Try a different search or clear the filters.' : 'Import a CSV file to add assets.'}
          />
        </div>
      )}
      {list.isSuccess && list.data.items.length > 0 && (
        <>
          <div className={`overflow-x-auto rounded-lg border border-border ${list.isPlaceholderData ? 'opacity-60' : ''}`}>
            <table className="w-full min-w-[46rem] border-collapse text-left text-sm">
              <caption className="sr-only">Assets, page {page}</caption>
              <thead className="bg-surface-muted text-xs tracking-wider text-text-muted uppercase">
                <tr>
                  {COLUMNS.map((c) => (
                    <SortHeader key={c.key} label={c.label} className={c.className} active={c.key === sort} dir={dir} onSort={() => onSort(c.key)} />
                  ))}
                </tr>
              </thead>
              <tbody>
                {list.data.items.map((a) => (
                  <tr key={a.asset_id} className="border-t border-border hover:bg-surface-muted">
                    <td className="px-4 py-3 font-medium whitespace-nowrap">
                      <Link to={`/assets/${encodeURIComponent(a.asset_id)}`} className="text-primary underline">
                        {a.asset_id}
                      </Link>
                    </td>
                    <td className="px-4 py-3">{a.asset_name}</td>
                    <td className="px-4 py-3">
                      <TypeBadge type={a.asset_type} />
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={a.operational_status} />
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-text-muted">
                      {a.parent_asset_id ? (
                        <Link to={`/assets/${encodeURIComponent(a.parent_asset_id)}`} className="text-primary underline">
                          {a.parent_asset_id}
                        </Link>
                      ) : (
                        '—'
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={page} pageSize={PAGE_SIZE} total={total} noun="assets" onPage={(p) => update({ page: p === 1 ? '' : String(p) })} />
        </>
      )}
    </div>
  )
}
