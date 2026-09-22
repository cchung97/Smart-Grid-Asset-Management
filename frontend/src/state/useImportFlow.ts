import { useQueryClient } from '@tanstack/react-query'
import { ApiError, errorMessage } from '../api/client'
import { api, useInvalidateAssets } from '../api/queries'
import { useExplorer } from './explorerContext'

const STALE_NOTICE = 'The data changed since you checked the file, so it was checked again. Review the updated result.'

/** A message for a failed request, with the CSV line when the API reports one. */
function describe(err: unknown): string {
  const line = err instanceof ApiError ? err.body?.line : undefined
  return errorMessage(err) + (line ? ` (line ${line})` : '')
}

/**
 * The two-step import: check (validate, store nothing) then confirm (validate
 * again and store the valid rows). The server refuses a confirm whose outcome
 * differs from the one the user reviewed (412), and that is answered here by
 * checking the same file again and showing the new result.
 */
export function useImportFlow() {
  const { importFlow: flow, setImportFlow } = useExplorer()
  const client = useQueryClient()
  const invalidate = useInvalidateAssets()

  const select = (file: File) => setImportFlow({ stage: 'selected', file })
  const reset = () => setImportFlow({ stage: 'idle' })

  /** Validate the file without storing anything. Returns an error message, or null on success. */
  async function check(file: File, notice?: string): Promise<string | null> {
    setImportFlow({ stage: 'checking', file })
    try {
      const preview = await api.previewImport(file)
      setImportFlow({ stage: 'preview', file, preview, notice })
      return null
    } catch (err) {
      // A structurally bad file (not CSV, missing columns, too large): keep it selected so it can be swapped.
      setImportFlow({ stage: 'selected', file })
      return describe(err)
    }
  }

  /** Store the rows the user reviewed. */
  async function confirm(): Promise<void> {
    if (flow.stage !== 'preview') return
    const { file, preview } = flow
    setImportFlow({ stage: 'importing', file, preview })
    try {
      const result = await api.commitImport(file, preview.fingerprint)
      setImportFlow({ stage: 'done', fileName: file.name, result })
      if (result.committed) invalidate()
      void client.invalidateQueries({ queryKey: ['imports'] })
    } catch (err) {
      if (err instanceof ApiError && err.status === 412) {
        await check(file, STALE_NOTICE)
      } else if (err instanceof ApiError && err.status === 409) {
        setImportFlow({
          stage: 'preview', file, preview, conflict: true,
          error: 'Nothing was saved because the data changed while importing. Check the file again.',
        })
      } else {
        setImportFlow({ stage: 'preview', file, preview, error: describe(err) })
      }
    }
  }

  return { flow, select, reset, check, confirm }
}
