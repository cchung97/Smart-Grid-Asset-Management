import { ChevronDown, History } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { PAGE_SIZE, useImportDetail, useImportList } from '../api/queries'
import type { ImportSummary } from '../api/types'
import { formatDateTime } from '../lib'
import { useExplorer } from '../state/explorerContext'
import { RejectionTable } from './RejectionTable'
import { EmptyState } from './ui/EmptyState'
import { ErrorState } from './ui/ErrorState'
import { Pagination } from './ui/Pagination'
import { Skeleton, SkeletonRegion } from './ui/Skeleton'

function Outcome({ run }: { run: ImportSummary }) {
  const partial = run.committed && run.rejected_rows > 0
  const [label, cls] = !run.committed
    ? ['Nothing stored', 'bg-danger-soft text-danger']
    : partial
      ? ['Partly stored', 'bg-warning-soft text-warning']
      : ['All stored', 'bg-success-soft text-success']
  return <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold whitespace-nowrap ${cls}`}>{label}</span>
}

function RunDetail({ run }: { run: ImportSummary }) {
  const detail = useImportDetail(run.import_id, true)
  if (detail.isPending) {
    return (
      <SkeletonRegion label="import details">
        <Skeleton className="h-16" />
      </SkeletonRegion>
    )
  }
  if (detail.isError) {
    return <ErrorState title="Could not load this import" error={detail.error} onRetry={() => void detail.refetch()} className="py-4" />
  }
  const { rejections } = detail.data
  if (rejections.length === 0) return <p className="text-sm text-text-muted">No rows were rejected.</p>
  return (
    <RejectionTable fileName={run.filename} rejections={rejections} />
  )
}

/**
 * The history section of `/import`: one page of past imports, newest first.
 * Imports only — deletions are not recorded here. While the report of the import
 * just confirmed is on screen, its row is marked so it is easy to find.
 */
export function ImportHistory() {
  const [params, setParams] = useSearchParams()
  const page = Math.max(1, Number(params.get('page')) || 1)
  const list = useImportList(page)
  const { importFlow } = useExplorer()
  const justImported = importFlow.stage === 'done' ? importFlow.result.import_id : null
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set())

  const toggle = (id: string) =>
    setOpen((cur) => {
      const next = new Set(cur)
      if (!next.delete(id)) next.add(id)
      return next
    })

  return (
    <section aria-labelledby="import-history" className="space-y-4 rounded-lg border border-border bg-surface-muted p-4">
      <header>
        <h2 id="import-history" tabIndex={-1} className="text-xs font-semibold tracking-wider text-text-muted uppercase">
          Import history
        </h2>
        <p className="mt-1 text-sm text-text-muted">Imports only — deletions are not listed here. Newest first.</p>
      </header>

      {list.isPending && (
        <SkeletonRegion label="import activity" className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </SkeletonRegion>
      )}
      {list.isError && (
        <ErrorState title="Could not load import activity" error={list.error} onRetry={() => void list.refetch()} className="rounded-lg border border-border bg-surface" />
      )}
      {list.isSuccess && list.data.items.length === 0 && (
        <div className="rounded-lg border border-border bg-surface">
          <EmptyState icon={History} title="No imports yet" description="Once you import a CSV file it is listed here, with the rows that were rejected." />
        </div>
      )}
      {list.isSuccess && list.data.items.length > 0 && (
        <>
          <ul
            aria-label={`Past imports, page ${page}`}
            className={`divide-y divide-border overflow-hidden rounded-lg border border-border bg-surface ${list.isPlaceholderData ? 'opacity-60' : ''}`}
          >
            {list.data.items.map((run) => {
              const expanded = open.has(run.import_id)
              const fresh = run.import_id === justImported
              return (
                <li key={run.import_id}>
                  <div className={`space-y-1 p-4 ${fresh ? 'bg-primary-soft' : ''}`}>
                    <div className="flex items-start justify-between gap-3">
                      <button
                        type="button"
                        aria-expanded={expanded}
                        aria-label={`${run.filename}, ${expanded ? 'hide' : 'show'} details`}
                        onClick={() => toggle(run.import_id)}
                        className="inline-flex min-w-0 items-center gap-2 text-left text-sm font-medium text-primary underline"
                      >
                        <ChevronDown aria-hidden className={`size-4 shrink-0 transition-transform ${expanded ? '' : '-rotate-90'}`} />
                        <span className="truncate">{run.filename}</span>
                      </button>
                      <Outcome run={run} />
                    </div>
                    <p className="text-sm text-text">
                      {run.imported_rows} of {run.total_rows} rows imported
                      {run.rejected_rows > 0 && <span className="text-text-muted"> · {run.rejected_rows} rejected</span>}
                    </p>
                    <p className="text-xs text-text-muted">
                      {formatDateTime(run.created_at)}
                      {fresh && <span className="ml-2 font-semibold text-primary">Just imported</span>}
                    </p>
                  </div>
                  {expanded && (
                    <div className="space-y-3 border-t border-border p-4">
                      <p className="text-sm text-text-muted">
                        Uploaded from <span className="font-medium text-text">{run.client_ip}</span>
                      </p>
                      <RunDetail run={run} />
                    </div>
                  )}
                </li>
              )
            })}
          </ul>
          <Pagination
            page={page}
            pageSize={PAGE_SIZE}
            total={list.data.total}
            noun="imports"
            onPage={(p) => setParams(p === 1 ? {} : { page: String(p) }, { replace: true })}
          />
        </>
      )}
    </section>
  )
}
