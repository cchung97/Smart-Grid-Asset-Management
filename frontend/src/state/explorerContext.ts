import { createContext, useContext } from 'react'
import type { ImportPreviewResponse, ImportResponse } from '../api/types'

/**
 * Where the two-step import currently is. The chosen File lives here (in
 * memory only) so the upload card, the review screen and the confirm step all see it.
 *  idle -> selected -> checking -> preview -> importing -> done
 */
export type ImportFlow =
  | { stage: 'idle' }
  | { stage: 'selected'; file: File }
  | { stage: 'checking'; file: File }
  | {
      stage: 'preview'
      file: File
      preview: ImportPreviewResponse
      /** Shown above the result, e.g. "the data changed since you checked". */
      notice?: string
      /** A failed confirm; the preview is still valid unless `conflict`. */
      error?: string
      /** The commit lost a race (409): nothing was stored and the file must be checked again. */
      conflict?: boolean
    }
  | { stage: 'importing'; file: File; preview: ImportPreviewResponse }
  | { stage: 'done'; fileName: string; result: ImportResponse }

export interface Explorer {
  expanded: ReadonlySet<string>
  toggle: (id: string) => void
  expand: (id: string) => void
  collapse: (id: string) => void
  expandMany: (ids: string[]) => void
  collapseAll: () => void
  importFlow: ImportFlow
  setImportFlow: (flow: ImportFlow) => void
  /** Mobile only: whether the asset rail is open as a drawer. */
  railOpen: boolean
  setRailOpen: (open: boolean) => void
}

export const ExplorerContext = createContext<Explorer | null>(null)

export function useExplorer(): Explorer {
  const ctx = useContext(ExplorerContext)
  if (!ctx) throw new Error('useExplorer must be used inside <ExplorerProvider>')
  return ctx
}
