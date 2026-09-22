import { Download, Network, Upload } from 'lucide-react'
import { Link } from 'react-router-dom'
import { assetTypeMeta, sortTypes } from '../api/assetTypes'
import { useRoots, useStats } from '../api/queries'
import type { StatsResponse } from '../api/types'
import { TEMPLATE_URL } from './ImportPanel'
import { StatusDonut } from './StatusDonut'
import { StatusDot, TypeBadge } from './ui/Badges'
import { EmptyState } from './ui/EmptyState'
import { ErrorState } from './ui/ErrorState'
import { Skeleton, SkeletonRegion } from './ui/Skeleton'

const linkBtn =
  'inline-flex items-center gap-2 rounded-md border border-border-strong bg-surface px-3 py-2 text-sm font-medium text-text no-underline hover:bg-surface-muted'

function TypeCards({ stats }: { stats: StatsResponse }) {
  const types = sortTypes(stats.by_type, (t) => t.asset_type)
  return (
    <ul aria-label="Assets by type" className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
      <li className="rounded-lg border border-primary/30 bg-primary-soft px-4 py-3">
        <div className="text-2xl font-bold text-primary tabular-nums">{stats.total}</div>
        <div className="text-sm text-primary">Total assets</div>
      </li>
      {types.map((t) => {
        const meta = assetTypeMeta(t.asset_type)
        const Icon = meta.icon
        return (
          <li key={t.asset_type} className="rounded-lg border border-border bg-surface px-4 py-3">
            <div className="flex items-center justify-between">
              <span className="text-2xl font-bold tabular-nums">{t.count}</span>
              <Icon aria-hidden className="size-5 text-text-muted" />
            </div>
            <div className="text-sm text-text-muted">{meta.plural}</div>
          </li>
        )
      })}
    </ul>
  )
}

function Substations() {
  const roots = useRoots()
  return (
    <section aria-labelledby="subs-title">
      <h2 id="subs-title" className="mb-3 text-xs font-semibold tracking-wider text-text-muted uppercase">
        Top-level assets
      </h2>
      {roots.isPending && (
        <SkeletonRegion label="substations" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </SkeletonRegion>
      )}
      {roots.isError && (
        <ErrorState
          title="Could not load the top-level assets"
          error={roots.error}
          onRetry={() => void roots.refetch()}
          className="rounded-lg border border-border"
        />
      )}
      {roots.isSuccess && (
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
          {roots.data.roots.map((r) => (
            <li key={r.asset_id}>
              <Link
                to={`/assets/${encodeURIComponent(r.asset_id)}`}
                className="flex h-full flex-col rounded-lg border border-border bg-surface p-4 text-text no-underline hover:border-primary hover:bg-surface-muted"
              >
                <div className="flex items-center justify-between gap-2">
                  <TypeBadge type={r.asset_type} compact />
                  <span className="inline-flex items-center gap-2">
                    <StatusDot status={r.operational_status} />
                  </span>
                </div>
                <div className="mt-2 mb-1 line-clamp-2 min-h-[2lh] font-semibold" title={r.asset_name}>
                  {r.asset_name}
                </div>
                <div className="mt-auto flex flex-wrap items-baseline justify-between gap-x-2 text-sm text-text-muted">
                  <span>{r.asset_id}</span>
                  <span className="tabular-nums">{r.subtree_count - 1} assets beneath</span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function Onboarding() {
  const steps = [
    ['Download the template', 'It lists every column and shows a small example hierarchy.'],
    ['Fill it in', 'Required: asset_id, asset_type, asset_name, operational_status. Children point at their parent with parent_asset_id. Dates: YYYY-MM-DD or D/M/YY.'],
    ['Check, review, import', 'Open Import, choose the file, press Check file and read the result. Nothing is saved until you confirm.'],
  ]
  return (
    <section className="rounded-lg border border-border bg-surface p-6">
      <EmptyState icon={Network} title="No assets yet" description="Import a CSV file to build the asset hierarchy." className="pb-6" />
      <ol className="grid gap-4 sm:grid-cols-3">
        {steps.map(([title, text], i) => (
          <li key={title} className="rounded-lg bg-surface-muted p-4">
            <div className="mb-2 grid size-7 place-items-center rounded-full bg-primary text-sm font-bold text-on-primary">
              {i + 1}
            </div>
            <div className="font-semibold">{title}</div>
            <p className="mt-1 text-sm text-text-muted">{text}</p>
          </li>
        ))}
      </ol>
      <div className="mt-6 flex flex-wrap justify-center gap-2">
        <Link to="/import" className="inline-flex items-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-semibold text-on-primary no-underline hover:bg-primary-hover">
          <Upload aria-hidden className="size-4" />
          Go to Import
        </Link>
        <a href={TEMPLATE_URL} download className={linkBtn}>
          <Download aria-hidden className="size-4" />
          Download template
        </a>
      </div>
    </section>
  )
}

function Overview() {
  const stats = useStats()
  if (stats.isPending) {
    return (
      <SkeletonRegion label="overview" className="space-y-4">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </div>
        <Skeleton className="h-40" />
      </SkeletonRegion>
    )
  }
  if (stats.isError) {
    return (
      <ErrorState
        title="Could not load the overview"
        error={stats.error}
        onRetry={() => void stats.refetch()}
        className="rounded-lg border border-border"
      />
    )
  }
  if (stats.data.total === 0) return <Onboarding />
  return (
    <div className="space-y-6">
      <TypeCards stats={stats.data} />
      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <Substations />
        {/* stacked (a phone), the summary comes before 13+ cards rather than after them */}
        <StatusDonut stats={stats.data} className="order-first xl:order-none" />
      </div>
    </div>
  )
}

/** Main panel at `/`: a summary of everything stored. Navigation lives in the header, not here. */
export function HomePanel() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-bold">Overview</h1>
        <p className="mt-1 text-sm text-text-muted">What is stored, and where to go next. Select an asset in the tree or search to open it.</p>
      </header>
      <Overview />
    </div>
  )
}
