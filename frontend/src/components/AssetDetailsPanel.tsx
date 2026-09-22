import { SearchX, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { assetTypeMeta, sortTypes } from '../api/assetTypes'
import { ApiError, errorMessage } from '../api/client'
import { useAsset, useChildren, useDeleteAsset, useLookups } from '../api/queries'
import type { AssetDetail, ChildrenResponse, ParentRule } from '../api/types'
import { CHILD_SORT_KEYS, descendantTypes, sortChildren, type ChildSortKey, type SortDir } from '../lib'
import { StatusBadge, TypeBadge } from './ui/Badges'
import { SortHeader } from './ui/SortHeader'
import { ConfirmDialog } from './ui/ConfirmDialog'
import { EmptyState } from './ui/EmptyState'
import { ErrorState } from './ui/ErrorState'
import { Skeleton, SkeletonRegion } from './ui/Skeleton'

const NONE = '—'

function formatDate(iso: string | null): string {
  if (!iso) return NONE
  const [y, m, d] = iso.split('-').map(Number)
  if (!y || !m || !d) return iso
  return new Date(y, m - 1, d).toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-semibold tracking-wider text-text-muted uppercase">{label}</dt>
      <dd className="mt-1 text-sm break-words text-text">{children}</dd>
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="mb-2 text-xs font-semibold tracking-wider text-text-muted uppercase">
      {children}
    </h2>
  )
}

/** Delete this asset. Refused (and explained) while it still has children; the API enforces the same rule. */
function DeleteAssetButton({ asset }: { asset: AssetDetail }) {
  const children = useChildren(asset.asset_id)
  const del = useDeleteAsset()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const count = children.data?.total_children ?? 0

  return (
    <div className="flex flex-col items-start gap-1 sm:items-end">
      <button
        type="button"
        onClick={() => setOpen(true)}
        disabled={children.isPending || count > 0}
        className="inline-flex items-center gap-2 rounded-md border border-danger/40 bg-surface px-3 py-2 text-sm font-medium text-danger hover:bg-danger-soft disabled:cursor-not-allowed disabled:opacity-50"
      >
        <Trash2 aria-hidden className="size-4" />
        Delete asset
      </button>
      {count > 0 && (
        <p className="text-xs text-text-muted">
          Delete its {count} child {count === 1 ? 'asset' : 'assets'} first.
        </p>
      )}
      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!next) del.reset()
          setOpen(next)
        }}
        title="Delete this asset?"
        description={
          <>
            <strong>{asset.asset_name}</strong> ({asset.asset_id}) will be permanently deleted. This cannot be undone.
          </>
        }
        confirmLabel="Delete asset"
        pending={del.isPending}
        error={del.isError ? errorMessage(del.error) : null}
        onConfirm={() =>
          del.mutate(asset.asset_id, {
            onSuccess: () => {
              setOpen(false)
              void navigate(asset.parent_asset_id ? `/assets/${encodeURIComponent(asset.parent_asset_id)}` : '/assets')
            },
          })
        }
      />
    </div>
  )
}

function Header({ asset }: { asset: AssetDetail }) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <TypeBadge type={asset.asset_type} />
          <StatusBadge status={asset.operational_status} />
        </div>
        <h1 className="mt-2 text-3xl font-bold break-words">{asset.asset_name}</h1>
        <p className="mt-1 text-sm text-text-muted">{asset.asset_id}</p>
      </div>
      <DeleteAssetButton asset={asset} />
    </header>
  )
}

