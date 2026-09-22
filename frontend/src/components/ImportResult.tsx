import { CircleCheck, TriangleAlert, X } from 'lucide-react'
import type { ImportResponse } from '../api/types'
import { ImportStat } from './ImportStat'
import { DownloadReportButton, RejectionTable } from './RejectionTable'

/**
 * The final report of one import (brief 4.2): a yes/no committed line, total /
 * imported / rejected counts, and every rejected row with its reason. Used
 * right after confirming, on the Import page.
 */
export function ImportResult({
  fileName,
  result,
  onDismiss,
}: {
  fileName: string
  result: ImportResponse
  onDismiss?: () => void
}) {
  const { committed, rejections, ignored_columns: ignored } = result
  const partial = committed && result.rejected_rows > 0

  const headline = !committed
    ? 'Data committed: no — nothing was saved'
    : partial
      ? `Data committed: yes — ${result.imported_rows} rows stored, ${result.rejected_rows} rejected rows were not saved`
      : `Data committed: yes — all ${result.imported_rows} rows imported`

  return (
    <section aria-labelledby="import-result-title" className="rounded-lg border border-border bg-surface p-4 sm:p-6">
      <div className="flex items-start justify-between gap-3">
        <p
          className={`inline-flex items-center gap-2 text-sm font-semibold ${
            !committed ? 'text-danger' : partial ? 'text-warning' : 'text-success'
          }`}
        >
          {committed && !partial ? (
            <CircleCheck aria-hidden className="size-4" />
          ) : (
            <TriangleAlert aria-hidden className="size-4" />
          )}
          {headline}
        </p>
        {onDismiss && (
          <button
            type="button"
            onClick={onDismiss}
            aria-label="Dismiss import report"
            className="rounded-md p-1 text-text-muted hover:bg-neutral-soft"
          >
            <X aria-hidden className="size-5" />
          </button>
        )}
      </div>
      <h2 id="import-result-title" className="mt-1 text-2xl font-bold break-all">
        {fileName}
      </h2>

      <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-3">
        <ImportStat label="Total rows" value={result.total_rows} tone="border-border bg-surface" />
        <ImportStat label="Imported" value={result.imported_rows} tone="border-success/25 bg-success-soft text-success" />
        <ImportStat
          label="Rejected"
          value={result.rejected_rows}
          tone={result.rejected_rows > 0 ? 'border-danger/25 bg-danger-soft text-danger' : 'border-border bg-surface'}
        />
      </div>

      {ignored.length > 0 && (
        <p className="mt-4 text-sm text-text-muted">
          Ignored unrecognised columns: <span className="font-medium">{ignored.join(', ')}</span>
        </p>
      )}

      {rejections.length > 0 && (
        <div className="mt-6">
          <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-xs font-semibold tracking-wider text-text-muted uppercase">Rejected rows</h2>
            <DownloadReportButton fileName={fileName} rejections={rejections} />
          </div>
          <RejectionTable fileName={fileName} rejections={rejections} />
        </div>
      )}
    </section>
  )
}
