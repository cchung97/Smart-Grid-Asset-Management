import { ArrowDown } from 'lucide-react'
import { useExplorer } from '../state/explorerContext'
import { ImportFlowPanel } from './ImportFlowPanel'
import { ImportHistory } from './ImportHistory'
import { ImportPanel } from './ImportPanel'

/**
 * `/import`: everything about importing on one page. Two labelled cards side by
 * side on a wide screen: on the left the task (upload, then the check → review →
 * confirm report for the file in play), on the right the history of past imports.
 * Stacked in that order on a phone. No tree here; it is a task page, not the explorer.
 */
export function ImportPage() {
  const { importFlow } = useExplorer()
  // Stacked (below lg), a long review or rejection list pushes the history far down; offer a way to it.
  const reviewing = importFlow.stage === 'preview' || importFlow.stage === 'importing' || importFlow.stage === 'done'

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">Import</h1>
          <p className="mt-1 text-sm text-text-muted">
            Add assets from a CSV file. Check it first, review the result, then confirm. Nothing is saved until you do.
          </p>
        </div>
        {reviewing && (
          <a href="#import-history" className="inline-flex items-center gap-1 text-sm font-medium text-primary underline lg:hidden">
            Import history
            <ArrowDown aria-hidden className="size-4" />
          </a>
        )}
      </header>
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] 2xl:grid-cols-2">
        <div className="space-y-6">
          <ImportPanel />
          <ImportFlowPanel />
        </div>
        <ImportHistory />
      </div>
    </div>
  )
}
