import { Download } from 'lucide-react'
import type { Rejection } from '../api/types'
import { downloadText, rejectionsToCsv, reportFilename } from '../lib'

/** Save the rejection report next to the file it is about. */
export function DownloadReportButton({ fileName, rejections }: { fileName: string; rejections: Rejection[] }) {
  return (
    <button
      type="button"
      onClick={() => downloadText(reportFilename(fileName), rejectionsToCsv(rejections))}
      className="inline-flex items-center gap-2 rounded-md border border-border-strong bg-surface px-3 py-2 text-sm font-medium text-text hover:bg-surface-muted"
    >
      <Download aria-hidden className="size-4" />
      Download report (CSV)
    </button>
  )
}

/**
 * Every rejected row: its line in the file, the asset id and why. The row and id
 * columns are as wide as their content (w-px shrinks a column to fit) so the reason
 * sits right beside them, and ids never wrap (ids like PNL-001-1-01 stay on one line);
 * on a narrow screen the table scrolls sideways instead of squeezing the ids.
 */
export function RejectionTable({ fileName, rejections }: { fileName: string; rejections: Rejection[] }) {
  return (
    <div className="max-h-[28rem] overflow-auto rounded-lg border border-border bg-surface">
      <table className="w-full min-w-[32rem] border-collapse text-left text-sm">
        <caption className="sr-only">Rows rejected from {fileName}, with reasons</caption>
        <thead className="sticky top-0 bg-surface-muted text-xs tracking-wider text-text-muted uppercase">
          <tr>
            <th scope="col" className="w-px px-4 py-3 font-semibold whitespace-nowrap">
              Row
            </th>
            <th scope="col" className="w-px px-4 py-3 font-semibold whitespace-nowrap">
              Asset ID
            </th>
            <th scope="col" className="px-4 py-3 font-semibold">
              Reason
            </th>
          </tr>
        </thead>
        <tbody>
          {rejections.map((r) => (
            <tr key={`${r.csv_row}-${r.asset_id}`} className="border-t border-border align-top">
              <td className="px-4 py-3 tabular-nums">{r.csv_row}</td>
              <td className="px-4 py-3 font-medium whitespace-nowrap">{r.asset_id || '—'}</td>
              <td className="px-4 py-3 break-words text-text-muted">{r.reason}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
