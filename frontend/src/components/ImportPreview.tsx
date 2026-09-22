import { CircleAlert, LoaderCircle, ShieldCheck } from 'lucide-react'
import { useImportFlow } from '../state/useImportFlow'
import type { ImportFlow } from '../state/explorerContext'
import { DownloadReportButton, RejectionTable } from './RejectionTable'
import { ImportStat } from './ImportStat'

type Reviewing = Extract<ImportFlow, { stage: 'preview' | 'importing' }>

/**
 * Step 2 of the import: what checking the file found, before anything is
 * stored. The user either imports the rows that can be imported (rejected rows
 * are skipped) or cancels, fixes the file and checks it again.
 */
export function ImportPreview({ flow }: { flow: Reviewing }) {
  const { confirm, check, reset } = useImportFlow()
  const { file, preview } = flow
  const importing = flow.stage === 'importing'
  const conflict = flow.stage === 'preview' && flow.conflict
  const notice = flow.stage === 'preview' ? flow.notice : undefined
  const error = flow.stage === 'preview' ? flow.error : undefined
  const n = preview.importable_rows
  const rejected = preview.rejected_rows

  return (
    <section aria-labelledby="import-review-title" className="rounded-lg border border-border bg-surface p-4 sm:p-6">
      <p className="inline-flex items-center gap-2 text-sm font-semibold text-primary">
        <ShieldCheck aria-hidden className="size-4" />
        Nothing has been saved yet
      </p>
      <h2 id="import-review-title" className="mt-1 text-2xl font-bold break-all">
        Review {file.name}
      </h2>
      <p className="mt-1 text-sm text-text-muted">
        {rejected === 0
          ? 'Every row passed the checks.'
          : 'Rows that failed a check are skipped, along with any row under a skipped parent. Import the rest, or cancel and fix the file.'}
      </p>

      {notice && (
        <p role="status" className="mt-4 rounded-md bg-warning-soft px-3 py-2 text-sm text-warning">
          {notice}
        </p>
      )}

      <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-3">
        <ImportStat label="Total rows" value={preview.total_rows} tone="border-border bg-surface" />
        <ImportStat label="Can be imported" value={n} tone="border-success/25 bg-success-soft text-success" />
        <ImportStat
          label="Will be rejected"
          value={rejected}
          tone={rejected > 0 ? 'border-danger/25 bg-danger-soft text-danger' : 'border-border bg-surface'}
        />
      </div>

      {preview.ignored_columns.length > 0 && (
        <p className="mt-4 text-sm text-text-muted">
          Ignored unrecognised columns: <span className="font-medium">{preview.ignored_columns.join(', ')}</span>
        </p>
      )}

      {error && (
        <div role="alert" className="mt-4 flex items-start gap-2 rounded-md bg-danger-soft px-3 py-2 text-sm text-danger">
          <CircleAlert aria-hidden className="mt-1 size-4 shrink-0" />
          {error}
        </div>
      )}

      <div className="mt-6 flex flex-wrap items-center gap-3">
        {conflict ? (
          <button
            type="button"
            onClick={() => void check(file)}
            className="rounded-md bg-primary px-4 py-2 text-sm font-semibold text-on-primary hover:bg-primary-hover"
          >
            Check again
          </button>
        ) : (
          <button
            type="button"
            onClick={() => void confirm()}
            disabled={importing || n === 0}
            className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-semibold text-on-primary hover:bg-primary-hover disabled:cursor-not-allowed disabled:opacity-60"
          >
            {importing && <LoaderCircle aria-hidden className="size-4 animate-spin" />}
            {importing ? 'Importing…' : rejected === 0 ? `Import all ${n} rows` : `Import ${n} valid rows`}
          </button>
        )}
        <button
          type="button"
          onClick={reset}
          disabled={importing}
          className="rounded-md border border-border-strong bg-surface px-4 py-2 text-sm font-medium text-text hover:bg-surface-muted disabled:opacity-60"
        >
          Cancel
        </button>
        {rejected > 0 && <DownloadReportButton fileName={file.name} rejections={preview.rejections} />}
      </div>
      {n === 0 && !conflict && (
        <p className="mt-3 text-sm text-text-muted">No row can be imported. Fix the file and check it again.</p>
      )}

      {rejected > 0 && (
        <div className="mt-6">
          <h2 className="mb-2 text-xs font-semibold tracking-wider text-text-muted uppercase">Rejected rows</h2>
          <RejectionTable fileName={file.name} rejections={preview.rejections} />
          <p className="mt-3 text-sm text-text-muted">
            Assets that already exist are never overwritten. After fixing, upload only the corrected rows.
          </p>
        </div>
      )}
    </section>
  )
}