function Fields({ asset }: { asset: AssetDetail }) {
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-4 rounded-lg border border-border bg-surface-muted p-6 sm:grid-cols-2 lg:grid-cols-4">
      <Field label="Voltage">{asset.voltage_kv === null ? NONE : `${asset.voltage_kv} kV`}</Field>
      <Field label="Rating">
        {asset.rating_kva === null ? NONE : `${asset.rating_kva.toLocaleString()} kVA`}
      </Field>
      <Field label="Commissioned">{formatDate(asset.commissioned_date)}</Field>
      <Field label="Manufacturer">{asset.manufacturer ?? NONE}</Field>
      <Field label="Model">{asset.model ?? NONE}</Field>
      <Field label="Serial number">{asset.serial_number ?? NONE}</Field>
      <Field label="Parent">
        {asset.parent_asset_id === null ? (
          `${NONE} (root)`
        ) : (
          <Link to={`/assets/${encodeURIComponent(asset.parent_asset_id)}`} className="text-primary underline">
            {asset.parent_asset_id}
          </Link>
        )}
      </Field>
    </dl>
  )
}

const DEFAULT_SORT: ChildSortKey = 'type'

const COLUMNS: { key: ChildSortKey; label: string; align?: 'right' }[] = [
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'rating', label: 'Rating', align: 'right' },
  { key: 'commissioned', label: 'Commissioned' },
]

/**
 * One card per asset type that can occur beneath this asset (brief 4.3: for a substation, its
 * transformers, LV boards, switchboards and switchboard panels). The big number is how many sit
 * anywhere beneath it, at every depth (`descendant_counts`); the line under it says how many of those
 * are immediate children. A type with none shows 0. The types come from the parent rules, not from
 * code; a type the API returned but the rules do not mention is still shown.
 */
function ContentsCards({ data, assetType, rules }: { data: ChildrenResponse; assetType: string; rules: readonly ParentRule[] }) {
  const total = useMemo(() => new Map(data.descendant_counts.map((c) => [c.asset_type, c.count])), [data])
  const direct = useMemo(() => new Map(data.groups.map((g) => [g.asset_type, g.count])), [data])
  const types = useMemo(
    () => sortTypes([...new Set([...descendantTypes(assetType, rules), ...total.keys()])], (t) => t),
    [assetType, rules, total],
  )
  return (
    <ul aria-label="Contents by type" className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {types.map((t) => {
        const { plural, icon: Icon } = assetTypeMeta(t)
        const n = total.get(t) ?? 0
        const here = direct.get(t) ?? 0
        return (
          <li key={t} className="rounded-lg border border-border px-4 py-3">
            <div className={`text-2xl font-bold tabular-nums ${n === 0 ? 'text-text-muted' : ''}`}>{n}</div>
            <div className="flex items-center gap-1 text-sm text-text-muted">
              <Icon aria-hidden className="size-4 shrink-0" />
              {plural}
            </div>
            <div className="mt-1 text-xs text-text-muted">
              {n === 0 ? 'None' : `${here} direct${n > here ? ` · ${n - here} deeper` : ''}`}
            </div>
          </li>
        )
      })}
    </ul>
  )
}

