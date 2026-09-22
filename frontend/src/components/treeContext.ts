import { createContext, useContext } from 'react'

/** What every AssetTreeNode needs from the tree that renders it. */
export interface TreeContextValue {
  selectedId: string | undefined
  /** The one node that is in the tab order (roving tabindex). */
  tabbableId: string | null
  onSelect: (id: string) => void
  onFocusNode: (id: string) => void
}

export const TreeContext = createContext<TreeContextValue | null>(null)

export function useTree(): TreeContextValue {
  const ctx = useContext(TreeContext)
  if (!ctx) throw new Error('AssetTreeNode must be rendered inside <AssetTree>')
  return ctx
}
