import { Download, FileText, LoaderCircle, Upload, X } from 'lucide-react'
import { useState, type DragEvent } from 'react'
import { formatBytes } from '../lib'
import { useImportFlow } from '../state/useImportFlow'

const isCsv = (file: File) => file.name.toLowerCase().endsWith('.csv')

export const TEMPLATE_URL = '/api/imports/template'

/**
 * The upload card on the Import page, step 1 of the import. Pick a file, press
 * Check file and the result appears right below for review; nothing is stored
 * until the user confirms there.
 *
 * Once a file is chosen the dropzone is replaced by a chip with a remove button,
 * so a second file cannot be dropped over the first.
 *
 * The extension check here is UX only — the backend re-validates everything.
 */
export function ImportPanel() {
  const { flow, select, reset, check } = useImportFlow()

  const [hint, setHint] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  const file = 'file' in flow ? flow.file : null
  const busy = flow.stage === 'checking' || flow.stage === 'importing'

  function choose(next: File | undefined) {
    setError(null)
    if (!next) return
    if (!isCsv(next)) {
      setHint('Only .csv files can be imported.')
      return
    }
    setHint(null)
    select(next)
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragging(false)
    choose(e.dataTransfer.files[0])
  }

  async function onCheck() {
    if (flow.stage !== 'selected') return
    setError(null)
    const problem = await check(flow.file)
    if (problem) setError(problem)
  }

  function remove() {
    setError(null)
    setHint(null)
    reset()
  }

  return (
    <section aria-labelledby="upload-title" className="space-y-3 rounded-lg border border-border bg-surface-muted p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="upload-title" className="text-xs font-semibold tracking-wider text-text-muted uppercase">
          Upload a CSV
        </h2>
        <a href={TEMPLATE_URL} download className="inline-flex items-center gap-2 text-sm font-medium text-primary underline">
          <Download aria-hidden className="size-4" />
          Download template
        </a>
      </div>

      {file ? (
        <div className="flex items-center gap-3 rounded-lg border border-border-strong bg-surface p-4">
          <FileText aria-hidden className="size-6 shrink-0 text-primary" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-text" title={file.name}>
              {file.name}
            </p>
            <p className="text-xs text-text-muted">{formatBytes(file.size)}</p>
          </div>
          <button
            type="button"
            onClick={remove}
            disabled={busy}
            aria-label="Remove selected file"
            className="rounded-md p-2 text-text-muted hover:bg-neutral-soft disabled:cursor-not-allowed disabled:opacity-50"
          >
            <X aria-hidden className="size-4" />
          </button>
        </div>
      ) : (
        <div
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          className={`rounded-lg border border-dashed px-4 py-6 text-center ${
            dragging ? 'border-primary bg-primary-soft' : 'border-border-strong bg-surface'
          }`}
        >
          <Upload aria-hidden className="mx-auto mb-3 size-7 text-primary" />
          <p className="text-sm font-medium text-text">Drag a CSV file here</p>
          <p className="mt-1 mb-4 text-xs text-text-muted">or click to browse</p>
          <label className="inline-block cursor-pointer rounded-md bg-primary px-3 py-2 text-sm font-semibold text-on-primary hover:bg-primary-hover focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-primary">
            Choose file
            <input
              type="file"
              accept=".csv,text/csv"
              className="sr-only"
              onChange={(e) => choose(e.target.files?.[0])}
            />
          </label>
        </div>
      )}

      {hint && (
        <p role="alert" className="text-sm text-danger">
          {hint}
        </p>
      )}

      <button
        type="button"
        onClick={() => void onCheck()}
        disabled={flow.stage !== 'selected'}
        className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-semibold text-on-primary hover:bg-primary-hover disabled:cursor-not-allowed disabled:bg-neutral-soft disabled:text-text-muted sm:w-auto"
      >
        {flow.stage === 'checking' && <LoaderCircle aria-hidden className="size-4 animate-spin" />}
        {flow.stage === 'checking'
          ? 'Checking…'
          : flow.stage === 'importing'
            ? 'Importing…'
            : flow.stage === 'preview'
              ? 'Checked — review below'
              : 'Check file'}
      </button>
      <p className="text-xs leading-relaxed text-text-muted">Checking validates the file. Nothing is saved until you confirm.</p>

      {error && (
        <p role="alert" className="rounded-md bg-danger-soft px-3 py-2 text-sm text-danger">
          {error}
        </p>
      )}    </section>
  )
}
