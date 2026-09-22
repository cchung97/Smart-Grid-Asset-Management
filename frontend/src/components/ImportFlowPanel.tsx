import { LoaderCircle } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useImportFlow } from '../state/useImportFlow'
import { ImportPreview } from './ImportPreview'
import { ImportResult } from './ImportResult'

/** The report side of the import, under the upload card: what to show for each stage. Nothing while idle or just selected. */
export function ImportFlowPanel() {
  const { flow, reset } = useImportFlow()
  switch (flow.stage) {
    case 'checking':
      return (
        <section role="status" className="flex items-center gap-3 rounded-lg border border-border bg-surface p-4 text-sm">
          <LoaderCircle aria-hidden className="size-5 animate-spin text-primary" />
          <span>
            Checking <span className="font-medium">{flow.file.name}</span>… nothing is being saved.
          </span>
        </section>
      )
    case 'preview':
    case 'importing':
      return <ImportPreview flow={flow} />
    case 'done':
      return (
        <div className="space-y-3">
          <ImportResult fileName={flow.fileName} result={flow.result} onDismiss={reset} />
          {flow.result.committed && (
            <Link to="/assets" className="inline-block text-sm font-medium text-primary underline">
              Browse assets
            </Link>
          )}
        </div>
      )
    default:
      return null
  }
}