function ChildrenSection({ data, assetType, rules }: { data: ChildrenResponse; assetType: string; rules: readonly ParentRule[] }) {
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort')
  const sort = CHILD_SORT_KEYS.find((k) => k === sortParam) ?? DEFAULT_SORT
  const dir: SortDir = params.get('dir') === 'desc' ? 'desc' : 'asc'
  const rows = useMemo(() => sortChildren(data.groups.flatMap((g) => g.assets), sort, dir), [data, sort, dir])

  // Same column again flips the direction; a new column starts ascending.
  const onSort = (key: ChildSortKey) =>
    setParams(
      (cur) => {
        const next = new URLSearchParams(cur)
        next.set('sort', key)
        next.set('dir', key === sort && dir === 'asc' ? 'desc' : 'asc')
        return next
      },
      { replace: true },
    )

  if (data.groups.length === 0) {
    return (
      <>
        <SectionTitle>Immediate children</SectionTitle>
        <p className="rounded-lg border border-border px-4 py-6 text-center text-sm text-text-muted">
          This asset has no child assets.
        </p>
      </>
    )
  }
  return (
    <>
      <SectionTitle>Contents by type, at every level</SectionTitle>
      <ContentsCards data={data} assetType={assetType} rules={rules} />

      <div className="mt-6">
        <SectionTitle>Immediate children</SectionTitle>
      </div>
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full min-w-[40rem] border-collapse text-left text-sm">
          <caption className="sr-only">
            Immediate children, sorted by {COLUMNS.find((c) => c.key === sort)?.label.toLowerCase()} ({dir === 'asc' ? 'ascending' : 'descending'})
          </caption>
          <thead className="bg-surface-muted text-xs tracking-wider text-text-muted">
            <tr>
              <th scope="col" className="px-4 py-3 font-semibold uppercase">Asset ID</th>
              {COLUMNS.map((c) => (
                <SortHeader key={c.key} label={c.label} align={c.align} active={c.key === sort} dir={dir} onSort={() => onSort(c.key)} />
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rows.map((a) => (
              <tr key={a.asset_id} className="hover:bg-surface-muted">
                <td className="px-4 py-3 whitespace-nowrap text-text-muted">{a.asset_id}</td>
                <td className="px-4 py-3 font-medium">
                  <Link to={`/assets/${encodeURIComponent(a.asset_id)}`} className="text-text no-underline hover:underline">
                    {a.asset_name}
                  </Link>
                </td>
                <td className="px-4 py-2">
                  <TypeBadge type={a.asset_type} />
                </td>
                <td className="px-4 py-2">
                  <StatusBadge status={a.operational_status} />
                </td>
                <td className="px-4 py-3 text-right whitespace-nowrap tabular-nums">{a.rating_kva == null ? NONE : `${a.rating_kva} kVA`}</td>
                <td className="px-4 py-3 whitespace-nowrap">{formatDate(a.commissioned_date)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}

const NO_RULES: ParentRule[] = []

function ChildrenPanel({ asset }: { asset: AssetDetail }) {
  const children = useChildren(asset.asset_id)
  // The parent rules say which types can sit beneath an asset; without them the cards still show what the API returned.
  const rules = useLookups().data?.parent_rules ?? NO_RULES
  return (
    <section aria-labelledby="children-title">
      <h2 id="children-title" className="sr-only">
        Contents and immediate children
      </h2>
      {children.isPending && (
        <>
          <SectionTitle>Immediate children</SectionTitle>
          <SkeletonRegion label="children" className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </SkeletonRegion>
        </>
      )}
      {children.isError && (
        <>
          <SectionTitle>Immediate children</SectionTitle>
          <ErrorState
            title="Could not load children"
            error={children.error}
            onRetry={() => void children.refetch()}
            className="rounded-lg border border-border"
          />
        </>
      )}
      {children.isSuccess && <ChildrenSection data={children.data} assetType={asset.asset_type} rules={rules} />}
    </section>
  )
}

/** Main panel at `/assets/:assetId`. Keyed on the route param, so back/forward and refresh just work. */
export function AssetDetailsPanel() {
  const { assetId = '' } = useParams()
  const asset = useAsset(assetId)

  if (asset.isPending) {
    return (
      <SkeletonRegion label="asset details">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="mt-3 h-9 w-2/3" />
        <Skeleton className="mt-6 h-40" />
      </SkeletonRegion>
    )
  }
  if (asset.isError) {
    if (asset.error instanceof ApiError && asset.error.status === 404) {
      return (
        <EmptyState
          icon={SearchX}
          title="Asset not found"
          description={`There is no asset with ID "${assetId}".`}
        >
          <Link to="/" className="text-sm font-medium text-primary underline">
            Back to the explorer
          </Link>
        </EmptyState>
      )
    }
    return (
      <ErrorState
        title="Could not load this asset"
        error={asset.error}
        onRetry={() => void asset.refetch()}
      />
    )
  }

  return (
    <article>
      <Header asset={asset.data} />
      <div className="mt-6 space-y-6">
        <Fields asset={asset.data} />
        <ChildrenPanel asset={asset.data} />
      </div>
    </article>
  )
}
